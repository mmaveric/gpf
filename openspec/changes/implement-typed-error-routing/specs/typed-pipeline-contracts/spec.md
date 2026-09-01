## MODIFIED Requirements

### Requirement: Statically typed port connections
The graph contract SHALL expose `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)` for ordinary typed ports and `func ConnectError[E any](builder *Builder, output ErrorPort[E], input InPort[E], config EdgeConfig[E]) (EdgeID, error)` for registered typed error ports, both owned by an open `Builder`. `EdgeConfig[T]` SHALL expose `Capacity uint32`, `BatchCapacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]`. `queue.Factory[T]` SHALL have the exact signature `func(queue.Config) (queue.QueueOps[T], error)`, and `queue.Config` SHALL expose matching `Capacity uint32`, `BatchCapacity uint32`, and `Policy queue.Policy` fields. A valid connection SHALL mutate the builder only once and return a dense stable `EdgeID`; incompatible ordinary or error element types SHALL be rejected by the Go compiler. Valid port handles SHALL be created by typed node registration; zero-value and cross-builder handles SHALL be rejected at runtime without topology mutation or edge-ID consumption. Ordinary `Connect` SHALL reject only ordinary-port invalidity and SHALL not accept an error handle.

#### Scenario: Connect matching heterogeneous graph ports
- **WHEN** a program registers and connects matching int ports, matching string ports, and matching typed error ports in one open builder with nonzero queue and batch capacities
- **THEN** the program compiles and the builder records all typed edges with dense stable edge IDs

#### Scenario: Reject incompatible port types at compile time
- **WHEN** a program attempts to pass `OutPort[int]` and `InPort[string]`, or `ErrorPort[int]` and `InPort[string]`, to one corresponding connect call
- **THEN** compilation fails before graph connection validation

#### Scenario: Require an explicit error route
- **WHEN** a caller passes a registered `ErrorPort[E]` to ordinary `Connect`
- **THEN** compilation fails because ordinary Connect requires `OutPort[E]`

#### Scenario: Reject a compile-compatible error output at runtime
- **WHEN** legacy source attempts to pass the registered typed error handle to ordinary `Connect` under the prior contract
- **THEN** the source no longer compiles because registration returns `ErrorPort[E]`; callers must use `ConnectError` and no runtime unavailable-routing error is needed

### Requirement: Node and error result contract
The node contract SHALL define sealed output shapes `Outputs0` through `Outputs8` for zero through eight ordinary typed outputs, and every shape SHALL implement exact exported method `OutputCount() uint8`. `Outputs0` SHALL remain an empty output shape; `Outputs1` through `Outputs8` SHALL expose corresponding typed `Output[T]` fields so a handler can acquire or commit at most one batch per ordinary output in one invocation. It SHALL define explicit-state handler sets `Node0` through `Node8` and state-capturing closure handler sets `ClosureNode0` through `ClosureNode8`. `Node0` SHALL use a typed `SourceHandler`; every input of `Node1` through `Node8` SHALL have a separate `InputHandler` whose batch element type matches that input, and closure handler fields SHALL preserve the equivalent per-input types without an explicit state parameter. Multi-input explicit and closure handler sets SHALL respectively expose typed selectors over `ReadyMask`; `ReadyMask` SHALL represent eight input positions through zero-based `PortIndex` constants `Port1` through `Port8`. Whenever called for multiple ready inputs, a selector MUST return a `PortIndex` that is both within its handler set's declared input range and marked ready in the supplied mask; the executor SHALL invoke and enforce this contract.

Every handler SHALL return `Result[E]`. Registration SHALL expose that `E` as one distinct optional `ErrorPort[E]` beyond the eight ordinary outputs. A failed result SHALL carry its unboxed `Value` only to a matching explicit error route or be discarded when unconnected; it SHALL not be represented by a node-package output placeholder. The declarations and execution bindings SHALL NOT require reflection, runtime type assertions, or boxed data-plane values.

#### Scenario: Declare typed handler outcome
- **WHEN** a developer declares an explicit or closure handler set with error type E
- **THEN** every source or input handler returns `Result[E]` without boxing its inputs, outputs, or error value

#### Scenario: Compile all handler-set arities
- **WHEN** a developer instantiates explicit and closure handler sets at each supported input arity
- **THEN** `Node0` through `Node8` and `ClosureNode0` through `ClosureNode8` compile against any generated output shape and concrete error type

