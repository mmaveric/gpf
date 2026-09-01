## MODIFIED Requirements

### Requirement: Runtime-neutral reusable definition
Build SHALL preserve the dormant typed registration and edge-binding closures that retain each configured `Stateful.Factory`, `ClosureFactory`, optional explicit `Init` and `Close` lifecycle callback, and `queue.Factory`. Build SHALL NOT invoke any of those functions; lifecycle callbacks returned only by a `ClosureFactory` SHALL remain uncreated. A definition SHALL contain no factory-produced explicit state, `ClosureReplica`, live `QueueOps`, queue implementation, executor pointer, or lifecycle status.

A valid definition SHALL expose `Start() (*Executor, error)` for the single-group executor capability. Each sequential successful Start after the prior executor stops SHALL create fresh mutable state, closure replicas, batch pools, and queue operations from the stored factories without mutating definition identity or topology. Concurrent starts remain unsupported. The definition's existing inspection methods SHALL continue to expose no mutable executor resources.

#### Scenario: Keep factories dormant during Build
- **WHEN** registered nodes and edges contain observable state factories, closure factories, queue factories, and explicit lifecycle callbacks and Build succeeds
- **THEN** every invocation count remains zero, closure lifecycle callbacks remain uncreated, and the definition stores no factory-produced state or live queue operations

#### Scenario: Reuse definition sequentially
- **WHEN** a caller starts, stops, and starts an executor again from one definition
- **THEN** each start obtains fresh factory-created resources while the definition identity and topology remain unchanged

#### Scenario: Inspect during executor lifetime
- **WHEN** a definition has a running executor and a caller reads its count or topological-order accessors
- **THEN** the accessors expose only the same immutable structural snapshot and no executor, queue, state, or lifecycle data
