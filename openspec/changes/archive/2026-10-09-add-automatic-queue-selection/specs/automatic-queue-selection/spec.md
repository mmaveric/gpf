## Purpose
Define deterministic built-in queue choice from physical endpoint concurrency after topology expansion.
## ADDED Requirements
### Requirement: Select the least concurrent valid model
Physical edges SHALL use local transport only when all operations have one worker owner. Cross-worker 1:1, many:1, 1:many, and many:many endpoints SHALL select SPSC, MPSC, SPMC, and MPMC respectively. Invalid zero endpoint counts SHALL be rejected. Copied edge-plan metadata SHALL report model and cardinalities.
#### Scenario: Resolve a shared consumer
- **WHEN** an expanded edge has three producer workers and one consumer worker
- **THEN** its plan selects MPSC instead of SPSC or MPMC
### Requirement: Retain factory dormancy
Build SHALL select models without creating queues or invoking callbacks. Start SHALL materialize fresh typed storage. A custom factory incompatible with physical multi-worker ownership SHALL be rejected before invocation.
#### Scenario: Reject unsafe custom transport
- **WHEN** a custom local-only factory is configured on an expanded cross-worker edge
- **THEN** validation reports the edge and leaves the factory uncalled
