## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
The queue contract SHALL document typed `QueueOps[T]` operations and bounded queue behavior. For a local edge, nil factories select a built-in typed local queue, while a custom factory supplies fresh operations for that local executor edge and is called only by its group worker. A cross-group edge SHALL use the framework-owned bounded typed SPSC queue; a non-nil configured factory on that edge SHALL be rejected during Build without invocation. `Drop` SHALL release a batch that cannot be pushed. `Wait` SHALL retain a full-edge output for retry on later scheduler passes while allowing downstream work to run; it SHALL not block inside a handler.

Successful Build SHALL make an opaque structurally immutable graph definition available for sequential executor reuse, with the builder's persistent non-zero owner identity; dormant `Stateful.Factory`, `ClosureFactory`, eligible local `queue.Factory`, and available lifecycle callback function values; a private deterministic fixed group plan; and one immutable explicit nonnegative unique CPU binding for each effective group. Build SHALL NOT invoke those functions or an affinity adapter, or retain factory-produced state, a `ClosureReplica`, live `QueueOps`, a queue implementation, executor state, worker, locked thread, affinity adapter, affinity outcome, or lifecycle status. Start, one-locked-worker-per-fixed-group execution, an affinity attempt with mandatory locked-thread fallback, typed local queues, typed remote SPSC queues, handler execution, lifecycle cleanup, and read-only affinity outcomes SHALL be available only through the graph executor. Automatic grouping, CPU discovery, CPU-set policy, NUMA placement, replicas, MPSC/SPMC/MPMC queues, remote custom queues, fan-out, sharding, metrics, logging, error routing, concurrent starts, and custom-queue resource teardown SHALL remain unavailable.

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews graph, queue, node, and runtime documentation after building a valid manually placed graph with CPU bindings
- **THEN** the documentation distinguishes executable fixed-group locked-thread processing and observable fallback outcomes from unavailable automatic grouping, CPU discovery, NUMA placement, replicas, richer queues, observability, error routing, and concurrent starts

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph with observable state, closure, and eligible local queue factories plus explicit lifecycle callbacks and CPU bindings is built
- **THEN** Build returns immutable topology and plan with every callback and affinity-adapter invocation count still zero and stores no factory-produced state, live queue operations, worker, thread, or affinity outcome

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a valid definition starts its executor and later starts sequentially again
- **THEN** its factories create fresh runtime-local state and eligible local queue operations, while each start creates exactly one locked worker and one recorded affinity outcome per the same immutable CPU-bound group plan
