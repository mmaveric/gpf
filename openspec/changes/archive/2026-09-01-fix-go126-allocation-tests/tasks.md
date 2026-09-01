## 1. Serial Allocation Assertions

- [x] 1.1 Remove parallel scheduling from `TestPoolLifecycleAllocs` and `TestPoolExhaustionAllocs`, then verify both complete without panic and retain zero-allocation results with `go test ./batch -run 'TestPool(Lifecycle|Exhaustion)Allocs'`
- [x] 1.2 Remove parallel scheduling from the shared `TestAssertZeroAllocs` self-test, then verify it completes without panic with `go test ./internal/benchtest -run TestAssertZeroAllocs`
- [x] 1.3 Audit every `AssertZeroAllocs` call site for serial top-level scheduling and verify the existing graph allocation coverage still passes with `go test ./graph -run TestRemoteRingFIFOConcurrentAndAllocs`

## 2. Compatibility Verification

- [x] 2.1 Run `go test ./...` with Go 1.26 and verify every package passes without an `AllocsPerRun` parallel-test panic
- [x] 2.2 Run `go test -race ./...` and verify serializing allocation assertions introduces no race or lifecycle regression
- [x] 2.3 Run `go vet ./...` and verify static analysis passes
- [x] 2.4 Run `openspec validate fix-go126-allocation-tests --strict` and `openspec validate --all --strict` and verify all change and repository specifications pass
