## 1. Pooled Nodes
- [x] 1.1 Confirm queue selection, ownership, and base metrics are complete; verify current node lifecycle tests.
- [x] 1.2 Add count/CPU metadata and overlap validation; verify invalid pools fail without invoking factories.
- [x] 1.3 Expand one logical node into stable physical workers and shared typed queues; verify per-port cardinality selection and no implicit broadcast.
- [x] 1.4 Materialize worker-local explicit/closure state and selectors; verify each accepted input is processed once and state remains independent.
## 2. Integration
- [x] 2.1 Add per-worker affinity/metric identity and rollback/stop coverage; verify all workers join and every successful factory lifecycle closes once.
- [x] 2.2 Run race, serial allocation, and fixed-count benchmark fixtures; verify count-one behavior and no dynamic resizing.
- [x] 2.3 Reconcile current parallelism deferral deltas/docs, regenerate, and run `openspec validate add-node-worker-pools --strict` plus affected Go tests.
