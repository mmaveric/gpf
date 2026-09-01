## MODIFIED Requirements

### Requirement: Queue and execution semantics contract
The queue contract SHALL document typed `QueueOps[T]` operations and bounded queue behavior. For a local edge, nil factories select a built-in typed local queue, while a custom factory supplies fresh operations for that local executor edge and is called only by its group worker. A cross-group edge SHALL use the framework-owned bounded typed SPSC queue; a non-nil configured factory on that edge SHALL be rejected during Build without invocation. `Drop` SHALL release a batch that cannot be pushed. `Wait` SHALL retain a full-edge output for retry on later scheduler passes while allowing downstream work to run; it SHALL not block inside a handler.

Successful Build SHALL make an opaque structurally immutable graph definition available for sequential executor reuse, with the builder's persistent non-zero owner identity; dormant `Stateful.Factory`, `ClosureFactory`, eligible local `queue.Factory`, and available lifecycle callback function values; and a private deterministic fixed group plan. Build SHALL NOT invoke those functions or retain factory-produced state, a `ClosureReplica`, live `QueueOps`, a queue implementation, executor state, worker, or lifecycle status. Start, one-worker-per-fixed-group execution, typed local queues, typed remote SPSC queues, handler execution, and lifecycle cleanup SHALL be available only through the graph executor. Automatic grouping, CPU placement, affinity, `LockOSThread`, replicas, MPSC/SPMC/MPMC queues, remote custom queues, fan-out, sharding, metrics, logging, error routing, concurrent starts, and custom-queue resource teardown SHALL remain unavailable.

#### Scenario: Review bounded manual multi-group behavior
- **WHEN** a developer reviews graph, queue, node, and runtime documentation after building a valid manually placed graph
- **THEN** the documentation distinguishes executable fixed-group local and remote-SPSC processing from unavailable automatic grouping, affinity, replicas, richer queues, observability, error routing, and concurrent starts

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph with observable state, closure, and eligible local queue factories plus explicit lifecycle callbacks is built
- **THEN** Build returns immutable topology and plan with every callback invocation count still zero and stores no factory-produced state, live queue operations, or worker

#### Scenario: Start materializes fresh fixed-plan resources
- **WHEN** a valid definition starts its executor and later starts sequentially again
- **THEN** its factories create fresh runtime-local state and eligible local queue operations, while each start creates exactly one worker per the same immutable group plan
