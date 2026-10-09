## MODIFIED Requirements

### Requirement: Opaque structurally immutable topology
`Definition` SHALL have only unexported fields and SHALL own dense copied node metadata, dense copied edge metadata, deeply copied forward and reverse adjacency, a copied canonical topological order from `ValidationReport`, copied selected edge queue plans, and copied automatic-grouping constraint rows. After publication, framework code SHALL NOT mutate those fields or any nested backing storage.

This capability SHALL expose `func (d *Definition) NodeCount() int`, `func (d *Definition) EdgeCount() int`, `func (d *Definition) TopologicalOrder() []NodeID`, `func (d *Definition) EdgeQueuePlans() []EdgeQueuePlan`, and `func (d *Definition) Constraints() PlanningConstraints` as public inspection methods, and SHALL NOT add public adjacency accessors. Count methods SHALL return zero for nil/zero definitions; slice/map-shaped previews SHALL return nil for nil/zero receivers and newly allocated independent slices for valid definitions.

#### Scenario: Mutate returned topology and constraints
- **WHEN** a caller changes any rows returned by TopologicalOrder, EdgeQueuePlans, or Constraints
- **THEN** later reads and execution preserve the original immutable definition plan

#### Scenario: Mutate returned topology data
- **WHEN** a caller changes a non-empty TopologicalOrder slice
- **THEN** later reads return the original canonical order

#### Scenario: Mutate returned edge queue plans
- **WHEN** a caller changes a non-empty slice returned by `EdgeQueuePlans`
- **THEN** later reads return the original model and physical cardinalities without altering Build or Start behavior

#### Scenario: Inspect nil and zero definitions
- **WHEN** a caller invokes inspection methods on a nil receiver or zero-value definition
- **THEN** counts are zero and all slice-shaped accessors return nil

#### Scenario: Preserve the validated canonical order
- **WHEN** Build succeeds for a DAG with more than one valid topological ordering
- **THEN** the definition stores the validator's canonical order

#### Scenario: Detach from builder storage
- **WHEN** a definition is built from mutable builder topology and constraint metadata
- **THEN** returned previews have no alias to builder or definition storage

### Requirement: Runtime-neutral reusable definition
Build SHALL preserve dormant typed registration and edge-binding closures, factory/lifecycle callbacks, immutable effective manual placement and deterministic group plan, validated CPU bindings, local-or-remote classifications, numeric edge queue plans, and numeric placement/boundary constraints. Build SHALL NOT invoke callbacks or retain factory-produced state, live queue operations, queue implementations, pools, batches, executor state, workers, locked threads, affinity adapter/outcomes, errors, or lifecycle status. Constraint preview SHALL be pure control-plane data and SHALL NOT enable automatic merge or execution. A valid definition SHALL expose Start; each sequential Start SHALL create fresh mutable resources without changing the definition's topology, CPU bindings, classifications, queue plans, or constraint rows.

#### Scenario: Keep factories and resource construction dormant during Build
- **WHEN** a valid graph has observable factories, lifecycle callbacks, queues, and constraints
- **THEN** Build leaves callback counts at zero and stores only immutable plans, not runtime resources

#### Scenario: Reuse fixed and replicated definitions sequentially
- **WHEN** an executor stops and its definition starts again
- **THEN** state, queues, pools, workers, and affinity outcomes are fresh while topology and numeric previews remain unchanged

#### Scenario: Reuse a node-pool definition sequentially
- **WHEN** a stopped node-pool executor starts again from the same definition
- **THEN** each physical worker receives fresh state/resources and logical topology/worker plan storage remains unchanged

#### Scenario: Inspect constraints during executor lifetime
- **WHEN** a caller reads Constraints during execution
- **THEN** only an independent numeric snapshot is returned and no runtime resource is exposed

#### Scenario: Inspect during executor lifetime
- **WHEN** a definition has a running multi-group executor and a caller reads count, topological-order, or queue-plan accessors
- **THEN** accessors expose only independent immutable structural and numeric constraint snapshots

#### Scenario: Inspect logical topology with physical expansion
- **WHEN** a caller reads topology accessors on a replicated definition
- **THEN** logical counts/order remain unchanged, returned storage is independent, and physical expansion remains inaccessible except through copied read-only execution-plan/affinity/metrics identities

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a valid graph with a configured local typed error factory is built
- **THEN** its factory count remains zero and no pool, queue operation, batch, or error value is stored

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** observable factories and lifecycle callbacks exist on a valid explicitly CPU-bound plan
- **THEN** Build invokes no callback and stores no runtime result, pool, queue, worker, thread, or affinity outcome

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** the same fixed definition starts after a previous stop
- **THEN** runtime resources and affinity outcomes are fresh while topology, placements, and CPU bindings remain unchanged

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** a stopped definition with typed error edges starts again
- **THEN** it obtains fresh eligible queues and error pools without changing its plan
