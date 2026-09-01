## MODIFIED Requirements

### Requirement: Runtime-neutral reusable definition
Build SHALL preserve the dormant typed registration and edge-binding closures that retain each configured `Stateful.Factory`, `ClosureFactory`, optional explicit `Init` and `Close` lifecycle callback, and `queue.Factory`. Build SHALL NOT invoke any of those functions; lifecycle callbacks returned only by a `ClosureFactory` SHALL remain uncreated. A definition SHALL contain no factory-produced explicit state, `ClosureReplica`, live `QueueOps`, queue implementation, executor pointer, worker, pool, or lifecycle status. It SHALL additionally retain only the private immutable effective manual placements, deterministic group plan, and local-or-remote edge classifications supplied by the manual-multigroup-runtime capability.

A valid definition SHALL expose `Start() (*Executor, error)` for the configuration-free fixed-plan executor capability. Each sequential successful Start after the prior executor stops SHALL create fresh mutable state, closure replicas, batch pools, queue operations, and exactly one worker per planned group from the stored factories and plan without mutating definition identity, topology, placement, or classification. Concurrent starts remain unsupported. The definition's existing inspection methods SHALL continue to expose no mutable executor resources, queue operations, worker resources, or mutable plan storage.

#### Scenario: Keep factories and plan resources dormant during Build
- **WHEN** registered nodes and edges contain observable state factories, closure factories, queue factories, and explicit lifecycle callbacks and Build succeeds with one or more manual groups
- **THEN** every invocation count remains zero, closure lifecycle callbacks remain uncreated, and the definition stores no factory-produced state, live queue operations, worker, pool, or executor resource

#### Scenario: Reuse a multi-group definition sequentially
- **WHEN** a fixed-plan executor has stopped and the same definition starts again
- **THEN** each start obtains fresh factory-created resources and one worker per the same immutable group plan while definition identity, topology, placement, and classification remain unchanged

#### Scenario: Inspect during executor lifetime
- **WHEN** a definition has a running one- or multi-group executor and a caller reads its count or topological-order accessors
- **THEN** the accessors expose only the same immutable structural snapshot and no executor, worker, queue, state, lifecycle, or mutable plan data
