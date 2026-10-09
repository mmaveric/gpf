## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
The queue contract SHALL document typed `QueueOps[T]` operations and bounded queue behavior. For a local ordinary or error edge whose operations share one worker owner, nil factories select a built-in typed local queue, while a custom factory supplies fresh operations for that local executor edge and is called only by its group worker. For each physical edge, the immutable definition SHALL expose an independent edge-ordered `EdgeQueuePlans()` copy containing EdgeID, selected QueueModel, producer-worker cardinality, consumer-worker cardinality, and remote classification. Model selection SHALL use finalized physical endpoint-worker counts: one producer/one consumer with one shared worker owner selects Local; cross-worker 1:1 selects SPSC; N:1 selects MPSC; 1:N selects SPMC; and N:M selects MPMC. Zero cardinalities and a local model with multiple physical workers SHALL be rejected by the resolver. A non-nil custom local queue factory SHALL be rejected before invocation for any non-Local selected model. Build SHALL calculate and retain queue plans without constructing queues or invoking factories; Start SHALL materialize fresh typed storage for the retained model. The current fixed one-worker-per-group topology SHALL continue to produce only Local or SPSC plans. `Drop` SHALL release a batch that cannot be pushed. `Wait` SHALL retain a full-edge output for retry on later scheduler passes while allowing downstream work to run; it SHALL not block inside a handler.

Successful Build SHALL make an opaque structurally immutable graph definition available for sequential executor reuse, with the builder's persistent non-zero owner identity; dormant `Stateful.Factory`, `ClosureFactory`, eligible local `queue.Factory`, and available lifecycle callback function values; a private deterministic fixed group plan; and one immutable explicit nonnegative unique CPU binding for each effective group. Build SHALL NOT invoke those functions or an affinity adapter, or retain factory-produced state, a `ClosureReplica`, live `QueueOps`, a queue implementation, executor state, worker, locked thread, affinity adapter, affinity outcome, lifecycle status, error value, or mutable metrics. Start, one-locked-worker-per-fixed-group execution, an affinity attempt with mandatory locked-thread fallback, typed local queues, typed remote SPSC queues, standalone typed MPSC/SPMC/MPMC queue constructors, read-only edge queue plans, typed error routing, handler execution, lifecycle cleanup, read-only affinity outcomes, and runtime-metrics numeric counters and snapshots SHALL be available. Automatic grouping, CPU discovery, CPU-set policy, NUMA placement, replicas, automatic topology expansion, remote custom queues, fan-out, sharding, timing, custom user metrics, exporters, logging, handler retries, panic recovery, error mapping, error fan-out/distribution, ingress, and concurrent starts SHALL remain unavailable.

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews graph, queue, node, and runtime documentation after building a valid manually placed graph with CPU bindings and optional typed error edges
- **THEN** the documentation distinguishes executable fixed-group locked-thread processing, fixed-topology Local/SPSC edge plans, standalone concurrent queue constructors, typed error routing, numeric runtime snapshots, and observable fallback outcomes from unavailable automatic grouping, CPU discovery, NUMA placement, replicas, richer graph topology, broader observability, and concurrent starts

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph with observable state, closure, and eligible local queue factories plus explicit lifecycle callbacks, CPU bindings, and optional typed error edges is built
- **THEN** Build returns immutable topology, fixed group plan, and ordered edge queue models/cardinalities with every callback and affinity-adapter invocation count still zero and stores no factory-produced state, live queue operations, queue implementation, error pool, mutable metrics, worker, thread, or affinity outcome

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a valid definition starts and later starts sequentially again
- **THEN** its factories create fresh runtime-local state, eligible local queue operations, error pools, and metrics, while each start creates exactly one locked worker per planned group and materializes fresh typed queues matching each immutable edge model plan

#### Scenario: Build without runtime error resource creation
- **WHEN** a valid graph with a configured typed error edge is built
- **THEN** Build retains only dormant typed bindings and creates no error pool, queue, batch, factory result, worker, or error value

#### Scenario: Resolve an expanded physical edge
- **WHEN** the pure resolver receives nonzero producer/consumer worker counts and a physical ownership classification
- **THEN** it selects Local, SPSC, MPSC, SPMC, or MPMC according to the cardinality matrix and returns an error for zero or inconsistent local cardinality
