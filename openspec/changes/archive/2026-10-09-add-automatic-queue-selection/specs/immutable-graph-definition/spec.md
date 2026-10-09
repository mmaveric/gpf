## MODIFIED Requirements

### Requirement: Opaque structurally immutable topology
`Definition` SHALL have only unexported fields and SHALL own dense copied node metadata, dense copied edge metadata, deeply copied forward and reverse adjacency, a copied canonical topological order from `ValidationReport`, and copied selected edge queue plans. After publication, framework code SHALL NOT mutate those fields or any nested backing storage.

This capability SHALL expose `func (d *Definition) NodeCount() int`, `func (d *Definition) EdgeCount() int`, `func (d *Definition) TopologicalOrder() []NodeID`, and `func (d *Definition) EdgeQueuePlans() []EdgeQueuePlan` as public definition inspection methods, and SHALL NOT add a public adjacency accessor. Count methods SHALL return zero for a nil receiver or zero-value definition. `TopologicalOrder` SHALL return nil for a nil receiver or zero-value definition; for a valid definition it SHALL allocate a new slice of length `NodeCount`, copy the stored order, and return that clone, including a non-nil empty slice for a valid zero-node definition. `EdgeQueuePlans` SHALL return nil for a nil receiver or zero-value definition and SHALL return a newly allocated independent edge-ordered slice for a valid definition, including a non-nil empty slice when there are no edges.

#### Scenario: Mutate returned topology data
- **WHEN** a caller changes a non-empty slice returned by `TopologicalOrder`
- **THEN** later reads return the original definition topology and canonical order

#### Scenario: Mutate returned edge queue plans
- **WHEN** a caller changes a non-empty slice returned by `EdgeQueuePlans`
- **THEN** later reads return the original model and physical cardinalities without altering Build or Start behavior

#### Scenario: Inspect nil and zero definitions
- **WHEN** a caller invokes the four inspection methods on a nil receiver or zero-value definition
- **THEN** both counts are zero and both slice accessors return nil

#### Scenario: Preserve the validated canonical order
- **WHEN** Build succeeds for a DAG with more than one valid topological ordering
- **THEN** the definition stores the validator's canonical topological order rather than deriving another order from storage iteration

#### Scenario: Detach from builder storage
- **WHEN** a definition is built from topology held in mutable builder storage
- **THEN** every slice-backed topology value and selected queue-plan row in the definition is owned by the definition and cannot be changed through an alias to builder storage

### Requirement: Runtime-neutral reusable definition
Build SHALL preserve the dormant typed registration and ordinary-or-error edge-binding closures that retain each configured `Stateful.Factory`, `ClosureFactory`, optional explicit `Init` and `Close` lifecycle callback, and eligible local `queue.Factory`. Build SHALL NOT invoke any of those functions; lifecycle callbacks returned only by a `ClosureFactory` SHALL remain uncreated. A definition SHALL contain no factory-produced explicit state, `ClosureReplica`, live `QueueOps`, queue implementation, executor pointer, worker, locked OS thread, affinity adapter, affinity status, pool, error batch, error value, or lifecycle status. It SHALL additionally retain only the private immutable effective manual placements, deterministic group plan, one validated CPU binding for each group, local-or-remote classification, and ordered numeric edge queue plans selected from physical endpoint cardinalities.

A valid definition SHALL expose `Start() (*Executor, error)` for the configuration-free fixed-plan executor capability. Each sequential successful Start after the prior executor stops SHALL create fresh mutable state, closure replicas, ordinary and error batch pools, queue operations matching each retained edge model, exactly one locked worker per planned group, and fresh affinity outcomes from the stored immutable CPU bindings without mutating definition identity, topology, placement, CPU binding, classification, or queue plans. Concurrent starts remain unsupported. Definition inspection methods SHALL expose only independent immutable topology and numeric queue-plan snapshots, not mutable executor resources, CPU-plan storage, queue operations, worker resources, affinity adapter, affinity status, state, lifecycle, or error-routing resources.

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a valid graph with a configured local typed error queue factory is built
- **THEN** its factory invocation count remains zero and the definition stores no pool, queue operation, batch, or error value

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** registered nodes and ordinary or error edges contain observable state, closure, and eligible local queue factories plus explicit lifecycle callbacks, every effective group has a CPU binding, and Build succeeds
- **THEN** every callback invocation count remains zero, closure lifecycle callbacks remain uncreated, and the definition stores no factory-produced state, live queue operations, pool, error batch, worker, thread, adapter, or affinity outcome

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a fixed-plan executor has stopped and the same definition starts again
- **THEN** each start obtains fresh factory-created resources, one locked worker and one new affinity result per the same immutable CPU-bound group plan, while definition identity, topology, placement, CPU bindings, classification, and queue plans remain unchanged

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** an executor with local or remote error edges has stopped and the same definition starts again
- **THEN** it obtains fresh eligible local queue operations and error pools while definition identity, topology, group plan, queue plans, and CPU bindings remain unchanged

#### Scenario: Inspect during executor lifetime
- **WHEN** a definition has a running one- or multi-group executor and a caller reads count, topological-order, or queue-plan accessors
- **THEN** the accessors expose only independent immutable structural and numeric queue-plan snapshots, not executor, worker, thread, affinity, live queue, error batch, state, lifecycle, or mutable plan resources
