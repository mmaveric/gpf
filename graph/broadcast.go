package graph

import (
	"errors"
	"fmt"
	"math"
	"unsafe"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

var ErrInvalidBroadcast = errors.New("invalid broadcast destinations")

type broadcastRuntime[T any] struct {
	group        EdgeID
	source       endpointID
	destinations []endpointID
	config       queue.Config
	factory      queue.Factory[T]
	allocators   AllocatorDescriptors[T]
	current      *edgeBindingTarget
	plans        []EdgeQueuePlan
	ready        []bool
	branches     []edgeMetadata
}

// ConnectBroadcast atomically connects one ordinary output to multiple distinct
// ordinary inputs. It makes shallow T copies into separate branch-owned batches.
func ConnectBroadcast[T any](b *Builder, output OutPort[T], inputs []InPort[T], config EdgeConfig[T]) ([]EdgeID, error) {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return nil, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return nil, ErrBuilderSealed
	}
	if !output.Valid() || len(inputs) < 2 {
		return nil, ErrInvalidBroadcast
	}
	source := endpointID{nodeID: output.nodeID, portID: output.portID}
	metadata, ok := b.outputs[source]
	if output.owner == b.owner && !ok {
		return nil, ErrInvalidPort
	}
	if output.owner != b.owner {
		return nil, ErrForeignPort
	}
	if !ok || metadata.isError || metadata.hasConsumer {
		return nil, ErrFanOutUnsupported
	}
	if config.Capacity >= math.MaxUint32-1 {
		return nil, ErrInvalidBroadcast
	}
	queueConfig := queue.Config{Capacity: config.Capacity, BatchCapacity: config.BatchCapacity, Policy: config.Policy, NUMA: config.NUMA}
	if err := validateEdgeConfig(queueConfig); err != nil {
		return nil, err
	}
	destinations := make([]endpointID, len(inputs))
	seen := make(map[endpointID]bool, len(inputs))
	sourceGroup := effectiveGroup(b, source.nodeID)
	for index, input := range inputs {
		if !input.Valid() {
			return nil, ErrInvalidPort
		}
		target := endpointID{nodeID: input.nodeID, portID: input.portID}
		inputMetadata, exists := b.inputs[target]
		if input.owner == b.owner && !exists {
			return nil, ErrInvalidPort
		}
		if input.owner != b.owner {
			return nil, ErrForeignPort
		}
		if !exists || inputMetadata.hasProducer || seen[target] || pathExists(b, target.nodeID, source.nodeID) {
			return nil, ErrInvalidBroadcast
		}
		if config.Factory != nil && effectiveGroup(b, target.nodeID) != sourceGroup {
			return nil, ErrIncompatibleQueueFactory
		}
		seen[target] = true
		destinations[index] = target
	}
	for i := 1; i < len(destinations); i++ {
		for j := i; j > 0 && (destinations[j].nodeID < destinations[j-1].nodeID || (destinations[j].nodeID == destinations[j-1].nodeID && destinations[j].portID < destinations[j-1].portID)); j-- {
			destinations[j], destinations[j-1] = destinations[j-1], destinations[j]
		}
	}
	if uint64(len(b.edges))+uint64(len(destinations)) > uint64(^uint32(0)) {
		return nil, ErrNodeIDExhausted
	}
	first := EdgeID(len(b.edges) + 1)
	branches := make([]edgeMetadata, len(destinations))
	runtimeBinding := &broadcastRuntime[T]{group: first, source: source, destinations: destinations, config: queueConfig, factory: config.Factory, allocators: config.Allocators, plans: make([]EdgeQueuePlan, len(destinations)), ready: make([]bool, len(destinations)), branches: branches}
	ids := make([]EdgeID, len(destinations))
	for index, target := range destinations {
		id := EdgeID(len(b.edges) + 1)
		ids[index] = id
		branch := edgeMetadata{id: id, source: source, target: target, config: queueConfig, hasFactory: config.Factory != nil, broadcastGroup: first, broadcastIndex: uint32(index), broadcastCount: uint32(len(destinations))}
		b.edges = append(b.edges, branch)
		b.edgeBindings = append(b.edgeBindings, runtimeBinding.bind(index, id))
		b.inputs[target].hasProducer = true
		runtimeBinding.branches[index] = branch
	}
	metadata.hasConsumer = true
	b.revision++
	return ids, nil
}

