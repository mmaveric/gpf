package queue

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"github.com/asharipov/gpf/batch"
)

var (
	// ErrInvalidCapacity reports a zero queue capacity.
	ErrInvalidCapacity = errors.New("queue capacity must be greater than zero")
	// ErrCapacityOverflow reports capacity metadata that cannot fit in process memory indexing.
	ErrCapacityOverflow = errors.New("queue capacity overflows addressable storage")
)

const (
	cellFree uint32 = iota
	cellWriting
	cellReady
	cellReading
)

type concurrentCell[T any] struct {
	state atomic.Uint32
	value batch.Batch[T]
}

type concurrentRing[T any] struct {
	capacity uint32
	buffer   []concurrentCell[T]
	enqueue  atomic.Uint32
	dequeue  atomic.Uint32
	occupied atomic.Uint32
}

func newConcurrentRing[T any](capacity uint32) (*concurrentRing[T], error) {
	if capacity == 0 {
		return nil, ErrInvalidCapacity
	}
	maxInt := uint64(^uint(0) >> 1)
	cellSize := uint64(unsafe.Sizeof(concurrentCell[T]{}))
	if uint64(capacity) > maxInt || uint64(capacity) > maxInt/cellSize {
		return nil, ErrCapacityOverflow
	}
	return &concurrentRing[T]{capacity: capacity, buffer: make([]concurrentCell[T], int(capacity))}, nil
}

func newConcurrentRingAt[T any](capacity, start uint32) (*concurrentRing[T], error) {
	q, err := newConcurrentRing[T](capacity)
	if err != nil {
		return nil, err
	}
	start %= capacity
	q.enqueue.Store(start)
	q.dequeue.Store(start)
	return q, nil
}

func (q *concurrentRing[T]) next(index uint32) uint32 {
	index++
	if index == q.capacity {
		return 0
	}
	return index
}

func (q *concurrentRing[T]) reservePush() (uint32, *concurrentCell[T], bool) {
	position := q.enqueue.Load()
	if position >= q.capacity || !q.enqueue.CompareAndSwap(position, q.capacity) {
		return 0, nil, false
	}
	cell := &q.buffer[position]
	if !cell.state.CompareAndSwap(cellFree, cellWriting) {
		q.enqueue.Store(position)
		return 0, nil, false
	}
	q.occupied.Add(1)
	q.enqueue.Store(q.next(position))
	return position, cell, true
}

func (q *concurrentRing[T]) publish(_ uint32, cell *concurrentCell[T], value batch.Batch[T]) {
	cell.value = value
	cell.state.Store(cellReady)
}

func (q *concurrentRing[T]) reservePop() (uint32, *concurrentCell[T], bool) {
	position := q.dequeue.Load()
	if position >= q.capacity || !q.dequeue.CompareAndSwap(position, q.capacity) {
		return 0, nil, false
	}
	cell := &q.buffer[position]
	if !cell.state.CompareAndSwap(cellReady, cellReading) {
		q.dequeue.Store(position)
		return 0, nil, false
	}
	q.dequeue.Store(q.next(position))
	return position, cell, true
}

func (q *concurrentRing[T]) release(_ uint32, cell *concurrentCell[T], destination *batch.Batch[T]) {
	*destination = cell.value.Move()
	cell.state.Store(cellFree)
	q.occupied.Add(^uint32(0))
}

func (q *concurrentRing[T]) len() uint32 {
	used := q.occupied.Load()
	if used > q.capacity {
		return q.capacity
	}
	return used
}

func (q *concurrentRing[T]) tryPush(value batch.Batch[T]) bool {
	position, cell, ok := q.reservePush()
	if !ok {
		return false
	}
	q.publish(position, cell, value)
	return true
}

func (q *concurrentRing[T]) tryPop(destination *batch.Batch[T]) bool {
	if destination == nil {
		return false
	}
	position, cell, ok := q.reservePop()
	if !ok {
		return false
	}
	q.release(position, cell, destination)
	return true
}

