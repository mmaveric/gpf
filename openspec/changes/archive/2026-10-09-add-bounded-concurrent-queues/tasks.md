## 1. Queue Models
- [x] 1.1 Confirm `fix-moved-input-ownership` is complete and add constructor/ownership conformance fixtures; verify invalid capacities reject without usable storage.
- [x] 1.2 Implement the preallocated MPSC model; verify multiple publishers deliver unique batches to one consumer with nonblocking unavailable results.
- [x] 1.3 Implement the preallocated SPMC model; verify one publisher and multiple consumers never dequeue the same slot twice.
- [x] 1.4 Implement the preallocated MPMC model; verify simultaneous publishers/consumers preserve initialized contents and bounded capacity.
## 2. Concurrency and Measurement
- [x] 2.1 Add delayed-reservation and repeated cursor-wrap tests for all models; verify Try operations return rather than waiting for another caller.
- [x] 2.2 Add race/stress tests and serial allocation gates; verify no successful publication is lost or duplicated and framework allocations remain zero.
- [x] 2.3 Record same-toolchain model benchmarks and document Len observation limits; verify `go test -race ./...` and `openspec validate add-bounded-concurrent-queues --strict` pass.
