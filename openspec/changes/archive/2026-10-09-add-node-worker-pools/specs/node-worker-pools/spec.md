## Purpose
Specify fixed node-level worker expansion with independent state and typed shared batch queues.
## ADDED Requirements
### Requirement: Fixed node worker expansion
A logical node SHALL support a positive fixed worker count with explicit unique nonnegative CPU bindings. Every worker SHALL own a fresh factory result and per-input typed handlers/selectors. Shared input batches SHALL be processed by one worker, never broadcast implicitly. Worker identities SHALL be stable and observable.
#### Scenario: Process pooled input
- **WHEN** multiple workers consume accepted unique input batches
- **THEN** each batch is invoked once by one worker and state mutation remains local to that worker
### Requirement: Validate pooled topology and lifecycle
Build SHALL reject invalid counts, missing CPUs, and overlap with whole-group replication without invoking factories. Drain and Close SHALL include every physical worker and return queued/pending work once. No global output ordering SHALL be promised.
#### Scenario: Reject overlapping expansion
- **WHEN** a node pool is requested inside a replicated group
- **THEN** validation fails before any runtime instance is created

### Requirement: Per-port shared transport without implicit broadcast
Each logical input of a pooled node SHALL have one bounded shared queue with one consumer owner per physical worker, and each logical output SHALL use a queue selected from its physical producer/consumer cardinality. A batch SHALL be acquired from a producer-local pool and accepted by at most one downstream consumer. ReadyMask selection SHALL remain typed and local to the worker that invokes the selected handler. No ordering or fairness guarantee beyond accepted ownership SHALL be added.

#### Scenario: Route heterogeneous ready inputs
- **WHEN** two differently typed logical inputs are ready across a pooled node's workers
- **THEN** each worker's selector receives only its own ReadyMask and dispatches a matching typed handler exactly once
