## Purpose
Define the public typed pipeline contracts, including executable batch-pool ownership and lifecycle semantics.

## Requirements

### Requirement: Go module and public contract packages
The repository SHALL define a Go module requiring Go 1.22 or newer and SHALL provide compileable public contract packages for executable typed batch handles and pools, typed graph ports and connections, node result types, typed queue operations, and runtime configuration. The declarations and batch steady-state operations SHALL not use reflection, runtime type assertions, or `interface{}` to represent data-plane values.

#### Scenario: Compile public contracts
- **WHEN** the module is tested with Go 1.22 or newer
- **THEN** all public packages compile and the batch package works without requiring an executable graph runtime

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

### Requirement: Batch ownership contract
The batch contract SHALL provide explicit allocation-free ownership transfer and release for batches acquired from a typed pool. A caller that acquires a batch SHALL own it until it moves the batch or releases it; `Move` SHALL clear the source handle, and `Pool.Release` and `Batch.Release` SHALL clear the caller's handle. `Batch.Release` SHALL return an owned non-empty batch to the pool that allocated its slot, including after the batch has moved through compatible typed edges. The contract SHALL document that handlers MAY move an input only to a compatible output and that an un-moved, un-released input is returned to its originating source pool by the executor. Moving or explicitly releasing the handler's by-value copy SHALL relinquish automatic input cleanup, so a retained runner copy SHALL not release or reset a transferred slot. Invocation-local transfer bookkeeping SHALL remain allocation-free and SHALL not travel with independently moved batches. General runtime checks for arbitrary handle misuse SHALL NOT be required.

#### Scenario: Move an acquired batch
- **WHEN** a caller moves an acquired batch to another handle
- **THEN** the destination retains the slot and the source is an empty handle that cannot release the slot again through correct API use

#### Scenario: Release an origin-owned moved batch
- **WHEN** an executor releases an un-moved batch after it has crossed one or more compatible edges
- **THEN** Batch.Release returns it to its original allocating pool without type erasure or a runtime pool lookup

#### Scenario: Preserve a move from a by-value handler input
- **WHEN** a handler moves its copied input handle into a compatible committed output
- **THEN** the runner's retained cleanup handle does not release the transferred slot, and downstream retains exclusive ownership of its original length and items

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
An automatic grouping preview SHALL remain non-executable until `FinalizeAutomaticPlan` consumes that current preview and exactly one unique nonnegative CPU binding is supplied for every proposed physical group. `BindAutomaticPlanCPU` SHALL validate owner/revision/group identity, reject duplicate CPU and group assignments, preserve already hard-bound CPU values, and not change the manual plan. Incomplete, duplicate, negative, foreign, or stale assignments SHALL reject before callbacks or sealing. Successful finalization SHALL freeze physical nodes/groups/edges, selected typed queue models, replica and worker identities, and CPU bindings in an immutable definition. Ordinary Builder.Build and manual Start semantics SHALL remain unchanged unless a CPU-complete plan is explicitly finalized. Every physical worker then follows the existing locked-thread and portable affinity-fallback contract; queue ownership/Wait/Drop semantics remain unchanged.

#### Scenario: Keep manual Build as the default
- **WHEN** automatic preview is enabled but no explicit finalization is requested
- **THEN** Builder.Build continues to use the existing manual topology and placements rather than applying preview merges

#### Scenario: Reject incomplete or invalid automatic CPU assignment
- **WHEN** finalization is requested with missing, negative, duplicate, foreign, or stale group CPU bindings
- **THEN** it rejects before sealing or invoking factories

#### Scenario: Start an immutable CPU-complete plan
- **WHEN** a current automatic preview receives one valid unique CPU per physical group and is finalized
- **THEN** Build returns the finalized definition and Start creates one fresh locked worker per physical group with selected queue plans

#### Scenario: Reuse automatic plan sequentially
- **WHEN** a finalized automatic executor stops and the same definition starts again
- **THEN** runtime state and affinity results are fresh while logical topology and physical CPU/queue plans remain unchanged

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** documentation is reviewed after automatic CPU finalization is implemented
- **THEN** fixed manual execution remains the default and automatic execution requires explicit preview, assignment, and finalization

#### Scenario: Build without runtime resource creation
- **WHEN** a graph or preview is built before automatic finalization
- **THEN** factories and runtime queue/worker resources remain dormant

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a finalized plan starts sequentially
- **THEN** every physical group gets fresh resources on its bound CPU

#### Scenario: Build without runtime error resource creation
- **WHEN** a typed-error graph is previewed or built before finalization
- **THEN** no error pool, batch, queue, or error value is created

