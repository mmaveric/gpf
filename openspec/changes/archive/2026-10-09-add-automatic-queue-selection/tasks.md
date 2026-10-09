## 1. Planning
- [x] 1.1 Confirm `add-bounded-concurrent-queues` is complete; verify queue conformance tests before graph integration.
- [x] 1.2 Implement cardinality/model resolution; verify table cases for local, SPSC, MPSC, SPMC, MPMC, and invalid zero counts.
- [x] 1.3 Add copied per-edge physical model metadata; verify deterministic ordering and defensive copies.
## 2. Materialization and Verification
- [x] 2.1 Install dormant typed constructors in bindings; verify Build creates no storage, Start materializes the model in the retained plan, and synthetic expanded plans select the corresponding constructors.
- [x] 2.2 Reject incompatible custom factories; verify callback counts remain zero on failed validation and manual 1:1 behavior is preserved.
- [x] 2.3 Update current queue deferral deltas/docs, run allocation/race coverage and `openspec validate add-automatic-queue-selection --strict`; verify retained capability contracts.