func (f *broadcastRuntime[T]) bind(index int, branch EdgeID) edgeBinding {
	return func(bindings *edgeBindingTarget, plan EdgeQueuePlan) error {
		if plan.BroadcastID != f.group || index >= len(f.plans) || plan.EdgeID != f.branches[index].id {
			return ErrInvalidDefinition
		}
		if f.current == bindings {
			return nil
		}
		f.plans[index] = plan
		f.ready[index] = true
		for _, ready := range f.ready {
			if !ready {
				return nil
			}
		}
		f.current = bindings
		ops := make([]queue.QueueOps[T], len(f.plans))
		pools := make([]*batch.Pool[T], len(f.plans))
		metrics := make([]*edgeCounters, len(f.plans))
		for i, branchPlan := range f.plans {
			if i >= len(f.branches) || branchPlan.EdgeID != f.branches[i].id {
				return ErrInvalidDefinition
			}
			if f.factory != nil && branchPlan.Model != QueueModelLocal {
				return ErrIncompatibleQueueFactory
			}
			var err error
			ops[i], err = queueOperationsForModel[T](branchPlan.Model, f.config, f.factory, f.allocators.Queue)
			if err != nil {
				return fmt.Errorf("broadcast edge %d queue: %w", branchPlan.EdgeID, err)
			}
			pool, err := batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: f.config.Capacity + 2, BatchCapacity: f.config.BatchCapacity, NUMA: f.config.NUMA, OwnerID: uint32(branchPlan.EdgeID)}, Allocator: f.allocators.Pool})
			if err != nil {
				return fmt.Errorf("broadcast edge %d pool: %w", branchPlan.EdgeID, err)
			}
			pools[i] = pool
			if bindings.metrics != nil {
				metrics[i] = bindings.metrics.edges[branchPlan.EdgeID-1]
				bindings.metrics.pools = append(bindings.metrics.pools, poolMetricsBinding{identity: PoolMetrics{EdgeID: branchPlan.EdgeID, BroadcastID: f.group, SourceNodeID: f.source.nodeID, NUMA: f.config.NUMA}, read: pool.Metrics})
			}
		}
		scratch, err := batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: 2, BatchCapacity: f.config.BatchCapacity, NUMA: f.config.NUMA}, Allocator: f.allocators.Pool})
		if err != nil {
			return err
		}
		if bindings.metrics != nil {
			bindings.metrics.pools = append(bindings.metrics.pools, poolMetricsBinding{identity: PoolMetrics{BroadcastID: f.group, SourceNodeID: f.source.nodeID}, read: scratch.Metrics})
		}
		bindings.outputs = append(bindings.outputs, edgeOutputBinding{endpoint: f.source, configure: func(pointer unsafe.Pointer) func(bool) bool {
			output := (*node.Output[T])(pointer)
			node.InstallOutput(output, scratch.Acquire)
			pending := make([]batch.Batch[T], len(f.plans))
			done := make([]bool, len(f.plans))
			attempted := make([]bool, len(f.plans))
			active := false
			reset := func() {
				for i := range done {
					done[i] = false
					attempted[i] = false
					pending[i] = batch.Batch[T]{}
				}
				active = false
			}
			publish := func(source batch.Batch[T]) bool {
				length := source.Len()
				if !active {
					active = true
					if length > int(f.config.BatchCapacity) {
						for i, m := range metrics {
							if m != nil {
								m.producer.capacityRejected.Add(1)
								m.producer.capacityRejectedElements.Add(uint64(length))
							}
							done[i] = true
						}
					}
					if length <= int(f.config.BatchCapacity) {
						for i := range f.plans {
							clone, ok := pools[i].Acquire()
							if !ok {
								if metrics[i] != nil {
									metrics[i].producer.exhausted.Add(1)
								}
								done[i] = true
								continue
							}
							copy(clone.Mutable(), source.Items())
							clone.SetLen(length)
							pending[i] = clone
						}
					}
				}
				allDone := true
				for i := range f.plans {
					if done[i] {
						continue
					}
					if pending[i].Cap() == 0 {
						done[i] = true
						continue
					}
					if attempted[i] && metrics[i] != nil {
						metrics[i].producer.retries.Add(1)
					}
					if ops[i].TryPush(pending[i]) {
						if metrics[i] != nil {
							metrics[i].producer.accepted.Add(1)
							metrics[i].producer.elements.Add(uint64(length))
							metrics[i].sample(ops[i].Len())
						}
						pending[i] = batch.Batch[T]{}
						done[i] = true
						attempted[i] = false
						continue
					}
					if f.config.Policy == queue.Drop {
						if metrics[i] != nil {
							metrics[i].producer.dropped.Add(1)
							metrics[i].producer.droppedElements.Add(uint64(length))
							metrics[i].sample(ops[i].Len())
						}
						pending[i].Release()
						pending[i] = batch.Batch[T]{}
						done[i] = true
						continue
					}
					attempted[i] = true
					if metrics[i] != nil {
						metrics[i].sample(ops[i].Len())
					}
					allDone = false
				}
				if !allDone {
					for _, complete := range done {
						if !complete {
							return false
						}
					}
				}
				source.Release()
				reset()
				return true
			}
			discard := func(value batch.Batch[T]) {
				for i := range pending {
					if pending[i].Cap() != 0 {
						if metrics[i] != nil {
							if bindings.discarding {
								metrics[i].discard(pending[i].Len())
							} else {
								metrics[i].producer.failed.Add(1)
								metrics[i].producer.failedElements.Add(uint64(pending[i].Len()))
							}
						}
						pending[i].Release()
						pending[i] = batch.Batch[T]{}
					}
					done[i] = true
				}
				active = false
				_ = value
			}
			return func(failed bool) bool { return node.FinalizeOutputObserved(output, failed, publish, discard) }
		}})
		for index, target := range f.destinations {
			metric := metrics[index]
			op := ops[index]
			bindings.inputs = append(bindings.inputs, edgeInputBinding{endpoint: target, input: func(pointer unsafe.Pointer) {
				input := (*runtimeInput[T])(pointer)
				*input = runtimeInput[T]{ready: func() bool { return op.Len() != 0 }, pop: func(value *batch.Batch[T]) bool {
					if !op.TryPop(value) {
						return false
					}
					if metric != nil {
						metric.consumer.dequeued.Add(1)
						metric.consumer.elements.Add(uint64(value.Len()))
						metric.sample(op.Len())
					}
					return true
				}}
			}})
			bindings.drains = append(bindings.drains, func() {
				var value batch.Batch[T]
				for op.TryPop(&value) {
					if metric != nil {
						metric.discard(value.Len())
					}
					value.Release()
				}
			})
			bindings.queuesEmpty = append(bindings.queuesEmpty, func() bool { return op.Len() == 0 })
		}
		bindings.onStop = append(bindings.onStop, func() {
			if f.current == bindings {
				f.current = nil
				f.ready = make([]bool, len(f.plans))
			}
		})
		return nil
	}
}

func (f *broadcastRuntime[T]) bindOne(index int, plan EdgeQueuePlan) error {
	_ = index
	_ = plan
	return nil
}