#### Scenario: Review CPU-bound manual multi-group behavior
- **WHEN** documentation is reviewed after automatic plan finalization support is added
- **THEN** the current fixed manual executor and explicit automatic-finalization stages are distinguished

#### Scenario: Build without runtime resource creation
- **WHEN** a graph is built before automatic CPU finalization
- **THEN** only immutable topology and dormant numeric plans exist; factories and workers remain uncreated

#### Scenario: Start materializes fresh CPU-bound fixed-plan resources
- **WHEN** a finalized automatic plan starts sequentially
- **THEN** fresh state, queues, workers, and affinity outcomes match its retained physical plan

#### Scenario: Build without runtime error resource creation
- **WHEN** a typed-error graph is built before CPU finalization
- **THEN** no error pool, batch, queue, or error value is created

#### Scenario: Review manual and automatic plan side by side
- **WHEN** an opt-in preview is requested and then the definition is built in current manual mode
- **THEN** preview rows are independent and exploratory while executable placements remain the existing manual fixed plan

#### Scenario: Preserve physical hard constraints
- **WHEN** a preview contains replicas, node pools, explicit placements, or ingress boundaries
- **THEN** resulting groups never merge across those mandatory constraints and required CPU slots remain visible

#### Scenario: Reject stale preview use
- **WHEN** registration, connection, or placement changes after a preview
- **THEN** the preview's freshness check reports stale and it cannot be treated as executable plan data

#### Scenario: Start materializes a fixed manual plan
- **WHEN** a definition starts after a constraints preview
- **THEN** it executes the original manual or replicated fixed plan without applying preview merges

#### Scenario: Keep typed error resources dormant
- **WHEN** a graph with typed error edges builds while exposing planning constraints
- **THEN** no error pool, queue, batch, or error value is created by Build or preview

#### Scenario: Start materializes a fresh CPU-bound manual plan
- **WHEN** a caller starts the unchanged manual definition after inspecting constraints
- **THEN** it creates fresh resources on the original group and CPU plan without applying automatic merges

#### Scenario: Preserve error resource dormancy at Build
- **WHEN** a built graph has typed error routes alongside constraint preview data
- **THEN** Build still creates no error queue, pool, batch, or failure value

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

### Requirement: Typed external ingress source
AddIngress[T] SHALL register a dormant typed source with one ordinary output, a NodeID, and an IngressDefinition[T] handle. BindIngress SHALL validate provenance and return a fresh executor-local single-producer endpoint after Start. Invalid, foreign, or stopped bindings SHALL reject. Producer operations SHALL be serialized by one external caller; shutdown admission closure SHALL be allowed concurrently. The adapter SHALL process only accepted input work during drain and SHALL not count framework adapter polling as user handler calls.

#### Scenario: Connect external ingress
- **WHEN** a typed ingress output is connected to a matching input and the definition starts
- **THEN** its bound producer can submit owned or copied batches without boxing T

### Requirement: External ownership and status outcomes
Ingress Acquire SHALL return owned preallocated source storage. Commit SHALL return explicit accepted/full/closed/drop/invalid/oversize statuses, clearing the owned ingress handle only on acceptance or configured Drop. Submit SHALL reject oversize input atomically, copy one bounded batch, and release its private acquisition on nonaccepted outcomes. Uncommitted caller-owned batches SHALL remain releasable after stop. Ingress source pool rows SHALL use their registered source NodeID with EdgeID zero; ordinary edge pools SHALL keep their edge-origin identity.

#### Scenario: Release caller-owned work after stop
- **WHEN** a caller retains an uncommitted ingress acquisition through completed shutdown
- **THEN** Commit rejects without consuming it and Release returns it safely to its retained source pool

### Requirement: Dormant typed logging mapper declarations
Generated explicit-state and closure handler families SHALL expose optional typed `MapError ErrorMapper[E]` configuration matching the handler's concrete failure type. Typed mapper declarations and explicit conversion SHALL be available independently of asynchronous logging. Registration and Build SHALL not invoke the mapper, and Start without a logging destination SHALL leave runtime conversion dormant. Existing deferral of runtime logging/error-mapping execution SHALL not prohibit these typed declarations or explicit conversion helpers. Configured callbacks SHALL use immutable captures and SHALL be documented as nonblocking/nonallocating when invoked by a worker.

#### Scenario: Configure without logging activation
- **WHEN** a caller supplies a mapper matching a node's concrete E and starts the definition without a logger
- **THEN** failure routing remains typed and the mapper callback remains uncalled

