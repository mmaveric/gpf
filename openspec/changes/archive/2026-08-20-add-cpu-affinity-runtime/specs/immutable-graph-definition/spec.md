## MODIFIED Requirements

### Requirement: Runtime-neutral reusable definition
Build SHALL preserve the dormant typed registration and edge-binding closures that retain each configured `Stateful.Factory`, `ClosureFactory`, optional explicit `Init` and `Close` lifecycle callback, and `queue.Factory`. Build SHALL NOT invoke any of those functions; lifecycle callbacks returned only by a `ClosureFactory` SHALL remain uncreated. A definition SHALL contain no factory-produced explicit state, `ClosureReplica`, live `QueueOps`, queue implementation, executor pointer, worker, locked OS thread, affinity adapter, affinity status, pool, or lifecycle status. It SHALL additionally retain only the private immutable effective manual placements, deterministic group plan, one validated CPU binding for each group, and local-or-remote edge classifications supplied by the manual-multigroup-runtime and cpu-affinity-runtime capabilities.

A valid definition SHALL expose `Start() (*Executor, error)` for the configuration-free fixed-plan executor capability. Each sequential successful Start after the prior executor stops SHALL create fresh mutable state, closure replicas, batch pools, queue operations, exactly one locked worker per planned group, and fresh affinity outcomes from the stored immutable CPU bindings without mutating definition identity, topology, placement, CPU binding, or classification. Concurrent starts remain unsupported. The definition's existing inspection methods SHALL continue to expose no mutable executor resources, CPU-plan storage, queue operations, worker resources, affinity adapter, affinity status, state, lifecycle, or mutable plan storage.

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** registered nodes and edges contain observable state, closure, and eligible local queue factories plus explicit lifecycle callbacks, every effective group has a CPU binding, and Build succeeds
- **THEN** every callback invocation count remains zero, closure lifecycle callbacks remain uncreated, and the definition stores no factory-produced state, live queue operations, worker, thread, adapter, or affinity outcome

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a fixed-plan executor has stopped and the same definition starts again
- **THEN** each start obtains fresh factory-created resources, one locked worker and one new affinity result per the same immutable CPU-bound group plan, while definition identity, topology, placement, CPU bindings, and classification remain unchanged

#### Scenario: Inspect during executor lifetime
- **WHEN** a definition has a running one- or multi-group executor and a caller reads its count or topological-order accessors
- **THEN** the accessors expose only the same immutable structural snapshot and no executor, worker, thread, CPU binding, affinity, queue, state, lifecycle, or mutable plan data
