package batch

import (
	"errors"
	"github.com/mmaveric/gpf/numa"
	"sync/atomic"
)

var (
	// ErrInvalidPoolConfig reports a zero pool or batch capacity.
	ErrInvalidPoolConfig = errors.New("pool and batch capacities must be greater than zero")
	// ErrPoolCapacityOverflow reports a configuration that cannot fit Go slice lengths.
	ErrPoolCapacityOverflow = errors.New("pool storage capacity overflows int")
)

// PoolConfig defines fixed storage limits for a typed pool. NUMA is optional
// inactive metadata; Allocator belongs in PoolOptions and is currently dormant.
//
// When ClearOnRelease is false, pointer-containing elements can remain retained
// by reusable storage until a later write overwrites them.
type PoolConfig struct {
	Capacity       uint32
	BatchCapacity  uint32
	ClearOnRelease bool
	NUMA           numa.Placement
	OwnerID        uint32
}

// Allocator describes a typed backing-storage allocation function. Pool
// construction currently retains this descriptor but does not call it.
type Allocator[T any] func(elements int) ([]T, error)

// PoolOptions combines pool limits with an optional typed allocator
// descriptor. NewPoolWithOptions currently uses portable Go allocation and
// retains the descriptor without calling it.
type PoolOptions[T any] struct {
	Config    PoolConfig
	Allocator Allocator[T]
}

type slot[T any] struct {
	index     uint32
	values    []T
	pool      *Pool[T]
	length    int
	available atomic.Bool
}

// Batch is an owned handle for one preallocated pool slot. Its zero value is an
// empty handle. A batch is valid only until it is moved or released. A handler
// receives input batches by value; moving or releasing that copy also informs
// the executor's invocation scope so its retained cleanup copy will not release
// the transferred slot. Independently moved batches do not retain that scope.
type Batch[T any] struct {
	slot  *slot[T]
	scope *InputScope[T]
}

// Len returns the number of populated elements in the batch.
func (b Batch[T]) Len() int {
	if b.slot == nil {
		return 0
	}
	return b.slot.length
}

// Cap returns the fixed capacity of the batch storage.
func (b Batch[T]) Cap() int {
	if b.slot == nil {
		return 0
	}

	return cap(b.slot.values)
}

// Mutable returns the complete fixed-capacity storage for indexed writes.
// Callers must not append beyond this storage; SetLen publishes populated data.
func (b Batch[T]) Mutable() []T {
	if b.slot == nil {
		return nil
	}

	capacity := cap(b.slot.values)
	return b.slot.values[:capacity:capacity]
}

// Items returns the populated portion of a batch with no spare capacity.
func (b Batch[T]) Items() []T {
	if b.slot == nil {
		return nil
	}

	return b.slot.values[:b.slot.length:b.slot.length]
}

// SetLen updates the number of populated elements. It returns false when n is
// outside the fixed storage capacity and leaves the current length unchanged.
func (b *Batch[T]) SetLen(n int) bool {
	if b == nil || n < 0 || n > b.Cap() {
		return false
	}
	if b.slot == nil {
		return n == 0
	}

	b.slot.length = n
	return true
}

// Move transfers batch ownership and clears the source handle. When called on
// a scoped handler input, it relinquishes the executor's automatic cleanup for
// this invocation and detaches the returned handle from that scope.
func (b *Batch[T]) Move() Batch[T] {
	if b == nil {
		return Batch[T]{}
	}

	moved := *b
	if b.scope != nil {
		b.scope.consumed = true
		moved.scope = nil
	}
	*b = Batch[T]{}
	return moved
}

// Release returns this batch to the pool that allocated it and clears the
// handle. It remains valid after a compatible batch has moved across edges.
func (b *Batch[T]) Release() {
	if b == nil || b.slot == nil || b.slot.pool == nil {
		return
	}
	b.slot.pool.Release(b)
}

