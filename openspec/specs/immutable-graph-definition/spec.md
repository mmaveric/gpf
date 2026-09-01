## Purpose
Define transactional graph building and immutable reusable graph definitions.

## Requirements

### Requirement: Transactional graph build
The graph package SHALL expose exactly `func (b *Builder) Build() (*Definition, error)`. A nil receiver or zero-value or otherwise uninitialized builder SHALL return `(nil, ErrInvalidBuilder)` without panic, owner allocation, or receiver mutation. Build on an initialized open builder SHALL consume the complete `ValidationReport` from `add-graph-dag-validation` before changing builder state. A report containing issues SHALL return `(nil, *ValidationError)`, SHALL NOT cache a partial definition or seal the builder, and SHALL leave valid subsequent registration and connection corrections available. An issue-free report SHALL produce and atomically cache one definition, seal all topology mutation, and return that definition with a nil error.

`ErrInvalidBuilder` and `ErrBuilderSealed` SHALL be the shared sentinels supplied by the `implement-typed-graph-builder` prerequisite; this capability SHALL NOT define duplicate sentinels. Registration and connection mutation after successful Build SHALL return `ErrBuilderSealed`. Build on that sealed builder SHALL return its cached definition and nil rather than `ErrBuilderSealed`.

#### Scenario: Reject nil and zero builders
- **WHEN** Build is called through a nil builder pointer or a zero-value builder
- **THEN** it returns a nil definition and `ErrInvalidBuilder` without panic or mutation

#### Scenario: Correct topology after failed Build
- **WHEN** Build reports validation issues and the caller makes valid registration or connection corrections on the same builder
- **THEN** the corrections are accepted and a later Build can validate the corrected topology

#### Scenario: Seal mutation after successful Build
- **WHEN** Build succeeds and the caller attempts to register a node or connect an edge
- **THEN** the mutation is rejected with the prerequisite's `ErrBuilderSealed` sentinel and the built definition is unchanged

### Requirement: Idempotent successful definition
After the first successful Build, every subsequent Build call on that sealed builder SHALL return the same cached non-nil `*Definition` pointer and a nil error without rebuilding, revalidating, or changing topology metadata. Successful Build SHALL be the only sealing transition, and every sealed builder SHALL therefore have that cached pointer.

#### Scenario: Repeat successful Build
- **WHEN** a caller invokes Build repeatedly after the first successful Build
- **THEN** every call returns the identical cached definition and the same canonical topology

### Requirement: Validation report is the build topology source
The `ValidationReport` consumed by Build SHALL contain sorted validation issues, canonical forward adjacency, canonical reverse adjacency, and canonical topological order. Each adjacency outer slice SHALL be dense by `NodeID-1`; each forward inner slice SHALL contain `EdgeID` values ordered by destination `NodeID` then `EdgeID`, and each reverse inner slice SHALL contain `EdgeID` values ordered by source `NodeID` then `EdgeID`. An issue-free report SHALL contain every node exactly once in canonical topological order. Build SHALL copy these report values and SHALL NOT recompute adjacency or topological order.

#### Scenario: Consume canonical report topology
- **WHEN** Build receives an issue-free `ValidationReport` for a DAG with multiple valid traversal or topological orders
- **THEN** the definition stores copies of the report's canonical forward adjacency, reverse adjacency, and topological order without deriving alternatives

#### Scenario: Reject a report with issues transactionally
- **WHEN** the `ValidationReport` contains one or more sorted validation issues
- **THEN** Build returns the corresponding `ValidationError`, retains no report topology in a definition, and leaves the builder open

### Requirement: Opaque structurally immutable topology
`Definition` SHALL have only unexported fields and SHALL own dense copied node metadata, dense copied edge metadata, deeply copied forward and reverse adjacency, and a copied canonical topological order from `ValidationReport`. After publication, framework code SHALL NOT mutate those fields or any nested backing storage.

This capability SHALL add exactly `func (d *Definition) NodeCount() int`, `func (d *Definition) EdgeCount() int`, and `func (d *Definition) TopologicalOrder() []NodeID` as public definition inspection methods, and SHALL NOT add a public adjacency accessor. Count methods SHALL return zero for a nil receiver or zero-value definition. `TopologicalOrder` SHALL return nil for a nil receiver or zero-value definition; for a valid definition it SHALL allocate a new slice of length `NodeCount`, copy the stored order, and return that clone, including a non-nil empty slice for a valid zero-node definition.

#### Scenario: Mutate returned topology data
- **WHEN** a caller changes a non-empty slice returned by `TopologicalOrder`
- **THEN** later reads return the original definition topology and canonical order

#### Scenario: Inspect nil and zero definitions
- **WHEN** a caller invokes the three inspection methods on a nil receiver or zero-value definition
- **THEN** both counts are zero and `TopologicalOrder` is nil

#### Scenario: Preserve the validated canonical order
- **WHEN** Build succeeds for a DAG with more than one valid topological ordering
- **THEN** the definition stores the validator's canonical topological order rather than deriving another order from storage iteration

#### Scenario: Detach from builder storage
- **WHEN** a definition is built from topology held in mutable builder storage
- **THEN** every slice-backed topology value in the definition is owned by the definition and cannot be changed through an alias to builder storage

### Requirement: Persistent graph identity
Every successfully built definition SHALL retain the exact non-zero private owner identity established for its builder by `NewBuilder`. The owner SHALL be immutable identity only and SHALL NOT reference mutable builder topology or lifecycle state. Build SHALL NOT substitute numeric node or edge identities or a newly allocated owner for that identity, and definition APIs SHALL NOT expose a way to read, construct, replace, mutate, or forge it. A nil receiver or zero-value definition SHALL have no valid owner and SHALL never be returned by successful Build.

#### Scenario: Retain handle provenance after Build
- **WHEN** a future consumer checks a builder-created typed handle against the successfully built definition
- **THEN** the handle remains attributable to that definition while a handle from another builder does not

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
