## Purpose
Define the public typed pipeline contracts, including executable batch-pool ownership and lifecycle semantics.

## Requirements

### Requirement: Go module and public contract packages
The repository SHALL define a Go module requiring Go 1.22 or newer and SHALL provide compileable public contract packages for executable typed batch handles and pools, typed graph ports and connections, node result types, typed queue operations, and runtime configuration. The declarations and batch steady-state operations SHALL not use reflection, runtime type assertions, or `interface{}` to represent data-plane values.

#### Scenario: Compile public contracts
- **WHEN** the module is tested with Go 1.22 or newer
- **THEN** all public packages compile and the batch package works without requiring an executable graph runtime

### Requirement: Statically typed port connections
The graph contract SHALL expose `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)` between ordinary typed ports owned by an open `Builder`. `EdgeConfig[T]` SHALL expose `Capacity uint32`, `BatchCapacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]`. `queue.Factory[T]` SHALL have the exact signature `func(queue.Config) (queue.QueueOps[T], error)`, and `queue.Config` SHALL expose matching `Capacity uint32`, `BatchCapacity uint32`, and `Policy queue.Policy` fields. Connect SHALL mutate the builder only for a valid connection and return a dense stable `EdgeID`. A program connecting matching port element types SHALL compile without a runtime type registry, and an incompatible connection SHALL be rejected by the Go compiler. Valid port handles SHALL be created by typed node registration; zero-value and cross-builder handles SHALL be rejected at runtime without topology mutation or edge-ID consumption. Registered typed error outputs SHALL remain reserved but Connect SHALL reject them with `ErrErrorRoutingUnavailable` until error routing is implemented.

#### Scenario: Connect matching heterogeneous graph ports
- **WHEN** a program registers and connects matching int ports and matching string ports in one open builder with nonzero queue and batch capacities
- **THEN** the program compiles and the builder records both typed edges with dense stable edge IDs

#### Scenario: Reject incompatible port types at compile time
- **WHEN** a program attempts to pass `OutPort[int]` and `InPort[string]` to one `Connect[T]` call
- **THEN** compilation fails before graph connection validation

#### Scenario: Reject a compile-compatible error output at runtime
- **WHEN** a registered typed error `OutPort[E]` and ordinary `InPort[E]` satisfy the generic `Connect[E]` signature
- **THEN** `Connect` returns `ErrErrorRoutingUnavailable` and records no edge

### Requirement: Batch ownership contract
The batch contract SHALL provide explicit allocation-free ownership transfer and release for batches acquired from a typed pool. A caller that acquires a batch SHALL own it until it moves the batch or releases it; `Move` SHALL clear the source handle, and `Pool.Release` and `Batch.Release` SHALL clear the caller's handle. `Batch.Release` SHALL return an owned non-empty batch to the pool that allocated its slot, including after the batch has moved through compatible typed edges. The contract SHALL document that handlers MAY move an input only to a compatible output and that an un-moved input is returned to its originating source pool by the executor. Runtime ownership checks SHALL NOT be required.

#### Scenario: Move an acquired batch
- **WHEN** a caller moves an acquired batch to another handle
- **THEN** the destination retains the slot and the source is an empty handle that cannot release the slot again through correct API use

#### Scenario: Release an origin-owned moved batch
- **WHEN** an executor releases an un-moved batch after it has crossed one or more compatible edges
- **THEN** Batch.Release returns it to its original allocating pool without type erasure or a runtime pool lookup

### Requirement: Preallocated typed pool
`NewPool[T]` SHALL validate a nonzero pool capacity and batch capacity, reject invalid size arithmetic, and allocate all slot metadata and typed batch storage before returning a pool. A pool SHALL NOT resize or allocate additional batch storage after construction.

#### Scenario: Construct a valid pool
- **WHEN** a caller constructs a pool with positive slot and batch capacities
- **THEN** the pool contains exactly the configured number of fixed-capacity typed batch slots

#### Scenario: Reject invalid pool configuration
- **WHEN** a caller constructs a pool with zero capacity, zero batch capacity, or overflowing storage size
- **THEN** construction returns an error without creating a usable pool

### Requirement: Controlled batch slice access
An acquired `Batch[T]` SHALL expose `Mutable()` as its full fixed-capacity writable storage and SHALL expose `Items()` limited to its explicit logical length. `SetLen(n)` SHALL accept only a length from zero through `Cap()` and SHALL reject an out-of-range length without allocating. Callers SHALL fill `Mutable()` by index and SHALL NOT rely on capacity-growing append behavior.

#### Scenario: Fill and publish logical content
- **WHEN** a caller writes elements into `Mutable()` and sets a valid logical length
- **THEN** `Items()` exposes exactly those logical elements and `Cap()` remains the configured batch capacity

