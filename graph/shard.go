package graph

import (
	"errors"
	"fmt"
	"math"
	"unsafe"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

var (
	// ErrInvalidShard reports invalid sharding configuration or destinations.
	ErrInvalidShard = errors.New("invalid sharding configuration")
	// ErrInvalidShardIndex is panicked when a selector returns an invalid branch.
	ErrInvalidShardIndex = errors.New("shard selector returned an out-of-range branch")
)

type shardRuntime[T any] struct {
	group        EdgeID
	source       endpointID
	destinations []endpointID
	selectBranch func(T) uint32
	config       queue.Config
	factory      queue.Factory[T]
	allocators   AllocatorDescriptors[T]
	current      *edgeBindingTarget
	plans        []EdgeQueuePlan
	ready        []bool
	branches     []edgeMetadata
}

// ConnectShard routes each element to exactly one of the distinct ordinary
// inputs. Selector indices refer to inputs in the supplied order. The selector
// must return an index in [0, len(inputs)); violating that contract panics with
// ErrInvalidShardIndex during execution.
func ConnectShard[T any](b *Builder, output OutPort[T], inputs []InPort[T], selector func(T) uint32, config EdgeConfig[T]) ([]EdgeID, error) {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return nil, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return nil, ErrBuilderSealed
	}
	if !output.Valid() || len(inputs) < 2 || selector == nil {
		return nil, ErrInvalidShard
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
	if config.Capacity >= math.MaxUint32-2 {
		return nil, ErrInvalidShard
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
			return nil, ErrInvalidShard
		}
		if config.Factory != nil && effectiveGroup(b, target.nodeID) != sourceGroup {
			return nil, ErrIncompatibleQueueFactory
		}
		seen[target] = true
		destinations[index] = target
	}
	if uint64(len(b.edges))+uint64(len(destinations)) > uint64(^uint32(0)) {
		return nil, ErrNodeIDExhausted
	}
	first := EdgeID(len(b.edges) + 1)
	branches := make([]edgeMetadata, len(destinations))
	runtimeBinding := &shardRuntime[T]{group: first, source: source, destinations: destinations, selectBranch: selector, config: queueConfig, factory: config.Factory, allocators: config.Allocators, plans: make([]EdgeQueuePlan, len(destinations)), ready: make([]bool, len(destinations)), branches: branches}
	ids := make([]EdgeID, len(destinations))
	for index, target := range destinations {
		id := EdgeID(len(b.edges) + 1)
		ids[index] = id
		branch := edgeMetadata{id: id, source: source, target: target, config: queueConfig, hasFactory: config.Factory != nil, shardGroup: first, shardIndex: uint32(index), shardCount: uint32(len(destinations))}
		b.edges = append(b.edges, branch)
		b.edgeBindings = append(b.edgeBindings, runtimeBinding.bind(index, id))
		b.inputs[target].hasProducer = true
		runtimeBinding.branches[index] = branch
	}
	metadata.hasConsumer = true
	b.revision++
	return ids, nil
}

