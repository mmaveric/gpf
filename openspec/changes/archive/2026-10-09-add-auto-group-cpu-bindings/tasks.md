## 1. Plan Finalization
- [x] 1.1 Confirm automatic preview is complete; verify deterministic physical identities and revision invalidation.
- [x] 1.2 Add explicit plan-bound CPU assignments; verify stale/foreign/missing/negative/duplicate bindings reject without sealing or callback execution.
- [x] 1.3 Freeze a CPU-complete automatic Definition; verify dormant factories and immutable plan metadata.
## 2. Execution and Verification
- [x] 2.1 Start each physical worker with lock/affinity attempt and copied outcomes; verify fallback and replica identity under injected affinity rejection.
- [x] 2.2 Add sequential-start, rollback, drain, and moved-batch fixtures; verify all physical workers/resources have fresh exactly-once ownership.
- [x] 2.3 Reconcile current automatic/immutable/affinity deltas/docs, run race/allocation tests, and run `openspec validate add-auto-group-cpu-bindings --strict`.
