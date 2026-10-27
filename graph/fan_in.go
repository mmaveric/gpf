package graph

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

var ErrFanInCycle = errors.New("fan-in connection would create a cycle")

type fanInRuntime[T any] struct {
	mergeID    EdgeID
	sources    []endpointID
	target     endpointID
	config     queue.Config
	factory    queue.Factory[T]
	allocators AllocatorDescriptors[T]
	branchIDs  []EdgeID
	current    *edgeBindingTarget
}

// ConnectFanIn atomically connects multiple homogeneous ordinary outputs to one
// input. The returned EdgeIDs identify each producer branch. Delivery order
// across producers is unspecified and each accepted batch is delivered once.
func ConnectFanIn[T any](b *Builder, outputs []OutPort[T], input InPort[T], config EdgeConfig[T]) ([]EdgeID, error) {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return nil, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return nil, ErrBuilderSealed
	}
	if len(outputs) < 2 || !input.Valid() {
		return nil, ErrFanInUnsupported
	}
	target := endpointID{nodeID: input.nodeID, portID: input.portID}
	inputMetadata, ok := b.inputs[target]
	if input.owner == b.owner && !ok {
		return nil, ErrInvalidPort
	}
	if input.owner != b.owner {
		return nil, ErrForeignPort
	}
	if inputMetadata.hasProducer {
		return nil, ErrInputConnected
	}
	queueConfig := queue.Config{Capacity: config.Capacity, BatchCapacity: config.BatchCapacity, Policy: config.Policy, NUMA: config.NUMA}
	if err := validateEdgeConfig(queueConfig); err != nil {
		return nil, err
	}
	sources := make([]endpointID, len(outputs))
	seen := make(map[endpointID]bool, len(outputs))
	targetGroup := effectiveGroup(b, input.nodeID)
	for index, output := range outputs {
		if !output.Valid() {
			return nil, ErrInvalidPort
		}
		source := endpointID{nodeID: output.nodeID, portID: output.portID}
		metadata, exists := b.outputs[source]
		if output.owner == b.owner && !exists {
			return nil, ErrInvalidPort
		}
		if output.owner != b.owner {
			return nil, ErrForeignPort
		}
		if !exists || metadata.isError || metadata.hasConsumer || seen[source] {
			return nil, ErrFanInUnsupported
		}
		if pathExists(b, target.nodeID, source.nodeID) {
			return nil, ErrFanInCycle
		}
		if config.Factory != nil && effectiveGroup(b, source.nodeID) != targetGroup {
			return nil, ErrIncompatibleQueueFactory
		}
		seen[source] = true
		sources[index] = source
	}
	// Canonical producer order makes branch IDs and snapshots deterministic.
	for i := 1; i < len(sources); i++ {
		for j := i; j > 0 && (sources[j].nodeID < sources[j-1].nodeID || (sources[j].nodeID == sources[j-1].nodeID && sources[j].portID < sources[j-1].portID)); j-- {
			sources[j], sources[j-1] = sources[j-1], sources[j]
		}
	}
	if uint64(len(b.edges))+uint64(len(sources)) > uint64(^uint32(0)) {
		return nil, ErrNodeIDExhausted
	}
	first := EdgeID(len(b.edges) + 1)
	runtimeBinding := &fanInRuntime[T]{mergeID: first, sources: sources, target: target, config: queueConfig, factory: config.Factory, allocators: config.Allocators}
	ids := make([]EdgeID, len(sources))
	for index, source := range sources {
		id := EdgeID(len(b.edges) + 1)
		ids[index] = id
		b.edges = append(b.edges, edgeMetadata{id: id, source: source, target: target, config: queueConfig, hasFactory: config.Factory != nil, fanInMerge: first, fanInCount: uint32(len(sources))})
		b.edgeBindings = append(b.edgeBindings, runtimeBinding.bind(id, source))
		b.outputs[source].hasConsumer = true
	}
	runtimeBinding.branchIDs = append([]EdgeID(nil), ids...)
	inputMetadata.hasProducer = true
	b.revision++
	return ids, nil
}

func effectiveGroup(b *Builder, id NodeID) GroupID {
	if group := b.placements[id]; group != InvalidGroupID {
		return group
	}
	return DefaultGroupID
}

func pathExists(b *Builder, start, target NodeID) bool {
	if start == target {
		return true
	}
	seen := map[NodeID]bool{start: true}
	queueNodes := []NodeID{start}
	for len(queueNodes) > 0 {
		current := queueNodes[0]
		queueNodes = queueNodes[1:]
		for _, edge := range b.edges {
			if edge.source.nodeID != current {
				continue
			}
			if edge.target.nodeID == target {
				return true
			}
			if !seen[edge.target.nodeID] {
				seen[edge.target.nodeID] = true
				queueNodes = append(queueNodes, edge.target.nodeID)
			}
		}
	}
	return false
}

