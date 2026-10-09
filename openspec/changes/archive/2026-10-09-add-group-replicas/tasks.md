## 1. Physical Expansion
- [x] 1.1 Confirm ownership, automatic queue selection, and base metrics are complete; verify existing fixed-group tests before expansion.
- [x] 1.2 Add replica-count/CPU metadata and validation; verify zero counts, missing/duplicate CPUs, and node-pool overlap reject dormantly.
- [x] 1.3 Expand stable physical instances and pair internal edges; verify deterministic IDs, replica-local locality, and boundary cardinality models.
- [x] 1.4 Materialize one worker/state set per replica; verify independent state and once-per-instance factory calls on sequential starts.
## 2. Lifecycle and Verification
- [x] 2.1 Extend affinity/metric rows with replica identity; verify copied ordering and count-one compatibility.
- [x] 2.2 Add startup rollback, Drain/Close, and accepted-batch uniqueness tests; verify owner-worker callbacks and exactly-once reclamation under race detection.
- [x] 2.3 Reconcile current replica deferral deltas/docs, regenerate bindings, and run allocation gates and `openspec validate add-group-replicas --strict`.
