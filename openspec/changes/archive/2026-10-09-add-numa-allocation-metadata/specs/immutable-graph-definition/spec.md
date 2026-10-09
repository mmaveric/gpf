## MODIFIED Requirements

### Requirement: Opaque structurally immutable topology
`Definition` SHALL own copied dense node/edge metadata, canonical adjacency/order, selected queue plans, automatic constraints, and inactive NUMA annotations. It SHALL expose only NodeCount, EdgeCount, TopologicalOrder, EdgeQueuePlans, Constraints, GroupNUMA, and Start; no mutable runtime resource accessor is added. Every returned slice SHALL be an independent copy; nil and zero-value definitions return zero counts and nil snapshots. GroupNUMA rows SHALL be ordered by physical group identity and include logical group, replica index, and explicit-presence NUMA placement.

#### Scenario: Mutate returned topology data
- **WHEN** a caller mutates TopologicalOrder, EdgeQueuePlans, Constraints, or GroupNUMA results
- **THEN** subsequent inspection and execution retain the original definition metadata

#### Scenario: Mutate returned topology and constraints
- **WHEN** a caller changes any rows returned by TopologicalOrder, EdgeQueuePlans, or Constraints
- **THEN** later reads and execution preserve the original immutable definition plan

#### Scenario: Mutate returned edge queue plans
- **WHEN** a caller changes a non-empty slice returned by `EdgeQueuePlans`
- **THEN** later reads return the original model and physical cardinalities without altering Build or Start behavior

#### Scenario: Inspect nil and zero definitions
- **WHEN** inspection methods are invoked on nil or zero-value definitions
- **THEN** they return zero counts and nil slice snapshots

#### Scenario: Preserve validated canonical order
- **WHEN** Build succeeds for a DAG with multiple valid topological orders
- **THEN** the stored canonical order is the validator-provided order

#### Scenario: Detach all metadata from builder storage
- **WHEN** a definition is built and the original builder placement/NUMA inputs are changed
- **THEN** returned group and edge annotations remain the copied values

#### Scenario: Preserve the validated canonical order
- **WHEN** Build succeeds for a DAG with more than one valid topological ordering
- **THEN** the definition stores the validator's canonical topological order

#### Scenario: Detach from builder storage
- **WHEN** a definition is built from topology held in mutable builder storage
- **THEN** every slice-backed topology value is owned by the definition and cannot be changed through a builder alias

### Requirement: Runtime-neutral reusable definition
Build SHALL copy optional pool/group/queue NUMA annotations and typed pool/queue allocator descriptors as inactive metadata. Build and sequential Start SHALL never invoke those allocator descriptors, discover host NUMA topology, select local memory, change affinity, or alter portable allocation. Annotations SHALL be visible only through copied immutable GroupNUMA/EdgeQueuePlans and pool metric metadata. The definition SHALL still contain no factory-produced state, live allocator-owned storage, queue, pool, worker, affinity adapter/outcome, or lifecycle state. Factories, error resources, queues, and metrics remain dormant at Build and fresh at each Start.

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a valid graph with typed error configuration is built
- **THEN** no error factory, pool, queue, batch, or error value is created

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** a graph has observable factories and complete CPU/NUMA metadata
- **THEN** Build invokes no callback and stores no runtime resources or affinity outcome

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a definition starts again after stop
- **THEN** runtime resources are fresh while topology, CPU bindings, and NUMA annotations remain unchanged

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** a stopped error-routed definition restarts
- **THEN** fresh error pools and queue operations are created without changing copied metadata

#### Scenario: Inspect during executor lifetime
- **WHEN** group NUMA metadata is inspected during a running executor
- **THEN** only copied annotations are exposed, never mutable allocator/worker/queue resources

#### Scenario: Inspect during executor lifetime
- **WHEN** a caller inspects a running definition
- **THEN** accessors expose only copied logical topology and plan metadata

#### Scenario: Keep factories dormant through finalization
- **WHEN** a complete preview is assigned CPUs and finalized
- **THEN** no state, closure, or queue factory has run and the definition contains only immutable physical plans and dormant bindings

#### Scenario: Reuse a finalized automatic definition
- **WHEN** its executor stops and the same automatic definition starts again
- **THEN** fresh resources and affinity outcomes are created while logical topology and physical plan inspection remain unchanged

#### Scenario: Keep error factories and pools dormant during Build
- **WHEN** a graph contains a typed error factory before automatic finalization
- **THEN** Build invokes no error factory and stores no live pool or queue

#### Scenario: Keep factories and CPU plan dormant during Build
- **WHEN** an automatic preview is built with dormant node factories
- **THEN** no callback or runtime state is created before finalization and Start

#### Scenario: Reuse a CPU-bound multi-group definition sequentially
- **WHEN** a CPU-bound physical definition restarts
- **THEN** it creates fresh resources and affinity results without changing topology or bindings

#### Scenario: Reuse a definition with error routing sequentially
- **WHEN** an automatic definition with typed error edges restarts
- **THEN** error pools and queue operations are fresh

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

#### Scenario: Inspect logical topology with physical expansion
- **WHEN** a caller reads topology accessors on a replicated definition
- **THEN** logical counts/order remain unchanged, returned storage is independent, and physical expansion remains inaccessible except through copied read-only execution-plan/affinity/metrics identities
