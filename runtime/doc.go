// Package runtime defines lifecycle and execution-placement contracts.
//
// A future graph Definition is immutable after Build, and every Start creates
// a fresh runtime. CPU affinity and locked-thread execution are future platform
// responsibilities and are not available in this contract-only stage.
package runtime
