// Package batch provides preallocated typed storage for pipeline batches.
//
// Pools allocate all storage during construction. Atomic slot ownership permits
// the fixed remote SPSC execution path to acquire on its producer and release
// on its consumer. A caller owns an acquired batch until it moves or releases
// it. Batch.Release returns a handle to its allocating pool even after a
// compatible edge move. Error-routing batches use the same origin-pool release
// rule for local and remote transport, queue drains, and pending Wait cleanup.
// Mutable returns full fixed-capacity storage for indexed writes, while Items
// exposes only the explicit logical length. External ingress remains deferred.
package batch