// Pool owns fixed-capacity, preallocated storage for batches of T.
//
// Pools preallocate all storage. Their atomic slot ownership also supports the
// remote SPSC path, where the producer acquires and the consumer releases.
type Pool[T any] struct {
	inUse, highWater    atomic.Uint32
	slots               []slot[T]
	clearOnRelease      bool
	allocatorDescriptor Allocator[T]
	numa                numa.Placement
	ownerID             uint32
	next                atomic.Uint32
}

// NewPool allocates all slot metadata and typed backing storage for a pool.
func NewPool[T any](config PoolConfig) (*Pool[T], error) {
	if err := config.NUMA.Validate(); err != nil {
		return nil, err
	}
	slotCount, batchCapacity, totalCapacity, err := poolSizes(config)
	if err != nil {
		return nil, err
	}

	backing := make([]T, totalCapacity)
	slots := make([]slot[T], slotCount)
	for index := range slots {
		start := index * batchCapacity
		end := start + batchCapacity
		slots[index].index = uint32(index)
		slots[index].values = backing[start:end:end]
		slots[index].available.Store(true)
	}

	pool := &Pool[T]{
		slots:          slots,
		clearOnRelease: config.ClearOnRelease,
		numa:           config.NUMA,
		ownerID:        config.OwnerID,
	}
	for index := range pool.slots {
		pool.slots[index].pool = pool
	}
	return pool, nil
}

// OwnerID returns the numeric transport owner identity configured for the
// batch's pool, or zero when the batch has no pool.
func (b Batch[T]) OwnerID() uint32 {
	if b.slot == nil || b.slot.pool == nil {
		return 0
	}
	return b.slot.pool.ownerID
}

// NewPoolWithOptions retains a typed dormant allocator descriptor while using
// the same portable allocation behavior as NewPool.
func NewPoolWithOptions[T any](options PoolOptions[T]) (*Pool[T], error) {
	pool, err := NewPool[T](options.Config)
	if err != nil {
		return nil, err
	}
	pool.allocatorDescriptor = options.Allocator
	return pool, nil
}

// Acquire returns an owned batch when a preallocated slot is available.
// Exhaustion returns an empty batch and false without allocating.
func (p *Pool[T]) Acquire() (Batch[T], bool) {
	if p == nil || len(p.slots) == 0 {
		return Batch[T]{}, false
	}
	start := p.next.Add(1) - 1
	for offset := range len(p.slots) {
		index := (start + uint32(offset)) % uint32(len(p.slots))
		slot := &p.slots[index]
		if slot.available.CompareAndSwap(true, false) {
			p.acquired()
			return Batch[T]{slot: slot}, true
		}
	}
	return Batch[T]{}, false
}

// Release returns an owned batch slot to the pool and clears the caller's
// handle. Releasing an empty handle is a no-op.
func (p *Pool[T]) Release(batch *Batch[T]) {
	if p == nil || batch == nil || batch.slot == nil {
		return
	}
	if batch.scope != nil {
		batch.scope.consumed = true
	}

	if p.clearOnRelease {
		clear(batch.slot.values)
	}

	batch.slot.length = 0
	p.inUse.Add(^uint32(0))
	batch.slot.available.Store(true)
	*batch = Batch[T]{}
}

func poolSizes(config PoolConfig) (int, int, int, error) {
	if config.Capacity == 0 || config.BatchCapacity == 0 {
		return 0, 0, 0, ErrInvalidPoolConfig
	}

	maxInt := uint64(^uint(0) >> 1)
	slotCount := uint64(config.Capacity)
	batchCapacity := uint64(config.BatchCapacity)
	totalCapacity := slotCount * batchCapacity
	if slotCount > maxInt || batchCapacity > maxInt || totalCapacity > maxInt {
		return 0, 0, 0, ErrPoolCapacityOverflow
	}

	return int(slotCount), int(batchCapacity), int(totalCapacity), nil
}
