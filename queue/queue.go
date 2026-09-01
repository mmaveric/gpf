package queue

import "github.com/asharipov/gpf/batch"

// Policy controls how a future edge handles a full queue.
type Policy uint8

const (
	// Drop releases a batch when it cannot be queued.
	Drop Policy = iota
	// Wait retains a batch for retry on a later scheduler pass.
	Wait
)

// QueueOps defines allocation-free operations for a typed batch queue.
//
// Callers providing custom operations are responsible for non-blocking behavior
// and FIFO behavior. The single-group executor invokes them on one worker.
type QueueOps[T any] struct {
	TryPush func(batch.Batch[T]) bool
	TryPop  func(*batch.Batch[T]) bool
	Len     func() uint32
}

// Factory constructs fresh operations for one future runtime edge.
//
// Captures belong to the caller and must remain safe for later use. Each
// runtime invocation must return operations independent from other runtimes.
type Factory[T any] func(Config) (QueueOps[T], error)

// Config defines a bounded future queue.
type Config struct {
	Capacity      uint32
	BatchCapacity uint32
	Policy        Policy
}
