## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
The queue contract SHALL provide typed bounded Local/SPSC/MPSC/SPMC/MPMC operations selected from physical worker cardinality. Local custom factories are only valid when all queue operations share one worker owner; incompatible remote custom factories reject before callback invocation. Ordinary Connect SHALL retain one producer per input and one consumer per output. Explicit homogeneous ConnectFanIn SHALL atomically connect two or more distinct compatible ordinary output ports to one otherwise-unconnected ordinary input; it SHALL be the only multi-producer input path. Its producer ports and input must belong to the same builder and have identical T. It SHALL reject foreign, duplicate, already connected, or cycle-creating endpoints without partial mutation. FanIn SHALL materialize one shared bounded queue, one origin-owned pool per producer, branch-specific metrics, and one shared drain/close registration. Explicit ConnectBroadcast SHALL copy each ordinary batch into distinct shallow branch-owned storage; ConnectShard SHALL route each element to one branch using bounded staging. Those explicit fan-out operations retain independent branch ownership and remain incompatible with replica and node-worker-pool expansion in this version. Physical queue selection SHALL count distinct producer/consumer worker owners, not endpoint count, and accepted work SHALL be delivered once with no ordering or fairness guarantee across producers. Drop and Wait remain per producer output or explicit branch.

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews the manual runtime with typed edges
- **THEN** docs distinguish fixed placement and explicit FanIn from unavailable broadcast, automatic grouping, NUMA placement, and concurrent starts

#### Scenario: Build without runtime resource creation
- **WHEN** Build validates a definition containing FanIn edges
- **THEN** it retains dormant typed bindings and numeric queue plans without creating the shared queue or any producer pool

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a valid definition with FanIn starts sequentially
- **THEN** it materializes one shared queue with distinct producer pools and fresh workers from its retained plan

#### Scenario: Build without runtime error resource creation
- **WHEN** a graph contains both FanIn and typed error edges
- **THEN** Build still creates no error pools, queue operations, batches, or error values

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
