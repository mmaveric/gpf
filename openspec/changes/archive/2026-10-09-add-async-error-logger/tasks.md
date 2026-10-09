## 1. Bounded Logging
- [x] 1.1 Confirm `add-error-mapping`, `add-runtime-metrics`, and `add-drain-timeout` are complete; verify their contract tests before integration.
- [x] 1.2 Add per-physical-worker preallocated event rings; verify full publication drops immediately and does not block workers.
- [x] 1.3 Activate one typed mapping/publication per failed return; verify logging remains independent of error-port absence, exhaustion, and queue Drop.
- [x] 1.4 Start and fairly poll an executor-owned logger; verify callbacks execute off data workers and disabled logging creates no worker.
## 2. Lifecycle and Verification
- [x] 2.1 Add accepted/dropped/delivered counters and shared-budget flushing; verify cancellation discards leftovers and returning callbacks permit worker joining.
- [x] 2.2 Add logging saturation race tests and serial mapping/publication allocation gates; verify framework zero allocation and exactly-once accepted-event outcomes.
- [x] 2.3 Reconcile current logging deferral deltas/docs and run `go test -race ./...` and `openspec validate add-async-error-logger --strict`; verify callback shutdown limits are explicit.