func (f *shardRuntime[T]) bind(index int, branch EdgeID) edgeBinding {
	return func(bindings *edgeBindingTarget, plan EdgeQueuePlan) error {
		if plan.ShardID != f.group || index >= len(f.plans) || plan.EdgeID != f.branches[index].id || branch != plan.EdgeID {
			return ErrInvalidDefinition
		}
		if f.current == bindings {
			return nil
		}
		f.plans[index], f.ready[index] = plan, true
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
			if f.factory != nil && branchPlan.Model != QueueModelLocal {
				return ErrIncompatibleQueueFactory
			}
			var err error
			ops[i], err = queueOperationsForModel[T](branchPlan.Model, f.config, f.factory, f.allocators.Queue)
			if err != nil {
				return fmt.Errorf("shard edge %d queue: %w", branchPlan.EdgeID, err)
			}
			pools[i], err = batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: f.config.Capacity + 1, BatchCapacity: f.config.BatchCapacity, NUMA: f.config.NUMA, OwnerID: uint32(branchPlan.EdgeID)}, Allocator: f.allocators.Pool})
			if err != nil {
				return fmt.Errorf("shard edge %d pool: %w", branchPlan.EdgeID, err)
			}
			if bindings.metrics != nil {
				metrics[i] = bindings.metrics.edges[branchPlan.EdgeID-1]
				bindings.metrics.pools = append(bindings.metrics.pools, poolMetricsBinding{identity: PoolMetrics{EdgeID: branchPlan.EdgeID, ShardID: f.group, SourceNodeID: f.source.nodeID, NUMA: f.config.NUMA}, read: pools[i].Metrics})
			}
		}
		scratch, err := batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: 2, BatchCapacity: f.config.BatchCapacity, NUMA: f.config.NUMA}, Allocator: f.allocators.Pool})
		if err != nil {
			return err
		}
		if bindings.metrics != nil {
			bindings.metrics.pools = append(bindings.metrics.pools, poolMetricsBinding{identity: PoolMetrics{ShardID: f.group, SourceNodeID: f.source.nodeID}, read: scratch.Metrics})
		}
		bindings.outputs = append(bindings.outputs, edgeOutputBinding{endpoint: f.source, configure: func(pointer unsafe.Pointer) func(bool) bool {
			output := (*node.Output[T])(pointer)
			node.InstallOutput(output, scratch.Acquire)
			staging := make([]batch.Batch[T], len(f.plans))
			attempted := make([]bool, len(f.plans))
			active, cursor := false, 0
			reset := func() {
				for i := range staging {
					staging[i] = batch.Batch[T]{}
					attempted[i] = false
				}
				active, cursor = false, 0
			}
			flush := func(index int) bool {
				value := staging[index]
				if value.Cap() == 0 {
					return true
				}
				length := value.Len()
				if attempted[index] && metrics[index] != nil {
					metrics[index].producer.retries.Add(1)
				}
				if ops[index].TryPush(value) {
					if metrics[index] != nil {
						metrics[index].producer.accepted.Add(1)
						metrics[index].producer.elements.Add(uint64(length))
						metrics[index].sample(ops[index].Len())
					}
					staging[index] = batch.Batch[T]{}
					attempted[index] = false
					return true
				}
				if f.config.Policy == queue.Drop {
					if metrics[index] != nil {
						metrics[index].producer.dropped.Add(1)
						metrics[index].producer.droppedElements.Add(uint64(length))
						metrics[index].sample(ops[index].Len())
					}
					value.Release()
					staging[index] = batch.Batch[T]{}
					attempted[index] = false
					return true
				}
				attempted[index] = true
				if metrics[index] != nil {
					metrics[index].sample(ops[index].Len())
				}
				return false
			}
			publish := func(source batch.Batch[T]) bool {
				if !active {
					active = true
				}
				for index := range staging {
					if uint32(staging[index].Len()) == f.config.BatchCapacity && !flush(index) {
						return false
					}
				}
				items := source.Items()
				for cursor < len(items) {
					index := int(f.selectBranch(items[cursor]))
					if index < 0 || index >= len(staging) {
						panic(ErrInvalidShardIndex)
					}
					if staging[index].Cap() == 0 {
						value, ok := pools[index].Acquire()
						if !ok {
							if metrics[index] != nil {
								metrics[index].producer.exhausted.Add(1)
								metrics[index].producer.poolDroppedElements.Add(1)
							}
							cursor++
							continue
						}
						staging[index] = value
					}
					length := staging[index].Len()
					staging[index].Mutable()[length] = items[cursor]
					staging[index].SetLen(length + 1)
					cursor++
					if uint32(staging[index].Len()) == f.config.BatchCapacity && !flush(index) {
						return false
					}
				}
				for index := range staging {
					if staging[index].Len() != 0 && !flush(index) {
						return false
					}
				}
				source.Release()
				reset()
				return true
			}
			discard := func(source batch.Batch[T]) {
				unassigned := len(source.Items()) - cursor
				if unassigned < 0 {
					unassigned = 0
				}
				if len(metrics) != 0 && metrics[0] != nil && unassigned != 0 {
					if bindings.discarding {
						metrics[0].producer.shardShutdownElements.Add(uint64(unassigned))
					} else {
						metrics[0].producer.failed.Add(1)
						metrics[0].producer.failedElements.Add(uint64(unassigned))
					}
				}
				for index := range staging {
					if staging[index].Cap() == 0 {
						continue
					}
					if metrics[index] != nil {
						if bindings.discarding {
							metrics[index].discard(staging[index].Len())
						} else {
							metrics[index].producer.failed.Add(1)
							metrics[index].producer.failedElements.Add(uint64(staging[index].Len()))
						}
					}
					staging[index].Release()
				}
				reset()
			}
			return func(failed bool) bool { return node.FinalizeOutputObserved(output, failed, publish, discard) }
		}})
		for index, target := range f.destinations {
			op, metric := ops[index], metrics[index]
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