func (f *fanInRuntime[T]) bind(branch EdgeID, source endpointID) edgeBinding {
	return func(bindings *edgeBindingTarget, plan EdgeQueuePlan) error {
		if plan.MergeID != f.mergeID || plan.Model == QueueModelInvalid {
			return ErrInvalidDefinition
		}
		if f.current == bindings {
			return nil
		}
		ops, err := queueOperationsForModel[T](plan.Model, f.config, f.factory, f.allocators.Queue)
		if err != nil {
			return err
		}
		f.current = bindings
		sources, targets := f.sources, []endpointID{f.target}
		if plan.endpoints != nil {
			sources, targets = plan.endpoints.sources, plan.endpoints.targets
		}
		pools := make([]*batch.Pool[T], len(sources))
		for index, producer := range sources {
			ownerID := branch
			if index < len(f.branchIDs) {
				ownerID = f.branchIDs[index]
			}
			pool, err := batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: f.config.Capacity + 1, BatchCapacity: f.config.BatchCapacity, NUMA: f.config.NUMA, OwnerID: uint32(ownerID)}, Allocator: f.allocators.Pool})
			if err != nil {
				return fmt.Errorf("fan-in edge %d pool: %w", ownerID, err)
			}
			pools[index] = pool
			if bindings.metrics != nil && ownerID > 0 && int(ownerID) <= len(bindings.metrics.edges) {
				metric := bindings.metrics.edges[ownerID-1]
				metric.identity.SourceNodeID = producer.nodeID
				bindings.metrics.pools = append(bindings.metrics.pools, poolMetricsBinding{identity: PoolMetrics{EdgeID: metric.identity.EdgeID, MergeID: f.mergeID, SourceNodeID: producer.nodeID, SourceGroupID: metric.identity.SourceGroupID, NUMA: f.config.NUMA}, read: pool.Metrics})
			}
		}
		for index, producer := range sources {
			pool := pools[index]
			var metric *edgeCounters
			if bindings.metrics != nil && index < len(f.branchIDs) && f.branchIDs[index] > 0 && int(f.branchIDs[index]) <= len(bindings.metrics.edges) {
				metric = bindings.metrics.edges[f.branchIDs[index]-1]
			}
			bindings.outputs = append(bindings.outputs, edgeOutputBinding{endpoint: producer, configure: func(pointer unsafe.Pointer) func(bool) bool {
				output := (*node.Output[T])(pointer)
				node.InstallOutput(output, pool.Acquire)
				pending := false
				publish := func(value batch.Batch[T]) bool {
					length := uint64(value.Len())
					if pending && metric != nil {
						metric.producer.retries.Add(1)
					}
					if ops.TryPush(value) {
						if metric != nil {
							metric.producer.accepted.Add(1)
							metric.producer.elements.Add(length)
							metric.sample(ops.Len())
						}
						pending = false
						return true
					}
					if f.config.Policy == queue.Drop {
						if metric != nil {
							metric.producer.dropped.Add(1)
							metric.producer.droppedElements.Add(length)
							metric.sample(ops.Len())
						}
						value.Release()
						return true
					}
					pending = true
					if metric != nil {
						metric.sample(ops.Len())
					}
					return false
				}
				return func(failed bool) bool {
					return node.FinalizeOutputObserved(output, failed, publish, func(value batch.Batch[T]) {
						if metric != nil {
							if bindings.discarding {
								metric.discard(value.Len())
							} else {
								metric.producer.failed.Add(1)
								metric.producer.failedElements.Add(uint64(value.Len()))
							}
						}
						pending = false
					})
				}
			}})
		}
		for _, consumer := range targets {
			bindings.inputs = append(bindings.inputs, edgeInputBinding{endpoint: consumer, input: func(pointer unsafe.Pointer) {
				input := (*runtimeInput[T])(pointer)
				*input = runtimeInput[T]{ready: func() bool { return ops.Len() != 0 }, pop: func(value *batch.Batch[T]) bool {
					if !ops.TryPop(value) {
						return false
					}
					owner := EdgeID(value.OwnerID())
					if bindings.metrics != nil && owner > 0 && int(owner) <= len(bindings.metrics.edges) {
						metric := bindings.metrics.edges[owner-1]
						metric.consumer.dequeued.Add(1)
						metric.consumer.elements.Add(uint64(value.Len()))
						metric.sample(ops.Len())
					}
					return true
				}}
			}})
		}
		bindings.drains = append(bindings.drains, func() {
			var value batch.Batch[T]
			for ops.TryPop(&value) {
				owner := EdgeID(value.OwnerID())
				if bindings.metrics != nil && owner > 0 && int(owner) <= len(bindings.metrics.edges) {
					bindings.metrics.edges[owner-1].discard(value.Len())
				}
				value.Release()
			}
		})
		bindings.queuesEmpty = append(bindings.queuesEmpty, func() bool { return ops.Len() == 0 })
		bindings.onStop = append(bindings.onStop, func() {
			if f.current == bindings {
				f.current = nil
			}
		})
		return nil
	}
}
