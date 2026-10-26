// Package queue defines typed bounded queue contracts and preallocated
// single- and multi-producer/consumer queue implementations.
//
// Local ordinary and error edges use non-blocking TryPush and TryPop operations
// on one group worker. Wait retains a full-edge batch for a later pass so
// downstream work can progress; Drop releases it. A Factory constructs fresh
// operations only for each started local edge and receives Capacity,
// BatchCapacity, and Policy. Graph definitions select a queue model from
// producer and consumer cardinalities; runtime construction uses the bounded
// SPSC, MPSC, SPMC, or MPMC implementation required by that topology.
//
// NewMPSC, NewSPMC, and NewMPMC return preallocated bounded typed queues. Their
// Try operations never wait for capacity or another caller: contention, full or
// empty state, or an unpublished head reservation returns false. A successful
// push transfers ownership; a failed push leaves ownership with the caller.
// Len is a bounded concurrent observation, not a linearizable snapshot of
// reservations and publications. Queue callers own custom Factory captures and
// cleanup resources.
// Config.NUMA and typed graph allocator descriptors are inactive metadata;
// queue construction continues to use the portable Go implementation.
package queue
