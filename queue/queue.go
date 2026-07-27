package queue

import "github.com/asharipov/gpf/batch"

// Policy controls how a future edge handles a full queue.
type Policy uint8

const (
	// Drop releases a batch when it cannot be queued.
	Drop Policy = iota
	// Wait retries queue publication with busy-spinning.
	Wait
)

// QueueOps defines allocation-free operations for a typed batch queue.
//
// Callers providing custom operations are responsible for non-blocking behavior
// and correct synchronization. Implementations are deferred to a later stage.
type QueueOps[T any] struct {
	TryPush func(batch.Batch[T]) bool
	TryPop  func(*batch.Batch[T]) bool
	Len     func() uint32
}

// Config defines a bounded future queue.
type Config struct {
	Capacity uint32
	Policy   Policy
}
