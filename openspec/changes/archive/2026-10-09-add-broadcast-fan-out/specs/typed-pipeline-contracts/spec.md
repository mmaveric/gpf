## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
Ordinary `Connect` SHALL retain one consumer per output and one producer per input. Explicit `ConnectBroadcast` SHALL atomically connect one compatible ordinary output to a fixed list of at least two distinct ordinary inputs with equal configured batch capacities. It SHALL allocate distinct branch-owned typed slots and shallow-copy elements; it SHALL NOT duplicate pointer referents. Branch queue/pool outcomes SHALL be independent and bounded: exhaustion affects only one branch, Drop releases only that branch, and Wait retains only the full branch copy. A completed branch SHALL never be recopied or redelivered. Every Branch's accepted batch remains exclusive and is reclaimed once on Drain/Close. ErrorPort cardinality and ConnectError semantics remain unchanged. Broadcast SHALL remain incompatible with whole-group replica or node-worker-pool expansion in this version; dynamic resizing, ordering, and deep copy remain unavailable.

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** docs are reviewed for an ordinary fixed-group graph with Broadcast available
- **THEN** manual placement, queue ownership, broadcast copying, and unavailable dynamic/NUMA features are distinguished

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph contains a Broadcast edge and Build succeeds
- **THEN** Build retains dormant branch plans without creating live branch pools or queues

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a definition with Broadcast starts sequentially
- **THEN** it creates fresh distinct branch pools/queues on its immutable CPU plan

#### Scenario: Build without runtime error resource creation
- **WHEN** Broadcast and typed error ports coexist during Build
- **THEN** no error pool, queue, batch, or error value is created

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