func (q *concurrentRing[T]) operations() QueueOps[T] {
	return QueueOps[T]{TryPush: q.tryPush, TryPop: q.tryPop, Len: q.len}
}

// MPSC is a bounded typed queue for multiple producers and one consumer.
type MPSC[T any] struct{ ring *concurrentRing[T] }

// NewMPSC allocates a bounded MPSC queue and all of its slot metadata.
func NewMPSC[T any](capacity uint32) (*MPSC[T], error) {
	ring, err := newConcurrentRing[T](capacity)
	if err != nil {
		return nil, err
	}
	return &MPSC[T]{ring: ring}, nil
}

// TryPush transfers value on success. On failure, the caller retains ownership.
func (q *MPSC[T]) TryPush(value batch.Batch[T]) bool {
	return q != nil && q.ring != nil && q.ring.tryPush(value)
}

// TryPop returns one owned value when an item is ready.
func (q *MPSC[T]) TryPop(value *batch.Batch[T]) bool {
	return q != nil && q.ring != nil && q.ring.tryPop(value)
}

// Len returns a bounded, non-linearizable occupancy observation.
func (q *MPSC[T]) Len() uint32 {
	if q == nil || q.ring == nil {
		return 0
	}
	return q.ring.len()
}

// Ops returns typed callbacks for queue.Factory integration.
func (q *MPSC[T]) Ops() QueueOps[T] {
	if q == nil || q.ring == nil {
		return QueueOps[T]{}
	}
	return q.ring.operations()
}

// SPMC is a bounded typed queue for one producer and multiple consumers.
type SPMC[T any] struct{ ring *concurrentRing[T] }

// NewSPMC allocates a bounded SPMC queue and all of its slot metadata.
func NewSPMC[T any](capacity uint32) (*SPMC[T], error) {
	ring, err := newConcurrentRing[T](capacity)
	if err != nil {
		return nil, err
	}
	return &SPMC[T]{ring: ring}, nil
}

// TryPush transfers value on success. On failure, the caller retains ownership.
func (q *SPMC[T]) TryPush(value batch.Batch[T]) bool {
	return q != nil && q.ring != nil && q.ring.tryPush(value)
}

// TryPop returns one owned value when an item is ready.
func (q *SPMC[T]) TryPop(value *batch.Batch[T]) bool {
	return q != nil && q.ring != nil && q.ring.tryPop(value)
}

// Len returns a bounded, non-linearizable occupancy observation.
func (q *SPMC[T]) Len() uint32 {
	if q == nil || q.ring == nil {
		return 0
	}
	return q.ring.len()
}

// Ops returns typed callbacks for queue.Factory integration.
func (q *SPMC[T]) Ops() QueueOps[T] {
	if q == nil || q.ring == nil {
		return QueueOps[T]{}
	}
	return q.ring.operations()
}

// MPMC is a bounded typed queue for multiple producers and multiple consumers.
type MPMC[T any] struct{ ring *concurrentRing[T] }

// NewMPMC allocates a bounded MPMC queue and all of its slot metadata.
func NewMPMC[T any](capacity uint32) (*MPMC[T], error) {
	ring, err := newConcurrentRing[T](capacity)
	if err != nil {
		return nil, err
	}
	return &MPMC[T]{ring: ring}, nil
}

// TryPush transfers value on success. On failure, the caller retains ownership.
func (q *MPMC[T]) TryPush(value batch.Batch[T]) bool {
	return q != nil && q.ring != nil && q.ring.tryPush(value)
}

// TryPop returns one owned value when an item is ready.
func (q *MPMC[T]) TryPop(value *batch.Batch[T]) bool {
	return q != nil && q.ring != nil && q.ring.tryPop(value)
}

// Len returns a bounded, non-linearizable occupancy observation.
func (q *MPMC[T]) Len() uint32 {
	if q == nil || q.ring == nil {
		return 0
	}
	return q.ring.len()
}

// Ops returns typed callbacks for queue.Factory integration.
func (q *MPMC[T]) Ops() QueueOps[T] {
	if q == nil || q.ring == nil {
		return QueueOps[T]{}
	}
	return q.ring.operations()
}
