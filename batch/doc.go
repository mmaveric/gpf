// Package batch defines typed batch and pool contracts.
//
// This package does not yet allocate storage or execute ownership transfers.
// Future handlers receive ownership of an input batch. They may move it only to
// a compatible output; the runtime releases an input that was not moved. Pools
// are fixed-capacity, and exhaustion drops work rather than allocating during
// steady-state processing.
package batch
