## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
An automatic grouping preview SHALL remain non-executable until `FinalizeAutomaticPlan` consumes that current preview and exactly one unique nonnegative CPU binding is supplied for every proposed physical group. `BindAutomaticPlanCPU` SHALL validate owner/revision/group identity, reject duplicate CPU and group assignments, preserve already hard-bound CPU values, and not change the manual plan. Incomplete, duplicate, negative, foreign, or stale assignments SHALL reject before callbacks or sealing. Successful finalization SHALL freeze physical nodes/groups/edges, selected typed queue models, replica and worker identities, and CPU bindings in an immutable definition. Ordinary Builder.Build and manual Start semantics SHALL remain unchanged unless a CPU-complete plan is explicitly finalized. Every physical worker then follows the existing locked-thread and portable affinity-fallback contract; queue ownership/Wait/Drop semantics remain unchanged.

#### Scenario: Keep manual Build as the default
- **WHEN** automatic preview is enabled but no explicit finalization is requested
- **THEN** Builder.Build continues to use the existing manual topology and placements rather than applying preview merges

#### Scenario: Reject incomplete or invalid automatic CPU assignment
- **WHEN** finalization is requested with missing, negative, duplicate, foreign, or stale group CPU bindings
- **THEN** it rejects before sealing or invoking factories

#### Scenario: Start an immutable CPU-complete plan
- **WHEN** a current automatic preview receives one valid unique CPU per physical group and is finalized
- **THEN** Build returns the finalized definition and Start creates one fresh locked worker per physical group with selected queue plans

#### Scenario: Reuse automatic plan sequentially
- **WHEN** a finalized automatic executor stops and the same definition starts again
- **THEN** runtime state and affinity results are fresh while logical topology and physical CPU/queue plans remain unchanged

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** documentation is reviewed after automatic CPU finalization is implemented
- **THEN** fixed manual execution remains the default and automatic execution requires explicit preview, assignment, and finalization

#### Scenario: Build without runtime resource creation
- **WHEN** a graph or preview is built before automatic finalization
- **THEN** factories and runtime queue/worker resources remain dormant

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a finalized plan starts sequentially
- **THEN** every physical group gets fresh resources on its bound CPU

#### Scenario: Build without runtime error resource creation
- **WHEN** a typed-error graph is previewed or built before finalization
- **THEN** no error pool, batch, queue, or error value is created

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** documentation is reviewed after automatic plan finalization support is added
- **THEN** the current fixed manual executor and explicit automatic-finalization stages are distinguished

#### Scenario: Build without runtime resource creation
- **WHEN** a graph is built before automatic CPU finalization
- **THEN** only immutable topology and dormant numeric plans exist; factories and workers remain uncreated

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a finalized automatic plan starts sequentially
- **THEN** fresh state, queues, workers, and affinity outcomes match its retained physical plan

#### Scenario: Build without runtime error resource creation
- **WHEN** a typed-error graph is built before CPU finalization
- **THEN** no error pool, batch, queue, or error value is created

#### Scenario: Review manual and automatic plan side by side
- **WHEN** an opt-in preview is requested and then the definition is built in current manual mode
- **THEN** preview rows are independent and exploratory while executable placements remain the existing manual fixed plan

#### Scenario: Preserve physical hard constraints
- **WHEN** a preview contains replicas, node pools, explicit placements, or ingress boundaries
- **THEN** resulting groups never merge across those mandatory constraints and required CPU slots remain visible

#### Scenario: Reject stale preview use
- **WHEN** registration, connection, or placement changes after a preview
- **THEN** the preview's freshness check reports stale and it cannot be treated as executable plan data

#### Scenario: Start materializes a fixed manual plan
- **WHEN** a definition starts after a constraints preview
- **THEN** it executes the original manual or replicated fixed plan without applying preview merges

#### Scenario: Keep typed error resources dormant
- **WHEN** a graph with typed error edges builds while exposing planning constraints
- **THEN** no error pool, queue, batch, or error value is created by Build or preview

#### Scenario: Start materializes a fresh CPU-bound manual plan
- **WHEN** a caller starts the unchanged manual definition after inspecting constraints
- **THEN** it creates fresh resources on the original group and CPU plan without applying automatic merges

#### Scenario: Preserve error resource dormancy at Build
- **WHEN** a built graph has typed error routes alongside constraint preview data
- **THEN** Build still creates no error queue, pool, batch, or failure value

#### Scenario: Leave unplaced nodes free only in automatic preview
- **WHEN** automatic preview is enabled for unplaced nodes
- **THEN** those nodes have no hard group label in the returned constraints while ordinary manual Build remains assigned to DefaultGroupID

#### Scenario: Preserve ordinary Connect cardinality
- **WHEN** a caller attempts a second ordinary producer for an already-connected input without FanIn
- **THEN** ordinary Connect rejects it and preserves existing topology

#### Scenario: Review a pooled node boundary
- **WHEN** a logical node with one ordinary input and one output is assigned three fixed workers
- **THEN** copied queue plans report one shared multi-consumer input and the correctly selected multi-producer output transport while adjacent nodes retain their groups

#### Scenario: Build without runtime worker creation
- **WHEN** a valid node pool with complete CPU assignments is built
- **THEN** Build stores immutable worker metadata and dormant factories without constructing state, queue operations, or workers

#### Scenario: Start fresh pooled instances
- **WHEN** a pooled definition starts after a prior executor stops
- **THEN** every physical worker receives fresh state and affinity outcome while logical topology remains unchanged

#### Scenario: Distribute ingress across replicated consumers
- **WHEN** one unreplicated typed ingress feeds three replicas through a shared SPMC boundary
- **THEN** accepted batches are consumed once across the three replicas with independent input/output pool ownership

#### Scenario: Preserve one source pool per producer
- **WHEN** three replica workers publish through a shared MPSC boundary
- **THEN** each worker acquires from its own typed pool and downstream release returns each slot to its producing replica's pool

#### Scenario: Resolve an expanded physical edge
- **WHEN** the pure resolver receives nonzero producer/consumer worker counts and a physical ownership classification
- **THEN** it selects Local, SPSC, MPSC, SPMC, or MPMC according to the cardinality matrix and returns an error for zero or inconsistent local cardinality
