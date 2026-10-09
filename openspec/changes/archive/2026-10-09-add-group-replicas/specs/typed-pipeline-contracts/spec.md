## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
For a fixed whole-group replica count N, each logical node SHALL materialize N physical instances with independent state. Each internal edge in the replicated group SHALL pair corresponding replica endpoints, so each internal queue is Local when paired on one worker. A boundary edge SHALL use one bounded typed queue shared by all physical producers or consumers, selecting SPSC/MPSC/SPMC/MPMC from its physical worker cardinality; every accepted boundary batch SHALL be delivered to one consumer instance, never implicitly broadcast. Every physical producer SHALL own a distinct preallocated source pool even when transport is shared. Queue operations and snapshots SHALL preserve logical edge identity plus replica/endpoint identity. Each physical group instance SHALL have one explicit unique CPU and one locked worker. Build SHALL validate and freeze the expansion without factory invocation; Start SHALL materialize fresh per-instance state and queue/pool resources. Remote/custom factories remain unavailable. The public logical node/edge handles and NodeCount/EdgeCount SHALL remain logical, and explicit Broadcast remains a separate capability.

#### Scenario: Distribute ingress across replicated consumers
- **WHEN** one unreplicated typed ingress feeds three replicas through a shared SPMC boundary
- **THEN** accepted batches are consumed once across the three replicas with independent input/output pool ownership

#### Scenario: Preserve one source pool per producer
- **WHEN** three replica workers publish through a shared MPSC boundary
- **THEN** each worker acquires from its own typed pool and downstream release returns each slot to its producing replica's pool

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews graph, queue, node, and runtime documentation after building a valid manually placed graph
- **THEN** the documentation distinguishes fixed execution, replicas, typed errors, metrics, and affinity outcomes from unavailable dynamic scaling and concurrent starts

#### Scenario: Build without runtime resource creation
- **WHEN** a valid replicated graph with observable factories and complete CPU assignments is built
- **THEN** Build returns a copied immutable logical and physical plan without invoking callbacks or creating live pools, queues, workers, or affinity results

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a replicated definition starts sequentially after a prior stop
- **THEN** each start creates fresh state, pools, queues, physical workers, and affinity results without mutating the retained plan

#### Scenario: Build without runtime error resource creation
- **WHEN** a replicated graph contains typed error routes
- **THEN** Build retains dormant typed bindings but creates no error pools, queue operations, batches, or error values

#### Scenario: Resolve an expanded physical edge
- **WHEN** the pure resolver receives nonzero producer/consumer worker counts and a physical ownership classification
- **THEN** it selects Local, SPSC, MPSC, SPMC, or MPMC according to the cardinality matrix and returns an error for zero or inconsistent local cardinality
