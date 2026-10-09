package graph

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/numa"
	"github.com/asharipov/gpf/queue"
)

// ErrInvalidIngress reports invalid configuration, provenance, or a stale bind.
var ErrInvalidIngress = errors.New("invalid ingress definition or configuration")

// IngressConfig bounds external single-producer transport and source storage.
type IngressConfig struct {
	Capacity, BatchCapacity uint32
	Policy                  queue.Policy
	NUMA                    numa.Placement
}

// IngressAllocator is a typed inactive source-pool allocator descriptor.
type IngressAllocator[T any] struct{ Pool batch.Allocator[T] }

// IngressDefinition is a dormant typed source handle owned by one definition.
type IngressDefinition[T any] struct {
	owner *graphOwner
	id    NodeID
}

// NodeID returns the ingress adapter's registered node identity.
func (d IngressDefinition[T]) NodeID() NodeID { return d.id }

// Output returns the typed ordinary port for graph connection.
func (d IngressDefinition[T]) Output() OutPort[T] {
	return OutPort[T]{owner: d.owner, nodeID: d.id, portID: 1}
}

// IngressStatus describes publication without hiding caller ownership.
type IngressStatus uint8

const (
	IngressAccepted IngressStatus = iota
	IngressFull
	IngressClosed
	IngressDropped
	IngressTooLarge
	IngressInvalidBatch
	IngressExhausted
)

type ingressCounters struct {
	id                                                      NodeID
	group                                                   GroupID
	accepted, elements, dropped, droppedElements, exhausted atomic.Uint64
}

// IngressMetrics counts external publication outcomes separately from edge work.
type IngressMetrics struct {
	NodeID                                                                                        NodeID
	GroupID                                                                                       GroupID
	AcceptedBatches, AcceptedElements, QueueDroppedBatches, QueueDroppedElements, PoolExhaustions uint64
}

// Ingress is an executor-local endpoint. Producer operations must be serialized
// by one caller; admission closure may overlap that producer. Uncommitted owned
// batches remain releasable after shutdown.
type Ingress[T any] struct {
	pool     *batch.Pool[T]
	queue    *remoteRing[T]
	config   IngressConfig
	counts   *ingressCounters
	open     atomic.Bool
	inflight atomic.Int64
}

// AddIngress registers a dormant external source and one typed ordinary output.
func AddIngress[T any](b *Builder, config IngressConfig, descriptors ...IngressAllocator[T]) (IngressDefinition[T], error) {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return IngressDefinition[T]{}, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return IngressDefinition[T]{}, ErrBuilderSealed
	}
	if config.Capacity == math.MaxUint32 || validateEdgeConfig(queue.Config{Capacity: config.Capacity, BatchCapacity: config.BatchCapacity, Policy: config.Policy, NUMA: config.NUMA}) != nil {
		return IngressDefinition[T]{}, ErrInvalidIngress
	}
	if len(descriptors) > 1 {
		return IngressDefinition[T]{}, ErrInvalidIngress
	}
	var allocator IngressAllocator[T]
	if len(descriptors) != 0 {
		allocator = descriptors[0]
	}
	metadata := nodeMetadata{kind: ingressRegistration, outputCount: 1}
	metadata.normalizer = func(target *normalizationTarget) error {
		id := target.currentNode
		pool, err := batch.NewPoolWithOptions[T](batch.PoolOptions[T]{Config: batch.PoolConfig{Capacity: config.Capacity + 1, BatchCapacity: config.BatchCapacity, NUMA: config.NUMA}, Allocator: allocator.Pool})
		if err != nil {
			return err
		}
		counts := &ingressCounters{id: id, group: target.currentGroup}
		endpoint := &Ingress[T]{pool: pool, queue: newRemoteRing[T](config.Capacity), config: config, counts: counts}
		endpoint.open.Store(true)
		if target.executor.ingresses == nil {
			target.executor.ingresses = make(map[NodeID]unsafe.Pointer)
		}
		logicalID := target.currentLogicalNode
		if logicalID == 0 {
			logicalID = id
		}
		target.executor.ingresses[logicalID] = unsafe.Pointer(endpoint)
		target.metrics.ingresses = append(target.metrics.ingresses, counts)
		target.metrics.pools = append(target.metrics.pools, poolMetricsBinding{identity: PoolMetrics{SourceNodeID: id, SourceGroupID: target.currentGroup, NUMA: config.NUMA}, read: pool.Metrics})
		target.executor.participants = append(target.executor.participants, stopParticipant{closeAdmission: endpoint.closeAdmission})
		var outputs node.Outputs1[T]
		finalizers := bindOutputs(target, id, &outputs)
		pending := false
		var incoming batch.Batch[T]
		target.runners = append(target.runners, runner{
			source: true,
			run: func() bool {
				if pending {
					if !target.finalize(finalizers, false) {
						return true
					}
					pending = false
				}
				if !endpoint.queue.tryPop(&incoming) {
					return false
				}
				if !outputs.Out1.Commit(&incoming) {
					incoming.Release()
					return true
				}
				pending = !target.finalize(finalizers, false)
				return true
			},
			pending: func() bool { return pending || endpoint.queue.len() != 0 },
			discard: func() {
				target.finalize(finalizers, true)
				pending = false
				for endpoint.queue.tryPop(&incoming) {
					incoming.Release()
				}
			},
		})
		return nil
	}
	id, err := b.register(metadata)
	if err != nil {
		return IngressDefinition[T]{}, err
	}
	return IngressDefinition[T]{owner: b.owner, id: id}, nil
}

