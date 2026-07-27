package batch

// Batch is a small typed handle to storage owned by a Pool.
//
// Its storage is allocated and managed by a future runtime. The zero value is
// an empty handle and is useful only for declaring contracts before that
// runtime exists.
type Batch[T any] struct {
	values []T
}

// Len returns the number of elements currently visible through the batch.
func (b Batch[T]) Len() int {
	return len(b.values)
}

// Cap returns the storage capacity reserved for the batch.
func (b Batch[T]) Cap() int {
	return cap(b.values)
}

// PoolConfig defines the fixed storage limits for a typed pool.
//
// A future builder rejects zero capacities. When ClearOnRelease is false,
// pointer-containing element values can be retained by released storage.
type PoolConfig struct {
	Capacity       uint32
	BatchCapacity  uint32
	ClearOnRelease bool
}

// Pool owns preallocated storage for batches of T.
//
// Allocation, acquisition, commit, and release are intentionally deferred to
// the batch-memory implementation stage.
type Pool[T any] struct {
	_ [0]func(T)
}
