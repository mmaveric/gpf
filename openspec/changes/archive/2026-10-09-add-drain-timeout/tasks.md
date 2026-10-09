## 1. Lifecycle Budget
- [x] 1.1 Confirm ownership and base metrics changes are complete; verify existing Drain/Close lifecycle tests pass.
- [x] 1.2 Add DrainContext and cancellation checks at safe boundaries; verify uncancelled behavior matches Drain and an already-canceled context forces safe cleanup.
- [x] 1.3 Preserve context and cleanup errors with joined reporting; verify errors.Is identifies both cancellation and injected lifecycle failure.
- [x] 1.4 Add shared-budget participant hooks; verify no component creates a fresh timeout and repeated stops preserve final results.
## 2. Verification
- [x] 2.1 Add deterministic queued/pending-work cancellation tests with returning callbacks; verify exactly-once reclamation and final loss counts.
- [x] 2.2 Document cooperative callback limits, run `go test -race ./...` and `openspec validate add-drain-timeout --strict`; verify no claim of forcibly interrupting user code.
