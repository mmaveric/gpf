## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
The queue contract SHALL document typed QueueOps and bounded ownership. Local single-owner edges SHALL use local queues and optional custom local factories; cross-worker edges SHALL select bounded SPSC/MPSC/SPMC/MPMC transport from physical producer/consumer cardinality. Drop releases a rejected batch; Wait retains it for nonblocking retry. A Definition SHALL expose copied read-only queue and placement constraint plans. Manual mode remains the default: unplaced nodes retain DefaultGroupID, explicit Place labels and mandatory physical cuts are preserved, and no automatic merge is performed by Build or Start. Opt-in planning preview SHALL treat unplaced nodes as free while retaining explicit labels as hard constraints; conflicting must-link/cut/manual-label combinations SHALL reject before callbacks. Ingress boundaries, replica-local internal pairing, and node-pool worker cuts SHALL remain explicit physical constraints. The current fixed/replicated/manual execution plan is not mutated by previews. Existing factory dormancy, CPU binding, typed errors, metrics, ingress, loggers, and deadline drain behavior SHALL remain available.

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** developers inspect a manually placed graph with constraints preview available
- **THEN** manual placements remain executable and the preview is visibly separate from automatic grouping execution

#### Scenario: Build without runtime resource creation
- **WHEN** a graph with automatic planning constraints is built
- **THEN** it stores copied numeric planning rows and invokes no factory, queue, worker, or affinity callback

#### Scenario: Start materializes a fixed manual plan
- **WHEN** a definition starts after a constraints preview
- **THEN** it executes the original manual or replicated fixed plan without applying preview merges

#### Scenario: Keep typed error resources dormant
- **WHEN** a graph with typed error edges builds while exposing planning constraints
- **THEN** no error pool, queue, batch, or error value is created by Build or preview

#### Scenario: Start materializes a fresh CPU-bound manual plan
- **WHEN** a caller starts the unchanged manual definition after inspecting constraints
- **THEN** it creates fresh resources on the original group and CPU plan without applying automatic merges

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a valid manual definition starts and later starts sequentially again
- **THEN** each start creates fresh state and queues on the same fixed group/CPU plan after the constraints-only preview

#### Scenario: Preserve error resource dormancy at Build
- **WHEN** a built graph has typed error routes alongside constraint preview data
- **THEN** Build still creates no error queue, pool, batch, or failure value

#### Scenario: Build without runtime error resource creation
- **WHEN** a valid graph with a configured typed error edge is built
- **THEN** Build retains dormant bindings and creates no error pool, queue, batch, factory result, worker, or error value

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