#### Scenario: Reject oversized logical content
- **WHEN** a caller sets a logical length greater than the batch capacity
- **THEN** the operation reports failure and the batch remains within its fixed capacity

### Requirement: Allocation-free pool lifecycle
`Acquire` SHALL return an owned typed batch and availability indicator without allocating after pool construction. If every slot is acquired, `Acquire` SHALL report exhaustion with no batch and SHALL NOT allocate a replacement. `Release` of an owned batch SHALL return its slot for reuse without allocating. Stage 1 pool operations SHALL be used by one owner goroutine and SHALL NOT promise concurrent safety.

#### Scenario: Reuse a released batch
- **WHEN** a caller acquires, fills, and releases a batch from a non-exhausted pool
- **THEN** a later acquire can reuse a pool slot without allocation

#### Scenario: Exhaust a pool
- **WHEN** a caller acquires every configured slot and requests one more batch
- **THEN** acquire reports exhaustion without allocation or capacity growth

### Requirement: Configurable release clearing
When `ClearOnRelease` is enabled, release SHALL zero the complete fixed-capacity storage of the released batch before making its slot reusable. When disabled, release SHALL retain element values until overwritten and SHALL document the pointer-retention risk.

#### Scenario: Clear a previously populated slot
- **WHEN** a caller releases a batch with clearing enabled and later reacquires its slot
- **THEN** every element of the slot's full batch capacity is the zero value

#### Scenario: Retain storage when clearing is disabled
- **WHEN** a caller releases a batch with clearing disabled
- **THEN** the pool does not perform clearing work and its documentation identifies possible retained references

### Requirement: Node and error result contract
The node contract SHALL define sealed output shapes `Outputs0` through `Outputs8` for zero through eight ordinary typed outputs, and every shape SHALL implement exact exported method `OutputCount() uint8`. `Outputs0` SHALL remain an empty output shape; `Outputs1` through `Outputs8` SHALL expose corresponding typed `Output[T]` fields so a handler can acquire or commit at most one batch per ordinary output in one invocation. It SHALL define explicit-state handler sets `Node0` through `Node8` and state-capturing closure handler sets `ClosureNode0` through `ClosureNode8`. `Node0` SHALL use a typed `SourceHandler`; every input of `Node1` through `Node8` SHALL have a separate `InputHandler` whose batch element type matches that input, and closure handler fields SHALL preserve the equivalent per-input types without an explicit state parameter. Multi-input explicit and closure handler sets SHALL respectively expose typed selectors over `ReadyMask`; `ReadyMask` SHALL represent eight input positions through zero-based `PortIndex` constants `Port1` through `Port8`. Whenever called for multiple ready inputs, a selector MUST return a `PortIndex` that is both within its handler set's declared input range and marked ready in the supplied mask; the single-group executor SHALL invoke and enforce this contract. Every handler SHALL return `Result[E]`. The node-package placeholder error handle SHALL remain absent, while the main contract SHALL reserve `E` for a later graph registration error output beyond the eight ordinary outputs. The declarations and execution bindings SHALL NOT require reflection, runtime type assertions, or boxed data-plane values.

#### Scenario: Declare typed handler outcome
- **WHEN** a developer declares an explicit or closure handler set with error type E
- **THEN** every source or input handler returns `Result[E]` without boxing its inputs, outputs, or error value

#### Scenario: Compile all handler-set arities
- **WHEN** a developer instantiates explicit and closure handler sets at each supported input arity
- **THEN** `Node0` through `Node8` and `ClosureNode0` through `ClosureNode8` compile against any generated output shape

#### Scenario: Report exact ordinary output count
- **WHEN** a developer calls `OutputCount()` on any generated output shape
- **THEN** the call returns that shape's arity from zero through eight as `uint8`

#### Scenario: Preserve heterogeneous input types
- **WHEN** a two-input node declares int as its first input and string as its second input
- **THEN** its first handler accepts `batch.Batch[int]`, its second handler accepts `batch.Batch[string]`, and both share the same output-shape and error types

#### Scenario: Reject arbitrary output shape
- **WHEN** external code attempts to instantiate a handler set with a type outside `Outputs0` through `Outputs8`
- **THEN** compilation fails because the type cannot implement the sealed OutputSet contract

#### Scenario: Address ready inputs by mask position
- **WHEN** a selector is called with multiple ready bits in a handler set's readiness mask
- **THEN** it returns a zero-based `Port1` through `Port8` index that is within that handler set's input arity and whose bit is ready

#### Scenario: Keep error outcome outside ordinary outputs
- **WHEN** a handler uses `Outputs8` and returns a typed error outcome
- **THEN** all eight ordinary output types remain available, `Result[E]` carries the error type without a node-package placeholder handle, and E remains reserved for later error routing

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
