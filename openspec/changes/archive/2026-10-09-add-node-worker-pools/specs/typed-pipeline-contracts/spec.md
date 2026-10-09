## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
An explicitly configured node worker pool SHALL expand one logical node into a fixed positive number of physical workers, each with a distinct explicit CPU and independent factory-created state. Surrounding logical nodes remain in their existing groups. Pool worker inputs SHALL share bounded typed queues with one consumer per worker; outputs SHALL use the selected physical-cardinality queue and distinct producer-local pools. The framework SHALL not broadcast accepted batches. Heterogeneous inputs SHALL preserve typed per-port handlers and worker-local ReadyMask selectors. Build SHALL validate queue models, CPU uniqueness/completeness, and reject overlap with whole-group replicas or ingress adapters without invoking callbacks. Start/Drain/Close SHALL include every physical worker and lifecycle. Existing zero-pool/default graphs retain their current behavior; no dynamic resizing, global order, or fairness guarantee is added.

#### Scenario: Review a pooled node boundary
- **WHEN** a logical node with one ordinary input and one output is assigned three fixed workers
- **THEN** copied queue plans report one shared multi-consumer input and the correctly selected multi-producer output transport while adjacent nodes retain their groups

#### Scenario: Build without runtime worker creation
- **WHEN** a valid node pool with complete CPU assignments is built
- **THEN** Build stores immutable worker metadata and dormant factories without constructing state, queue operations, or workers

#### Scenario: Start fresh pooled instances
- **WHEN** a pooled definition starts after a prior executor stops
- **THEN** every physical worker receives fresh state and affinity outcome while logical topology remains unchanged

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews the manual executor and optional fixed node pools
- **THEN** docs distinguish fixed placement/typed queue models from unavailable automatic grouping, NUMA policy, and dynamic resizing

#### Scenario: Build without runtime resource creation
- **WHEN** a valid pool with observable factories and complete CPU assignments is built
- **THEN** Build does not invoke callbacks or create pools, queues, workers, or affinity outcomes

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a pooled definition starts sequentially
- **THEN** each worker receives fresh state, source pools, typed queues, and a locked CPU-bound worker

#### Scenario: Build without runtime error resource creation
- **WHEN** a pooled graph has a configured typed error edge
- **THEN** Build retains dormant bindings and creates no error pool, queue, batch, or error value

#### Scenario: Distribute ingress across replicated consumers
- **WHEN** one unreplicated typed ingress feeds three replicas through a shared SPMC boundary
- **THEN** accepted batches are consumed once across the three replicas with independent input/output pool ownership

#### Scenario: Preserve one source pool per producer
- **WHEN** three replica workers publish through a shared MPSC boundary
- **THEN** each worker acquires from its own typed pool and downstream release returns each slot to its producing replica's pool

#### Scenario: Resolve an expanded physical edge
- **WHEN** the pure resolver receives nonzero producer/consumer worker counts and a physical ownership classification
- **THEN** it selects Local, SPSC, MPSC, SPMC, or MPMC according to the cardinality matrix and returns an error for zero or inconsistent local cardinality
