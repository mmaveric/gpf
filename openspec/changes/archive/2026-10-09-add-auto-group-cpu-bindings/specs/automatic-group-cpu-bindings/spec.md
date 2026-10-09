## Purpose
Define explicit CPU assignment and immutable executable finalization for automatic physical group plans.
## ADDED Requirements
### Requirement: Validate plan-bound CPU assignments
Automatic finalization SHALL require one unique nonnegative explicit CPU for every physical group. Foreign/stale plan revisions and missing/duplicate/negative bindings SHALL reject before sealing or factory invocation. Topology mutation SHALL require a fresh preview and CPU assignment.
#### Scenario: Reject a stale CPU plan
- **WHEN** topology changes after CPUs were associated with an earlier preview
- **THEN** executable finalization rejects the stale assignments without starting workers
### Requirement: Execute immutable automatic plans
A successfully finalized automatic Definition SHALL retain immutable physical groups and CPU assignments and create fresh resources per sequential Start. Every physical worker SHALL lock its thread and attempt affinity with the existing portable fallback outcome contract.
#### Scenario: Start a finalized plan
- **WHEN** a complete automatic definition starts on a platform rejecting affinity
- **THEN** every planned worker remains thread-locked and reports its assigned CPU and fallback outcome