// BindIngress resolves a typed runtime-local endpoint without boxing its values.
func BindIngress[T any](e *Executor, definition IngressDefinition[T]) (*Ingress[T], error) {
	if e == nil || definition.owner == nil || e.owner != definition.owner || definition.id == 0 {
		return nil, ErrInvalidIngress
	}
	pointer := e.ingresses[definition.id]
	if pointer == nil {
		return nil, ErrInvalidIngress
	}
	endpoint := (*Ingress[T])(pointer)
	if !endpoint.open.Load() {
		return nil, ErrInvalidIngress
	}
	return endpoint, nil
}

// Acquire returns a caller-owned batch from this endpoint's bounded source pool.
func (i *Ingress[T]) Acquire() (batch.Batch[T], bool) {
	if i == nil || !i.open.Load() {
		return batch.Batch[T]{}, false
	}
	value, ok := i.pool.Acquire()
	if !ok {
		i.counts.exhausted.Add(1)
	}
	return value, ok
}

// Commit publishes an owned ingress acquisition. Accepted and Dropped clear the
// handle; Full and Closed retain ownership for caller retry or release.
func (i *Ingress[T]) Commit(value *batch.Batch[T]) IngressStatus {
	if i == nil || !i.open.Load() {
		return IngressClosed
	}
	i.inflight.Add(1)
	defer i.inflight.Add(-1)
	if !i.open.Load() {
		return IngressClosed
	}
	if value == nil || value.Cap() == 0 {
		return IngressInvalidBatch
	}
	length := value.Len()
	if length > int(i.config.BatchCapacity) {
		return IngressTooLarge
	}
	if i.queue.tryPush(*value) {
		*value = batch.Batch[T]{}
		i.counts.accepted.Add(1)
		i.counts.elements.Add(uint64(length))
		return IngressAccepted
	}
	if i.config.Policy == queue.Drop {
		value.Release()
		i.counts.dropped.Add(1)
		i.counts.droppedElements.Add(uint64(length))
		return IngressDropped
	}
	return IngressFull
}

// Submit copies one bounded batch, with no partial delivery on rejection.
func (i *Ingress[T]) Submit(items []T) IngressStatus {
	if len(items) == 0 {
		return IngressAccepted
	}
	if i == nil || !i.open.Load() {
		return IngressClosed
	}
	if len(items) > int(i.config.BatchCapacity) {
		return IngressTooLarge
	}
	value, ok := i.Acquire()
	if !ok {
		if !i.open.Load() {
			return IngressClosed
		}
		return IngressExhausted
	}
	copy(value.Mutable(), items)
	value.SetLen(len(items))
	status := i.Commit(&value)
	if status != IngressAccepted && status != IngressDropped {
		value.Release()
	}
	return status
}

func (i *Ingress[T]) closeAdmission(context.Context) error {
	i.open.Store(false)
	for i.inflight.Load() != 0 {
		runtime.Gosched()
	}
	return nil
}
