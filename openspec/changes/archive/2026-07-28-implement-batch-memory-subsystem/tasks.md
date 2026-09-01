## 1. Executable Batch and Pool API

- [x] 1.1 Replace the declaration-only batch and pool placeholders with documented executable API types, including constructor errors and validated `PoolConfig`.
- [x] 1.2 Implement `NewPool[T]` with checked capacity arithmetic, a preallocated typed backing array, fixed-capacity slot views, and an initialized free-slot stack.
- [x] 1.3 Implement owned `Batch[T]` handles with `Cap`, `Len`, full-capacity `Mutable`, length-limited `Items`, validated `SetLen`, and source-clearing `Move`.
- [x] 1.4 Implement allocation-free single-owner `Acquire` and pointer-based `Release`, including exhaustion reporting, caller-handle clearing, and configurable complete-slot clearing.
- [x] 1.5 Update batch package documentation to define ownership, single-goroutine use, pointer-retention behavior, fixed-capacity fill rules, and the deferred `Commit`/`Submit` APIs.

## 2. Lifecycle Tests

- [x] 2.1 Add table-driven constructor tests for valid configurations, zero capacities, and overflowing storage-size arithmetic.
- [x] 2.2 Add tests for controlled writable/readable slice views, valid and invalid logical lengths, move invalidation, release invalidation, reuse, and pool exhaustion.
- [x] 2.3 Add tests for full-capacity clearing with pointer-containing elements and for retained storage when clearing is disabled.
- [x] 2.4 Add fuzz coverage for pool configuration and logical-length boundaries, ensuring invalid input cannot create out-of-capacity batch state.

## 3. Allocation Measurements

- [x] 3.1 Replace the skipped pool benchmark matrix entry with an executable acquire/fill/set-length/release benchmark that reports allocations; leave unimplemented benchmark categories explicitly skipped.
- [x] 3.2 Add `AllocsPerRun` assertions for repeated pool lifecycle work and pool exhaustion after constructor setup, requiring zero allocations.
- [x] 3.3 Benchmark clear-enabled and clear-disabled pool release paths without adding throughput or latency merge gates.

## 4. Verification

- [x] 4.1 Run `gofmt` on all changed Go source and test files.
- [x] 4.2 Run `go test ./...` and `go test -race ./...` to verify the package contract and lifecycle tests.
- [x] 4.3 Run the allocation tests and `go test -run '^$' -bench . -benchmem -v ./...`, confirming the pool case executes with zero allocations and other future paths remain unavailable.
