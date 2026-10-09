## 1. Admission and Binding
- [x] 1.1 Confirm `fix-moved-input-ownership` and `add-runtime-metrics` are complete; verify current ownership and snapshot tests pass before extending them.
- [x] 1.2 Add typed ingress registration and executor-local binding; verify zero/foreign/stale handles reject and Build keeps binders dormant.
- [x] 1.3 Materialize fresh pools/SPSC adapter queues per Start; verify repeated starts do not share admission or storage.
- [x] 1.4 Implement Acquire/Commit status and transfer behavior; verify accepted/Drop clear ownership while full Wait/closed rejection preserve it.
- [x] 1.5 Implement copying Submit; verify empty, oversized, exhaustion, and full-queue cases have no partial delivery or slot leaks.
## 2. Stop and Verification
- [x] 2.1 Close admission with an in-flight protocol before stop; verify race tests cover accepted-or-rejected commits and caller release after stop.
- [x] 2.2 Add serial allocation gates and ingress-to-sink tests; verify local/remote delivery and zero framework steady-state allocations.
- [x] 2.3 Update current ingress deferral deltas/docs, run `go test -race ./...` and `openspec validate add-typed-ingress --strict`; verify preserved prior capability requirements.