#### Scenario: Reject a mismatched mapper declaration
- **WHEN** a caller assigns an ErrorMapper[string] to an int-failure node's MapError field
- **THEN** Go compilation rejects the mismatched function type before graph registration

### Requirement: Explicit asynchronous logger configuration
The graph contract SHALL expose `LoggerConfig` with per-worker `Capacity uint32` and `Handle func(node.ErrorEvent)`, and `SetLogger(*Builder, LoggerConfig) error`. An open initialized builder SHALL accept at most one valid logger configuration. Zero/unrepresentable capacity and a nil handler SHALL reject without storing configuration; invalid/sealed builders and duplicate valid configuration SHALL report their corresponding categories. Configuration SHALL be copied, with user callback captures remaining immutable caller-owned configuration. This explicit capability enables bounded asynchronous logging for configured definitions; definitions without configuration SHALL retain the deferred/dormant logging behavior. Worker publication SHALL require no boxed E or formatted event payload.

#### Scenario: Reject an invalid logger
- **WHEN** SetLogger receives a zero-capacity configuration or nil callback
- **THEN** it reports an invalid logger and the builder remains available for a corrected configuration

### Requirement: Ordered numeric logger outcomes
Executor Metrics SHALL include independently copied per-worker logger rows with group identity and attempted, accepted, full-ring dropped, delivered, and shutdown-discarded event counts. Disabled logging SHALL return no logger rows. After completed stop, accepted events SHALL equal delivered plus shutdown-discarded events; attempted events SHALL equal accepted plus full-ring dropped events. Nil MapError configuration SHALL produce a zero numeric payload with runtime-owned identity when logging is installed.

#### Scenario: Inspect a stopped saturated logger
- **WHEN** a logger stops after ring-full losses and forced discard
- **THEN** final counters distinguish producer drops from accepted-event delivery/discard, and modifying returned rows does not alter future snapshots

### Requirement: Opt-in processing timing configuration
The graph contract SHALL expose TimingConfig with Mode, Every, and optional Clock func() uint64 fields and SetTiming(*Builder, TimingConfig) error. The default SHALL disable timing without clock reads. Sampled mode SHALL require a nonzero interval; unsupported modes and nonzero intervals for full/disabled modes SHALL reject without storing configuration. Builder configuration SHALL be copied and preserved by the immutable definition. Nil Clock SHALL select the built-in monotonic source for enabled timing. A configured Clock SHALL remain dormant during Build and snapshot reads, and disabled/unsampled dispatch SHALL not call it. NodeMetrics SHALL expose MeasuredCalls, MeasuredNanoseconds, and MaxMeasuredNanoseconds. Custom callbacks SHALL satisfy the nanosecond, modular monotonic, concurrent-safe, nonblocking/nonallocating contract. Timing SHALL not enable exporters, logger timing, or extrapolated estimates.

#### Scenario: Configure sampled timing
- **WHEN** a caller sets sampled timing with interval three before Build
- **THEN** each node independently measures returned calls one, four, seven, and so forth using the configured source or built-in fallback

#### Scenario: Configure a dormant clock
- **WHEN** a clock is configured with timing disabled
- **THEN** registration, Build, processing, and snapshots do not invoke it

### Requirement: Bound fixed numeric counter updates
The graph contract SHALL expose RegisterCounter(*Builder, NodeID, CounterID) error before Build. Counter IDs SHALL be nonzero and unique within their node; invalid/duplicate/unknown-node registration SHALL not add slots. Generated explicit and closure handlers SHALL expose optional typed BindCounter callbacks invoked during normalization with each registered ID and its bound func(uint64) update function. Executor.Counter SHALL resolve registered handles in the control plane. Updates SHALL use fixed atomic slots without runtime registry growth or labels, wrap modulo uint64, and become no-ops after completed stop. Metrics SHALL return ordered copied Counters rows.

#### Scenario: Reuse IDs in distinct nodes
- **WHEN** two nodes register the same numeric ID
- **THEN** each receives an independent update function and snapshot value

### Requirement: Independent optional worker polling timing
SetWorkerTiming SHALL configure an optional WorkerTimingConfig with Every and Clock fields before Build. Zero Every SHALL disable collection. A nil clock SHALL retain the default monotonic source; custom clock obligations SHALL match handler timing without enabling handler measurements. Group snapshots SHALL report MeasuredPasses, IdleNanoseconds, and WorkNanoseconds based on the scheduler's reported progress. These SHALL be sampled polling wall durations, not CPU usage or extrapolated totals.

#### Scenario: Leave worker timing disabled
- **WHEN** no nonzero worker timing interval is configured
- **THEN** worker passes and snapshots perform no worker timestamp calls
