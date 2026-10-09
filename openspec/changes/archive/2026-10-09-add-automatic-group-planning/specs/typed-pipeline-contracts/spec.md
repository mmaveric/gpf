## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
Automatic group planning SHALL be an explicit inspection opt-in, separate from executable Build/Start. Builder.PlanGroups SHALL expand existing fixed group replicas and node worker pools, obtain immutable hard constraints, run deterministic remote-edge minimization, select queue models from resulting physical producer/consumer worker counts, and return copied stable node/group/edge rows plus required CPU slots and CPU-completeness status. It SHALL preserve explicit group labels and mandatory worker/ingress/replica boundaries. Effective DefaultGroupID assignments SHALL not become hard labels in automatic preview. The preview SHALL carry builder ownership/revision freshness and mutation of builder topology/placement/connections SHALL make prior previews stale. Preview SHALL invoke no user factories, create no runtime resources, not seal the builder, and SHALL NOT change current manual Build/Start semantics. Dynamic resizing, discovery, and execution from a CPU-incomplete preview remain unavailable.

#### Scenario: Review manual and automatic plan side by side
- **WHEN** an opt-in preview is requested and then the definition is built in current manual mode
- **THEN** preview rows are independent and exploratory while executable placements remain the existing manual fixed plan

#### Scenario: Preserve physical hard constraints
- **WHEN** a preview contains replicas, node pools, explicit placements, or ingress boundaries
- **THEN** resulting groups never merge across those mandatory constraints and required CPU slots remain visible

#### Scenario: Reject stale preview use
- **WHEN** registration, connection, or placement changes after a preview
- **THEN** the preview's freshness check reports stale and it cannot be treated as executable plan data

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews graph, queue, node, and runtime documentation after building a valid manually placed graph with CPU bindings
- **THEN** docs distinguish fixed executable manual placement from pure automatic previews and unavailable automatic execution

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph with observable factories, lifecycle callbacks, explicit CPUs, and queue-plan preview is built
- **THEN** Build retains immutable plans without invoking callbacks or creating queues, workers, state, or affinity outcomes

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a manual definition starts and later restarts after the prior executor stops
- **THEN** it creates fresh state and queues on the same bound plan; automatic preview data is not executed

#### Scenario: Build without runtime error resource creation
- **WHEN** a graph with typed error routes is built while automatic planning is enabled
- **THEN** no error pool, queue, batch, or error value is created by Build or preview

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