#### Scenario: Report exact ordinary output count
- **WHEN** a developer calls `OutputCount()` on any generated output shape
- **THEN** the call returns that shape's ordinary arity from zero through eight as `uint8` without counting the distinct error port

#### Scenario: Preserve heterogeneous input types
- **WHEN** a two-input node declares int as its first input and string as its second input
- **THEN** its first handler accepts `batch.Batch[int]`, its second handler accepts `batch.Batch[string]`, and both share the same output-shape and error types

#### Scenario: Reject arbitrary output shape
- **WHEN** external code attempts to instantiate a handler set with a type outside `Outputs0` through `Outputs8`
- **THEN** compilation fails because the type cannot implement the sealed OutputSet contract

#### Scenario: Preserve all ordinary outputs with error routing
- **WHEN** a handler uses `Outputs8`, returns a typed failed outcome, and its registration has a matching error connection
- **THEN** all eight ordinary output types remain available and the failed `E` is routed through the separate typed error port

#### Scenario: Address ready inputs by mask position
- **WHEN** a selector is called with multiple ready bits in a handler set's readiness mask
- **THEN** it returns a zero-based `Port1` through `Port8` index that is within that handler set's input arity and whose bit is ready

#### Scenario: Keep error outcome outside ordinary outputs
- **WHEN** a handler uses `Outputs8` and returns a typed error outcome
- **THEN** all eight ordinary output types remain available, `Result[E]` carries the error type without a node-package placeholder handle, and registration exposes E through the distinct optional error port

### Requirement: Queue and execution semantics contract
The queue contract SHALL document typed `QueueOps[T]` operations and bounded queue behavior. For a local ordinary or error edge, nil factories select a built-in typed local queue, while a custom factory supplies fresh operations for that local executor edge and is called only by its group worker. A cross-group ordinary or error edge SHALL use the framework-owned bounded typed SPSC queue; a non-nil configured factory on that edge SHALL be rejected during Build without invocation. `Drop` SHALL release a batch that cannot be pushed. `Wait` SHALL retain a full-edge output for retry on later scheduler passes while allowing downstream work to run; it SHALL not block inside a handler.

Successful Build SHALL make an opaque structurally immutable graph definition available for sequential executor reuse, with the builder's persistent non-zero owner identity; dormant `Stateful.Factory`, `ClosureFactory`, eligible local `queue.Factory`, and available lifecycle callback function values; a private deterministic fixed group plan; and one immutable explicit nonnegative unique CPU binding for each effective group. Build SHALL NOT invoke those functions or an affinity adapter, or retain factory-produced state, a `ClosureReplica`, live `QueueOps`, a queue implementation, executor state, worker, locked thread, affinity adapter, affinity outcome, lifecycle status, or error value. Start, one-locked-worker-per-fixed-group execution, an affinity attempt with mandatory locked-thread fallback, typed local queues, typed remote SPSC queues, typed error routing, handler execution, lifecycle cleanup, and read-only affinity outcomes SHALL be available only through the graph executor. Automatic grouping, CPU discovery, CPU-set policy, NUMA placement, replicas, MPSC/SPMC/MPMC queues, remote custom queues, fan-out, sharding, metrics, logging, retries, panic recovery, error mapping, error fan-out/distribution, ingress, and concurrent starts SHALL remain unavailable.

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** a developer reviews graph, queue, node, and runtime documentation after building a valid manually placed graph with CPU bindings and optional typed error edges
- **THEN** the documentation distinguishes executable fixed-group locked-thread processing, typed error routing, and observable fallback outcomes from unavailable automatic grouping, CPU discovery, NUMA placement, replicas, richer queues, broader observability, and concurrent starts

#### Scenario: Build without runtime resource creation
- **WHEN** a valid graph with observable state, closure, and eligible local queue factories plus explicit lifecycle callbacks, CPU bindings, and optional typed error edges is built
- **THEN** Build returns immutable topology and plan with every callback and affinity-adapter invocation count still zero and stores no factory-produced state, live queue operations, error pool, worker, thread, or affinity outcome

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a valid definition starts its executor and later starts sequentially again
- **THEN** its factories create fresh runtime-local state, eligible local queue operations, and error pools, while each start creates exactly one locked worker and one recorded affinity outcome per the same immutable CPU-bound group plan

#### Scenario: Build without runtime error resource creation
- **WHEN** a valid graph with a configured typed error edge is built
- **THEN** Build retains only dormant typed bindings and creates no error pool, queue, batch, factory result, worker, or error value
