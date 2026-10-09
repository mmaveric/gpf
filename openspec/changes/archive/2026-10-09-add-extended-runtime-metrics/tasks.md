## 1. Utilization and Counter Slots
- [x] 1.1 Confirm `add-runtime-metrics` and `add-processing-timing` are complete; verify base snapshot, loss-boundary, and timing tests remain passing.
- [x] 1.2 Add sampled pool utilization/high-water records; verify bounds and remote release race-safety after exhaustion/recovery.
- [x] 1.3 Add fixed pre-Build numeric user-counter registration; verify invalid/duplicate IDs reject atomically without growing runtime storage.
- [x] 1.4 Bind replica-local update functions and copied snapshot rows; verify independent counters and zero framework update allocations.
- [x] 1.5 Add optional sampled idle/work polling durations; verify fake-clock classification, no readings on disabled/unsampled passes, and no OS-CPU-time claim.
## 2. Integration
- [x] 2.1 Reconcile current timing/logger/replica metric field ownership without duplicating totals; verify existing metric counts survive extensions unchanged.
- [x] 2.2 Document sample/pass limits and run `go test -race ./...`, metric benchmarks, and `openspec validate add-extended-runtime-metrics --strict`; verify no exporter dependency is added.
