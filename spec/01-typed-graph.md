# Typed data and graph

This document consolidates the current normative requirements. Scenarios are retained where they add behavioral detail; shared concepts should be read together across sections.


## typed-pipeline-contracts

Define the public typed pipeline contracts, including executable batch-pool ownership and lifecycle semantics.


### Go module and public contract packages

The repository SHALL define a Go module requiring Go 1.22 or newer and SHALL provide compileable public contract packages for executable typed batch handles and pools, typed graph ports and connections, node result types, typed queue operations, and runtime configuration. The declarations and batch steady-state operations SHALL not use reflection, runtime type assertions, or `interface{}` to represent data-plane values.

#### Compile public contracts
- the module is tested with Go 1.22 or newer
- all public packages compile and the batch package works without requiring an executable graph runtime


### Statically typed port connections

The graph contract SHALL expose `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)` for ordinary typed ports and `func ConnectError[E any](builder *Builder, output ErrorPort[E], input InPort[E], config EdgeConfig[E]) (EdgeID, error)` for registered typed error ports, both owned by an open `Builder`. `EdgeConfig[T]` SHALL expose `Capacity uint32`, `BatchCapacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]`. `queue.Factory[T]` SHALL have the exact signature `func(queue.Config) (queue.QueueOps[T], error)`, and `queue.Config` SHALL expose matching `Capacity uint32`, `BatchCapacity uint32`, and `Policy queue.Policy` fields. A valid connection SHALL mutate the builder only once and return a dense stable `EdgeID`; incompatible ordinary or error element types SHALL be rejected by the Go compiler. Valid port handles SHALL be created by typed node registration; zero-value and cross-builder handles SHALL be rejected at runtime without topology mutation or edge-ID consumption. Ordinary `Connect` SHALL reject only ordinary-port invalidity and SHALL not accept an error handle.

#### Connect matching heterogeneous graph ports
- a program registers and connects matching int ports, matching string ports, and matching typed error ports in one open builder with nonzero queue and batch capacities
- the program compiles and the builder records all typed edges with dense stable edge IDs

#### Reject incompatible port types at compile time
- a program attempts to pass `OutPort[int]` and `InPort[string]`, or `ErrorPort[int]` and `InPort[string]`, to one corresponding connect call
- compilation fails before graph connection validation

#### Require an explicit error route
- a caller passes a registered `ErrorPort[E]` to ordinary `Connect`
- compilation fails because ordinary Connect requires `OutPort[E]`

#### Reject a compile-compatible error output at runtime
- legacy source attempts to pass the registered typed error handle to ordinary `Connect` under the prior contract
- the source no longer compiles because registration returns `ErrorPort[E]`; callers must use `ConnectError` and no runtime unavailable-routing error is needed


### Batch ownership contract

The batch contract SHALL provide explicit allocation-free ownership transfer and release for batches acquired from a typed pool. A caller that acquires a batch SHALL own it until it moves the batch or releases it; `Move` SHALL clear the source handle, and `Pool.Release` and `Batch.Release` SHALL clear the caller's handle. `Batch.Release` SHALL return an owned non-empty batch to the pool that allocated its slot, including after the batch has moved through compatible typed edges. The contract SHALL document that handlers MAY move an input only to a compatible output and that an un-moved, un-released input is returned to its originating source pool by the executor. Moving or explicitly releasing the handler's by-value copy SHALL relinquish automatic input cleanup, so a retained runner copy SHALL not release or reset a transferred slot. Invocation-local transfer bookkeeping SHALL remain allocation-free and SHALL not travel with independently moved batches. General runtime checks for arbitrary handle misuse SHALL NOT be required.

#### Move an acquired batch
- a caller moves an acquired batch to another handle
- the destination retains the slot and the source is an empty handle that cannot release the slot again through correct API use

#### Release an origin-owned moved batch
- an executor releases an un-moved batch after it has crossed one or more compatible edges
- Batch.Release returns it to its original allocating pool without type erasure or a runtime pool lookup

#### Preserve a move from a by-value handler input
- a handler moves its copied input handle into a compatible committed output
- the runner's retained cleanup handle does not release the transferred slot, and downstream retains exclusive ownership of its original length and items


### Preallocated typed pool

`NewPool[T]` SHALL validate a nonzero pool capacity and batch capacity, reject invalid size arithmetic, and allocate all slot metadata and typed batch storage before returning a pool. A pool SHALL NOT resize or allocate additional batch storage after construction.

#### Construct a valid pool
- a caller constructs a pool with positive slot and batch capacities
- the pool contains exactly the configured number of fixed-capacity typed batch slots

#### Reject invalid pool configuration
- a caller constructs a pool with zero capacity, zero batch capacity, or overflowing storage size
- construction returns an error without creating a usable pool


### Controlled batch slice access

An acquired `Batch[T]` SHALL expose `Mutable()` as its full fixed-capacity writable storage and SHALL expose `Items()` limited to its explicit logical length. `SetLen(n)` SHALL accept only a length from zero through `Cap()` and SHALL reject an out-of-range length without allocating. Callers SHALL fill `Mutable()` by index and SHALL NOT rely on capacity-growing append behavior.

#### Fill and publish logical content
- a caller writes elements into `Mutable()` and sets a valid logical length
- `Items()` exposes exactly those logical elements and `Cap()` remains the configured batch capacity

#### Reject oversized logical content
- a caller sets a logical length greater than the batch capacity
- the operation reports failure and the batch remains within its fixed capacity


### Allocation-free pool lifecycle

`Acquire` SHALL return an owned typed batch and availability indicator without allocating after pool construction. If every slot is acquired, `Acquire` SHALL report exhaustion with no batch and SHALL NOT allocate a replacement. `Release` of an owned batch SHALL return its slot for reuse without allocating. Stage 1 pool operations SHALL be used by one owner goroutine and SHALL NOT promise concurrent safety.

#### Reuse a released batch
- a caller acquires, fills, and releases a batch from a non-exhausted pool
- a later acquire can reuse a pool slot without allocation

#### Exhaust a pool
- a caller acquires every configured slot and requests one more batch
- acquire reports exhaustion without allocation or capacity growth


### Configurable release clearing

When `ClearOnRelease` is enabled, release SHALL zero the complete fixed-capacity storage of the released batch before making its slot reusable. When disabled, release SHALL retain element values until overwritten and SHALL document the pointer-retention risk.

#### Clear a previously populated slot
- a caller releases a batch with clearing enabled and later reacquires its slot
- every element of the slot's full batch capacity is the zero value

#### Retain storage when clearing is disabled
- a caller releases a batch with clearing disabled
- the pool does not perform clearing work and its documentation identifies possible retained references


### Node and error result contract

The node contract SHALL define sealed output shapes `Outputs0` through `Outputs8` for zero through eight ordinary typed outputs, and every shape SHALL implement exact exported method `OutputCount() uint8`. `Outputs0` SHALL remain an empty output shape; `Outputs1` through `Outputs8` SHALL expose corresponding typed `Output[T]` fields so a handler can acquire or commit at most one batch per ordinary output in one invocation. It SHALL define explicit-state handler sets `Node0` through `Node8` and state-capturing closure handler sets `ClosureNode0` through `ClosureNode8`. `Node0` SHALL use a typed `SourceHandler`; every input of `Node1` through `Node8` SHALL have a separate `InputHandler` whose batch element type matches that input, and closure handler fields SHALL preserve the equivalent per-input types without an explicit state parameter. Multi-input explicit and closure handler sets SHALL respectively expose typed selectors over `ReadyMask`; `ReadyMask` SHALL represent eight input positions through zero-based `PortIndex` constants `Port1` through `Port8`. Whenever called for multiple ready inputs, a selector MUST return a `PortIndex` that is both within its handler set's declared input range and marked ready in the supplied mask; the executor SHALL invoke and enforce this contract.

Every handler SHALL return `Result[E]`. Registration SHALL expose that `E` as one distinct optional `ErrorPort[E]` beyond the eight ordinary outputs. A failed result SHALL carry its unboxed `Value` only to a matching explicit error route or be discarded when unconnected; it SHALL not be represented by a node-package output placeholder. The declarations and execution bindings SHALL NOT require reflection, runtime type assertions, or boxed data-plane values.

#### Declare typed handler outcome
- a developer declares an explicit or closure handler set with error type E
- every source or input handler returns `Result[E]` without boxing its inputs, outputs, or error value

#### Compile all handler-set arities
- a developer instantiates explicit and closure handler sets at each supported input arity
- `Node0` through `Node8` and `ClosureNode0` through `ClosureNode8` compile against any generated output shape and concrete error type

#### Report exact ordinary output count
- a developer calls `OutputCount()` on any generated output shape
- the call returns that shape's ordinary arity from zero through eight as `uint8` without counting the distinct error port

#### Preserve heterogeneous input types
- a two-input node declares int as its first input and string as its second input
- its first handler accepts `batch.Batch[int]`, its second handler accepts `batch.Batch[string]`, and both share the same output-shape and error types

#### Reject arbitrary output shape
- external code attempts to instantiate a handler set with a type outside `Outputs0` through `Outputs8`
- compilation fails because the type cannot implement the sealed OutputSet contract

#### Preserve all ordinary outputs with error routing
- a handler uses `Outputs8`, returns a typed failed outcome, and its registration has a matching error connection
- all eight ordinary output types remain available and the failed `E` is routed through the separate typed error port

#### Address ready inputs by mask position
- a selector is called with multiple ready bits in a handler set's readiness mask
- it returns a zero-based `Port1` through `Port8` index that is within that handler set's input arity and whose bit is ready

#### Keep error outcome outside ordinary outputs
- a handler uses `Outputs8` and returns a typed error outcome
- all eight ordinary output types remain available, `Result[E]` carries the error type without a node-package placeholder handle, and registration exposes E through the distinct optional error port


### Queue and execution semantics contract

An automatic grouping preview SHALL remain non-executable until `FinalizeAutomaticPlan` consumes that current preview and exactly one unique nonnegative CPU binding is supplied for every proposed physical group. `BindAutomaticPlanCPU` SHALL validate owner/revision/group identity, reject duplicate CPU and group assignments, preserve already hard-bound CPU values, and not change the manual plan. Incomplete, duplicate, negative, foreign, or stale assignments SHALL reject before callbacks or sealing. Successful finalization SHALL freeze physical nodes/groups/edges, selected typed queue models, replica and worker identities, and CPU bindings in an immutable definition. Ordinary Builder.Build and manual Start semantics SHALL remain unchanged unless a CPU-complete plan is explicitly finalized. Every physical worker then follows the existing locked-thread and portable affinity-fallback contract; queue ownership/Wait/Drop semantics remain unchanged.

#### Keep manual Build as the default
- automatic preview is enabled but no explicit finalization is requested
- Builder.Build continues to use the existing manual topology and placements rather than applying preview merges

#### Reject incomplete or invalid automatic CPU assignment
- finalization is requested with missing, negative, duplicate, foreign, or stale group CPU bindings
- it rejects before sealing or invoking factories

#### Start an immutable CPU-complete plan
- a current automatic preview receives one valid unique CPU per physical group and is finalized
- Build returns the finalized definition and Start creates one fresh locked worker per physical group with selected queue plans

#### Reuse automatic plan sequentially
- a finalized automatic executor stops and the same definition starts again
- runtime state and affinity results are fresh while logical topology and physical CPU/queue plans remain unchanged

#### Review CPU-bound manual multi-group behavior
- documentation is reviewed after automatic CPU finalization is implemented
- fixed manual execution remains the default and automatic execution requires explicit preview, assignment, and finalization

#### Build without runtime resource creation
- a graph or preview is built before automatic finalization
- factories and runtime queue/worker resources remain dormant

#### Start materializes fresh CPU-bound fixed-plan resources
- a finalized plan starts sequentially
- every physical group gets fresh resources on its bound CPU

#### Build without runtime error resource creation
- a typed-error graph is previewed or built before finalization
- no error pool, batch, queue, or error value is created

#### Review CPU-bound manual multi-group behavior
- documentation is reviewed after automatic plan finalization support is added
- the current fixed manual executor and explicit automatic-finalization stages are distinguished

#### Build without runtime resource creation
- a graph is built before automatic CPU finalization
- only immutable topology and dormant numeric plans exist; factories and workers remain uncreated

#### Start materializes fresh CPU-bound fixed-plan resources
- a finalized automatic plan starts sequentially
- fresh state, queues, workers, and affinity outcomes match its retained physical plan

#### Build without runtime error resource creation
- a typed-error graph is built before CPU finalization
- no error pool, batch, queue, or error value is created

#### Review manual and automatic plan side by side
- an opt-in preview is requested and then the definition is built in current manual mode
- preview rows are independent and exploratory while executable placements remain the existing manual fixed plan

#### Preserve physical hard constraints
- a preview contains replicas, node pools, explicit placements, or ingress boundaries
- resulting groups never merge across those mandatory constraints and required CPU slots remain visible

#### Reject stale preview use
- registration, connection, or placement changes after a preview
- the preview's freshness check reports stale and it cannot be treated as executable plan data

#### Start materializes a fixed manual plan
- a definition starts after a constraints preview
- it executes the original manual or replicated fixed plan without applying preview merges

#### Keep typed error resources dormant
- a graph with typed error edges builds while exposing planning constraints
- no error pool, queue, batch, or error value is created by Build or preview

#### Start materializes a fresh CPU-bound manual plan
- a caller starts the unchanged manual definition after inspecting constraints
- it creates fresh resources on the original group and CPU plan without applying automatic merges

#### Preserve error resource dormancy at Build
- a built graph has typed error routes alongside constraint preview data
- Build still creates no error queue, pool, batch, or failure value

#### Leave unplaced nodes free only in automatic preview
- automatic preview is enabled for unplaced nodes
- those nodes have no hard group label in the returned constraints while ordinary manual Build remains assigned to DefaultGroupID

#### Preserve ordinary Connect cardinality
- a caller attempts a second ordinary producer for an already-connected input without FanIn
- ordinary Connect rejects it and preserves existing topology

#### Review a pooled node boundary
- a logical node with one ordinary input and one output is assigned three fixed workers
- copied queue plans report one shared multi-consumer input and the correctly selected multi-producer output transport while adjacent nodes retain their groups

#### Build without runtime worker creation
- a valid node pool with complete CPU assignments is built
- Build stores immutable worker metadata and dormant factories without constructing state, queue operations, or workers

#### Start fresh pooled instances
- a pooled definition starts after a prior executor stops
- every physical worker receives fresh state and affinity outcome while logical topology remains unchanged

#### Distribute ingress across replicated consumers
- one unreplicated typed ingress feeds three replicas through a shared SPMC boundary
- accepted batches are consumed once across the three replicas with independent input/output pool ownership

#### Preserve one source pool per producer
- three replica workers publish through a shared MPSC boundary
- each worker acquires from its own typed pool and downstream release returns each slot to its producing replica's pool

#### Resolve an expanded physical edge
- the pure resolver receives nonzero producer/consumer worker counts and a physical ownership classification
- it selects Local, SPSC, MPSC, SPMC, or MPMC according to the cardinality matrix and returns an error for zero or inconsistent local cardinality


### Typed external ingress source

AddIngress[T] SHALL register a dormant typed source with one ordinary output, a NodeID, and an IngressDefinition[T] handle. BindIngress SHALL validate provenance and return a fresh executor-local single-producer endpoint after Start. Invalid, foreign, or stopped bindings SHALL reject. Producer operations SHALL be serialized by one external caller; shutdown admission closure SHALL be allowed concurrently. The adapter SHALL process only accepted input work during drain and SHALL not count framework adapter polling as user handler calls.

#### Connect external ingress
- a typed ingress output is connected to a matching input and the definition starts
- its bound producer can submit owned or copied batches without boxing T


### External ownership and status outcomes

Ingress Acquire SHALL return owned preallocated source storage. Commit SHALL return explicit accepted/full/closed/drop/invalid/oversize statuses, clearing the owned ingress handle only on acceptance or configured Drop. Submit SHALL reject oversize input atomically, copy one bounded batch, and release its private acquisition on nonaccepted outcomes. Uncommitted caller-owned batches SHALL remain releasable after stop. Ingress source pool rows SHALL use their registered source NodeID with EdgeID zero; ordinary edge pools SHALL keep their edge-origin identity.

#### Release caller-owned work after stop
- a caller retains an uncommitted ingress acquisition through completed shutdown
- Commit rejects without consuming it and Release returns it safely to its retained source pool


### Dormant typed logging mapper declarations

Generated explicit-state and closure handler families SHALL expose optional typed `MapError ErrorMapper[E]` configuration matching the handler's concrete failure type. Typed mapper declarations and explicit conversion SHALL be available independently of asynchronous logging. Registration and Build SHALL not invoke the mapper, and Start without a logging destination SHALL leave runtime conversion dormant. Existing deferral of runtime logging/error-mapping execution SHALL not prohibit these typed declarations or explicit conversion helpers. Configured callbacks SHALL use immutable captures and SHALL be documented as nonblocking/nonallocating when invoked by a worker.

#### Configure without logging activation
- a caller supplies a mapper matching a node's concrete E and starts the definition without a logger
- failure routing remains typed and the mapper callback remains uncalled

#### Reject a mismatched mapper declaration
- a caller assigns an ErrorMapper[string] to an int-failure node's MapError field
- Go compilation rejects the mismatched function type before graph registration


### Explicit asynchronous logger configuration

The graph contract SHALL expose `LoggerConfig` with per-worker `Capacity uint32` and `Handle func(node.ErrorEvent)`, and `SetLogger(*Builder, LoggerConfig) error`. An open initialized builder SHALL accept at most one valid logger configuration. Zero/unrepresentable capacity and a nil handler SHALL reject without storing configuration; invalid/sealed builders and duplicate valid configuration SHALL report their corresponding categories. Configuration SHALL be copied, with user callback captures remaining immutable caller-owned configuration. This explicit capability enables bounded asynchronous logging for configured definitions; definitions without configuration SHALL retain the deferred/dormant logging behavior. Worker publication SHALL require no boxed E or formatted event payload.

#### Reject an invalid logger
- SetLogger receives a zero-capacity configuration or nil callback
- it reports an invalid logger and the builder remains available for a corrected configuration


### Ordered numeric logger outcomes

Executor Metrics SHALL include independently copied per-worker logger rows with group identity and attempted, accepted, full-ring dropped, delivered, and shutdown-discarded event counts. Disabled logging SHALL return no logger rows. After completed stop, accepted events SHALL equal delivered plus shutdown-discarded events; attempted events SHALL equal accepted plus full-ring dropped events. Nil MapError configuration SHALL produce a zero numeric payload with runtime-owned identity when logging is installed.

#### Inspect a stopped saturated logger
- a logger stops after ring-full losses and forced discard
- final counters distinguish producer drops from accepted-event delivery/discard, and modifying returned rows does not alter future snapshots


### Opt-in processing timing configuration

The graph contract SHALL expose TimingConfig with Mode, Every, and optional Clock func() uint64 fields and SetTiming(*Builder, TimingConfig) error. The default SHALL disable timing without clock reads. Sampled mode SHALL require a nonzero interval; unsupported modes and nonzero intervals for full/disabled modes SHALL reject without storing configuration. Builder configuration SHALL be copied and preserved by the immutable definition. Nil Clock SHALL select the built-in monotonic source for enabled timing. A configured Clock SHALL remain dormant during Build and snapshot reads, and disabled/unsampled dispatch SHALL not call it. NodeMetrics SHALL expose MeasuredCalls, MeasuredNanoseconds, and MaxMeasuredNanoseconds. Custom callbacks SHALL satisfy the nanosecond, modular monotonic, concurrent-safe, nonblocking/nonallocating contract. Timing SHALL not enable exporters, logger timing, or extrapolated estimates.

#### Configure sampled timing
- a caller sets sampled timing with interval three before Build
- each node independently measures returned calls one, four, seven, and so forth using the configured source or built-in fallback

#### Configure a dormant clock
- a clock is configured with timing disabled
- registration, Build, processing, and snapshots do not invoke it


### Bound fixed numeric counter updates

The graph contract SHALL expose RegisterCounter(*Builder, NodeID, CounterID) error before Build. Counter IDs SHALL be nonzero and unique within their node; invalid/duplicate/unknown-node registration SHALL not add slots. Generated explicit and closure handlers SHALL expose optional typed BindCounter callbacks invoked during normalization with each registered ID and its bound func(uint64) update function. Executor.Counter SHALL resolve registered handles in the control plane. Updates SHALL use fixed atomic slots without runtime registry growth or labels, wrap modulo uint64, and become no-ops after completed stop. Metrics SHALL return ordered copied Counters rows.

#### Reuse IDs in distinct nodes
- two nodes register the same numeric ID
- each receives an independent update function and snapshot value


### Independent optional worker polling timing

SetWorkerTiming SHALL configure an optional WorkerTimingConfig with Every and Clock fields before Build. Zero Every SHALL disable collection. A nil clock SHALL retain the default monotonic source; custom clock obligations SHALL match handler timing without enabling handler measurements. Group snapshots SHALL report MeasuredPasses, IdleNanoseconds, and WorkNanoseconds based on the scheduler's reported progress. These SHALL be sampled polling wall durations, not CPU usage or extrapolated totals.

#### Leave worker timing disabled
- no nonzero worker timing interval is configured
- worker passes and snapshots perform no worker timestamp calls


## typed-graph-builder

Define typed graph builder ownership, registration, opaque handles, and generated port families.


### Exact builder identity and state contract

The graph package SHALL expose `func NewBuilder() *Builder`, `var ErrInvalidBuilder`, `var ErrBuilderSealed`, and `var ErrNodeIDExhausted`. `NewBuilder` SHALL return an open mutable builder with initialized registration state and a fresh private owner pointer. The pointed-to owner type SHALL have nonzero size so owner pointers from distinct live builders cannot compare equal. The zero-value `Builder` and a nil `*Builder` SHALL be invalid. Builder state SHALL represent invalid, open, and sealed values, but this capability SHALL provide no operation that transitions an open builder to sealed.

Every registration function SHALL return `ErrInvalidBuilder` for invalid state, `ErrBuilderSealed` for sealed state, and `ErrNodeIDExhausted` instead of wrapping node identity. Those failures SHALL leave registration state unchanged.

#### Construct builders with distinct owners
- a caller invokes `NewBuilder` twice and keeps both builders live
- each builder is open and their private non-zero-sized owner pointers compare unequal

#### Reject an uninitialized builder
- a caller registers through a nil builder pointer or a zero-value `Builder`
- registration returns `ErrInvalidBuilder` without consuming a node identity or returning valid handles

#### Reserve sealed-state rejection
- a same-package test places a builder in the reserved sealed state and attempts registration
- registration returns `ErrBuilderSealed` without mutation even though this capability exposes no sealing operation


### Zero-invalid stable node and port identities

The exact ID declarations SHALL be `type NodeID uint32`, `const InvalidNodeID NodeID = 0`, `type PortID uint8`, and `const InvalidPortID PortID = 0`. Successful registrations SHALL receive dense stable `NodeID` values beginning at 1 in registration order. Inputs and outputs SHALL use separate node-local `PortID` namespaces; input and ordinary-output positions SHALL use IDs 1 through 8, and the separate typed error output SHALL use output `PortID(9)`. Rejected registrations SHALL NOT consume an ID.

#### Assign dense node identities
- three nodes are registered successfully
- their node IDs are 1, 2, and 3 in registration order and remain stable

#### Assign one-based port identities
- a node with two inputs and two ordinary outputs is registered
- its input IDs are 1 and 2, its ordinary-output IDs are 1 and 2, and its typed error-output ID is 9

#### Preserve identity after rejection
- one registration fails and the next registration succeeds
- the successful registration receives the node ID that was next before the failure


### Exact opaque handle APIs

The graph package SHALL define opaque `InPort[T]`, `OutPort[T]`, `ErrorPort[E]`, and `OutputRef[O]` values with private owner and identity fields. `InPort[T]`, `OutPort[T]`, and `ErrorPort[E]` SHALL expose exactly the value-receiver accessors `NodeID() NodeID`, `PortID() PortID`, and `Valid() bool`. `OutputRef[O]` SHALL expose exactly `NodeID() NodeID`, `OutputCount() uint8`, and `Valid() bool`; it SHALL represent all ordinary outputs for one node and SHALL NOT expose a singular `PortID`.

Each singular handle's `Valid` method SHALL require a non-nil owner and nonzero node and port IDs. `OutputRef.Valid` SHALL require a non-nil owner, a nonzero node ID, and an output count from zero through eight. External callers SHALL have no owner constructor or field access, so a zero-value handle remains invalid while preserving its generic type. `ErrorPort[E]` SHALL not be assignable to `OutPort[E]`.

#### Inspect registered singular handles
- a caller inspects registered input, ordinary-output, and error-output handles
- each handle reports its stable node and port IDs and `Valid` reports true

#### Inspect a registered output reference
- a caller inspects an `OutputRef[node.Outputs3[int, string, bool]]` returned by registration
- it reports the registered node ID, output count 3, and valid status without exposing a port ID

#### Keep an error handle distinct
- external code attempts to use `ErrorPort[int]` where `OutPort[int]` is required
- compilation fails without a runtime provenance or type check

#### Inspect zero-value handles
- a caller inspects zero-value `InPort[int]`, `OutPort[string]`, `ErrorPort[error]`, and `OutputRef[node.Outputs0]` values
- singular handles report invalid IDs, the output reference reports an invalid node ID, and every `Valid` call reports false


### Exact NodePorts API

The graph package SHALL define opaque `NodePorts[I, O, E]`, with `O` constrained by `node.OutputSet`, and exactly these value-receiver accessors: `NodeID() NodeID`, `Inputs() I`, `Outputs() OutputRef[O]`, and `Errors() ErrorPort[E]`. A successful registration SHALL return one `NodePorts` whose four accessors refer to one owner and node. A zero-value `NodePorts` SHALL return invalid component handles and `InvalidNodeID`.

#### Access a heterogeneous registration result
- registration succeeds for two differently typed inputs, three differently typed ordinary outputs, and a typed error
- `NodeID`, `Inputs`, `Outputs`, and `Errors` return the matching typed values for one stable node identity

#### Access zero-value NodePorts
- a caller invokes every accessor on a zero-value `NodePorts`
- `NodeID` is invalid and the returned input, output reference, and error port are invalid without a panic


### Complete generated graph port and expansion families

The shared arity generator SHALL generate and commit graph-owned `InPorts0` through `InPorts8` with fields named `In1` through `In8` as applicable and `OutPorts0` through `OutPorts8` with fields named `Out1` through `Out8` as applicable. The generated expansion function names SHALL be exactly `OutputPorts0`, `OutputPorts1`, through `OutputPorts8`. `OutputPortsN` SHALL accept only `OutputRef[node.OutputsN[...]]` and return the matching `OutPortsN[...]`.

Registration SHALL obtain ordinary output arity by evaluating the zero value of `O node.OutputSet` and calling its prerequisite `OutputCount() uint8` method, without reflection or type assertion. It SHALL store that count in `OutputRef`. `OutputPortsN` SHALL require the stored count to equal N, preserve owner and node identity, and derive ordinary-output IDs 1 through N; a count mismatch or invalid reference SHALL return a zero invalid bundle.

#### Compile all exact generated names
- compile tests instantiate every `InPortsN`, `OutPortsN`, and `OutputPortsN` declaration from zero through eight
- all exact names compile and each nonzero field has its declared concrete element type

#### Expand heterogeneous outputs
- `OutputPorts3` expands a valid `OutputRef[node.Outputs3[int, string, bool]]`
- it returns `Out1 OutPort[int]`, `Out2 OutPort[string]`, and `Out3 OutPort[bool]` with port IDs 1, 2, and 3 for the reference owner and node

#### Reject an invalid expansion reference
- an `OutputPortsN` function receives an invalid reference or a same-package test supplies a stored count other than N
- it returns a zero `OutPortsN` containing no valid handles


### Exact generated registration families

The graph package SHALL generate package functions named exactly `AddNode0` through `AddNode8` and `AddClosureNode0` through `AddClosureNode8`. `AddNodeN` SHALL accept `*Builder` and the matching prerequisite `node.Stateful[S, node.NodeN[...]]`; `AddClosureNodeN` SHALL accept `*Builder` and the matching `node.ClosureFactory[node.ClosureNodeN[...]]`. Each SHALL return `(NodePorts[InPortsN[...], O, E], error)`, with `O` independently constrained by `node.OutputSet`. Arity zero SHALL omit input type parameters and return `InPorts0`. Each generated registration SHALL construct `ErrorPort[E]{owner: builder owner, nodeID: registered node, portID: 9}` only after successful registration.

The generated families SHALL vary only by input arity. Ordinary output arity SHALL be recovered later through the exact `OutputPortsN` functions, and the generator SHALL NOT emit an input/output arity cross-product.

#### Register explicit state
- a caller passes a prerequisite explicit-state definition to the `AddNodeN` function matching its input arity
- the function returns typed `NodePorts` with stable input, ordinary output-reference, and distinct error-port identities

#### Register a closure factory
- a caller passes a prerequisite closure factory to the `AddClosureNodeN` function matching its input arity
- the function returns the same graph-owned result shape with a distinct error-port identity and records closure registration kind

#### Register explicit state with an error handle
- a caller passes a prerequisite explicit-state definition to the `AddNodeN` function matching its input arity
- the function returns typed `NodePorts` with stable input, ordinary output-reference, and distinct error-port identities

#### Combine arities independently
- compile tests combine representative input arities with zero-, one-, and eight-output `node.OutputSet` shapes and typed error connections
- all combinations compile through the linear registration and expansion families without an input/output arity cross-product


### Precise static validation bridge

Every successful registration SHALL atomically append one private `nodeMetadata` record containing node ID, explicit or closure registration kind, input count, ordinary output count, one `staticValidation` descriptor, and one dormant `nodeNormalizer` closure. `validationMask` SHALL be a private `uint16`: bit 0 SHALL represent factory, bit 1 source handler, bits 2 through 9 input handlers 1 through 8, and bit 10 selector. `staticValidation` SHALL contain `required`, `present`, and `deferred` fields of that mask type.

Explicit-state registration SHALL mark its factory and source or input handlers as required, plus selector when input count exceeds one; it SHALL mark every statically non-nil required function as present and SHALL leave deferred empty. Closure registration SHALL mark its factory required and present when non-nil; it SHALL mark its factory-produced source or input handlers and multi-input selector as deferred because their presence is not statically observable. A later validator SHALL be able to compute missing statically known functions as `required &^ present` without invoking user code.

The exact private bridge type SHALL be `type nodeNormalizer func(*normalizationTarget) error`. `normalizationTarget` SHALL be private control-plane state with no dynamically typed fields. The generated generic registration function SHALL construct `nodeNormalizer` while all concrete types are in scope. The closure SHALL capture the complete concrete explicit-state or closure-factory contract and retain a monomorphized path to that future target. Registration SHALL store but SHALL NOT invoke the normalizer or any factory, handler, selector, initialization hook, or close hook.

Neither `nodeMetadata`, `staticValidation`, `nodeNormalizer`, nor their registration path SHALL use `any` or `interface{}` fields or parameters, reflection, or runtime type assertions to retain or recover registered values.

#### Record explicit static presence
- an explicit two-input registration has a non-nil factory and first handler but nil second handler and selector
- required marks factory, both handlers, and selector; present marks only factory and first handler; deferred is empty; and no callback runs

#### Record closure deferred presence
- a non-nil two-input closure factory is registered
- required and present mark the factory, deferred marks both produced handlers and selector, and the factory is not invoked

#### Audit dynamic mechanisms
- handwritten and generated graph registration code is inspected
- registration metadata and normalization contain no dynamic storage, reflection, or runtime type assertions


### Graph-specific generated output ownership

The shared standard-library-only arity generator MAY be extended with graph templates and SHALL define a graph-specific output manifest listing every generated graph port/bundle and registration file. A graph-specific stability test SHALL regenerate only that manifest into a temporary directory, compare every listed file byte-for-byte with committed output, and fail for a missing or unexpected generated graph file. Generated graph declarations SHALL remain in package `graph` and SHALL carry the standard generated-code header.

#### Regenerate the graph manifest
- the shared generator runs for its graph target with unchanged templates
- the graph-specific stability test reports every manifested file unchanged and no extra graph-generated output

#### Detect graph output drift
- a manifested graph file differs, is absent, or an unmanifested generated graph file appears
- the graph-specific stability test fails independently of node generated-output checks


### Acyclic package and stage boundary

The `graph` package SHALL import and consume `node` contracts, while `node` SHALL NOT import `graph`. Graph handles, bundles, registration results, and registration functions SHALL remain in `graph`. This capability SHALL add no edge model or connection behavior, no DAG or connectivity validation pass, no builder-sealing transition, no `Build` method, and no immutable-definition behavior. Reserved sealed state, `ErrBuilderSealed`, static masks, and dormant normalization SHALL be extension points only.

#### Verify package direction
- package dependencies are inspected and compiled
- dependency direction is `node <- graph` with no import cycle or graph handle in `node`

#### Verify focused builder state
- nodes are registered successfully
- the builder contains owner, lifecycle state, node identities, static validation descriptors, and dormant normalizers but no edges, topological order, build result, or immutable definition


## typed-graph-connections

Define typed graph edge configuration, connection validation, and dormant queue bindings.


### Typed per-runtime queue factory

The queue contract SHALL declare the exact API `type Factory[T any] func(Config) (QueueOps[T], error)`. Each future runtime instance SHALL invoke a configured factory separately for each local edge it binds, and a factory invocation SHALL return queue operations for the same element type `T` or an error. A nil local-edge factory SHALL select the future built-in local queue. A cross-group edge classified by Build SHALL use the framework-owned remote SPSC queue and SHALL reject a non-nil configured factory before runtime creation.

#### Preserve typed local custom queue construction
- an `EdgeConfig[int]` contains a custom `Factory[int]` and its endpoints have the same effective group
- the edge retains a constructor that can produce only `QueueOps[int]` for that future local runtime edge

#### Create fresh local operations for each future runtime
- two future runtime instances bind the same local edge with a custom factory
- each runtime invokes the factory independently and does not reuse queue operations created for the other runtime

#### Reject a custom remote queue
- a configured custom factory edge is classified cross-group by Build
- Build rejects the graph without invoking the factory and the future runtime uses no custom remote operations


### Valid typed edge configuration

The graph contract SHALL declare `type EdgeConfig[T any] struct` with exactly `Capacity uint32`, `BatchCapacity uint32`, `Policy queue.Policy`, and `Factory queue.Factory[T]` fields. One package-private pure function with the exact signature `func validateEdgeConfig(queue.Config) error` SHALL accept only positive capacity and batch capacity and the declared `queue.Drop` and `queue.Wait` policy values. `Connect[T]` and `ConnectError[E]` SHALL convert their configuration to `queue.Config` and use this validator before retaining configuration. Graph-wide validation SHALL reuse the same validator against stored ordinary and error edge metadata rather than duplicating these rules. The validator and both connection functions SHALL NOT invoke a custom factory.

#### Accept valid default queue configuration
- matching ordinary ports are connected with positive queue and batch capacities, a declared policy, and no custom factory
- the connection succeeds with configuration for a future built-in typed queue

#### Reject invalid edge configuration
- an ordinary connection uses zero capacity, zero batch capacity, or a policy value other than `queue.Drop` or `queue.Wait`
- the shared validator returns `*EdgeConfigError` matching `ErrInvalidEdgeConfig`, and the connection records no edge, occupies neither port, invokes no factory, and consumes no edge ID

#### Keep custom factory dormant
- matching ordinary ports are connected with a valid configuration containing a custom factory
- `Connect` records the typed future binding without invoking the factory or retaining live `QueueOps[T]`

#### Reuse configuration validation later
- graph-wide validation checks stored ordinary or error edge capacity, batch capacity, and policy
- it calls the same pure `validateEdgeConfig` function used by both connection APIs and does not invoke the stored factory

#### Accept a valid error edge configuration
- matching error and input ports are connected with positive queue and batch capacities, a declared policy, and no custom factory
- the connection succeeds with configuration for a future built-in typed error queue

#### Reject invalid edge configuration from either API
- an ordinary or error connection uses zero capacity, zero batch capacity, or a policy value other than `queue.Drop` or `queue.Wait`
- the shared validator returns `*EdgeConfigError` matching `ErrInvalidEdgeConfig`, and the connection records no edge, occupies neither port, invokes no factory, and consumes no edge ID


### Builder-targeted dense edge creation

The graph contract SHALL declare the exact APIs `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)` and `func ConnectError[E any](builder *Builder, output ErrorPort[E], input InPort[E], config EdgeConfig[E]) (EdgeID, error)`. Each SHALL mutate only an open builder. `EdgeID(0)` SHALL be invalid, the first successful connection of either kind SHALL return ID 1, and subsequent successful connections SHALL receive consecutive IDs in commit order.

#### Record a matching typed edge
- valid matching registered ordinary ports owned by one open builder are connected with valid configuration
- the builder records exactly one edge and `Connect` returns the next dense nonzero edge ID

#### Preserve density across an error
- one successful ordinary connection is followed by a rejected connection of either kind and then another successful connection
- the successful calls return consecutive edge IDs and the rejected call returns the invalid zero ID

#### Reject a sealed builder
- `Connect` or `ConnectError` targets a builder in the sealed state established by a successful Build
- it returns `ErrBuilderSealed` and the invalid zero edge ID without changing topology

#### Record ordinary and error edges together
- valid matching registered ordinary ports and valid matching registered error and input ports are connected in commit order
- the builder records each edge and returns one dense shared nonzero EdgeID sequence

#### Preserve density across an error-route failure
- one successful connection is followed by a rejected `ConnectError` call and then another successful connection
- the successful calls return consecutive edge IDs and the rejected call returns the invalid zero ID


### Port provenance and cardinality validation

`Connect` SHALL reject zero-value ordinary input or output handles and handles not owned by the target builder. `ConnectError` SHALL reject zero-value error or input handles and handles not owned by the target builder. Each input SHALL have at most one producer across ordinary and error edges. Each ordinary output and each error output SHALL have at most one consumer. A second consumer of either kind of output SHALL be rejected explicitly as unsupported fan-out rather than assigned implicit broadcast or distribution semantics.

#### Reject a zero-value handle
- either endpoint passed to `Connect` or `ConnectError` is a zero-value typed port handle
- the connection returns `ErrInvalidPort` and leaves builder topology and edge IDs unchanged

#### Reject a foreign handle
- either endpoint passed to `Connect` or `ConnectError` belongs to a builder other than the target builder
- the connection returns `ErrForeignPort` and leaves both builders unchanged

#### Reject a second producer for one input
- a caller connects an ordinary or error output to an input that already has a producer
- the connection returns `ErrInputConnected` and preserves the existing edge

#### Reject unsupported fan-out
- a caller connects an ordinary or error output that already has a consumer to another input
- the connection returns `ErrFanOutUnsupported` and preserves the existing edge without occupying the other input

#### Reject a foreign error handle
- `ConnectError` receives an error output or input from a builder other than the target builder
- it returns `ErrForeignPort` and leaves both builders unchanged

#### Reject an ordinary producer for an error-fed input
- an input already has a successful error-edge producer and a caller attempts ordinary `Connect` to it
- Connect returns `ErrInputConnected` and preserves the existing error edge

#### Reject unsupported error fan-out
- a caller connects an error output that already has a consumer to another input
- `ConnectError` returns `ErrFanOutUnsupported` and preserves the first edge without occupying the other input


### Error-edge validation and dormant typed binding

`ConnectError` SHALL complete builder state, error-handle validity and provenance, input validity and provenance, input/output cardinality, and the shared pure edge-configuration validation before committing state. It SHALL then atomically append error-edge structural metadata, occupy its input and error output, and retain a monomorphized future-binding closure for `E`. A failure SHALL perform none of these actions, SHALL not consume an edge ID, and SHALL not retain or invoke the rejected factory. The binding SHALL preserve typed error pool and queue operations without reflection, runtime type assertions, or data-plane values stored in `any`.

#### Keep an error queue factory dormant
- matching error and input ports are connected with a valid local custom `Factory[E]`
- ConnectError records the future typed binding without invoking the factory or retaining live `QueueOps[E]`

#### Reject invalid error configuration atomically
- ConnectError receives zero capacity, zero batch capacity, or an unsupported policy after all higher-precedence checks pass
- it returns an `EdgeConfigError`, records no edge, occupies no port, invokes no factory, and consumes no edge ID


### Stable connection errors and deterministic precedence

The graph package SHALL export `ErrBuilderSealed`, `ErrInvalidPort`, `ErrForeignPort`, `ErrInputConnected`, `ErrFanOutUnsupported`, and `ErrInvalidEdgeConfig` as stable sentinel errors. It SHALL export `type EdgeConfigError struct { Config queue.Config }` with `Error() string` and `Unwrap() error` methods, and `EdgeConfigError.Unwrap` SHALL return `ErrInvalidEdgeConfig`. Callers SHALL be able to match every category with `errors.Is` and inspect configuration failures with `errors.As`.

`Connect` SHALL select one error using this exact precedence: sealed builder; invalid ordinary output then invalid input; foreign ordinary output then foreign input; connected input; connected output as unsupported fan-out; invalid edge configuration. `ConnectError` SHALL use the equivalent precedence with invalid and foreign error output checks in the ordinary-output positions. Every failure SHALL return `EdgeID(0)`.

#### Match an invalid configuration error
- `Connect` rejects capacity, batch capacity, or policy after all higher-precedence checks pass
- `errors.Is` matches `ErrInvalidEdgeConfig` and `errors.As` yields `*EdgeConfigError` containing the rejected `queue.Config`

#### Select the higher-precedence error
- one ordinary connection attempt violates more than one local invariant
- `Connect` returns the sentinel or typed error for the first failing check in the specified precedence

#### Observe sealed state without performing sealing
- a successful Build has transitioned a builder to sealed and a caller invokes `Connect` or `ConnectError`
- the connection returns `ErrBuilderSealed`, while this capability itself defines no Build operation and performs no sealing transition

#### Match an invalid error-edge configuration
- ConnectError rejects capacity, batch capacity, or policy after all higher-precedence checks pass
- `errors.Is` matches `ErrInvalidEdgeConfig` and `errors.As` yields `*EdgeConfigError` containing the rejected `queue.Config`

#### Select the higher-precedence error for an error route
- one ConnectError attempt violates more than one local invariant
- it returns the sentinel or typed error for the first failing check in the specified precedence


### Atomic connection errors and typed future binding

`Connect` SHALL complete every local validation, including the shared pure edge-configuration validator, before committing connection state. On success it SHALL atomically append structural edge metadata, occupy the input and output, and retain a monomorphized future-binding closure for `T`. On error it SHALL perform none of those actions and SHALL NOT retain or invoke the rejected factory or consume an edge ID. The future-binding closure SHALL remain dormant during connection and future graph Build validation and SHALL preserve typed queue operations without reflection, runtime type assertions, or storage of data-plane values in an `any` registry.

#### Correct a rejected connection
- a connection attempt fails and the caller retries the same still-unoccupied ports with valid configuration
- the retry succeeds with the next dense edge ID

#### Preserve typed binding metadata
- `Connect[T]` records an edge with a custom `queue.Factory[T]`
- the stored future binding remains specialized for `T` and no queue instance is created during connection


## graph-dag-validation

Define deterministic graph validation, diagnostics, topology snapshots, and DAG analysis.


### Exact validation entry point and builder-state behavior

The graph package SHALL expose `func (b *Builder) Validate() (*ValidationReport, error)`. Callers SHALL construct usable builders with `NewBuilder`; a nil receiver or uninitialized zero-value receiver SHALL return a nil report and a `*ValidationError` containing exactly one `ValidationCodeInvalidBuilder` issue. An initialized builder containing no nodes SHALL return a non-nil report with empty topology snapshots and a `*ValidationError` containing exactly one `ValidationCodeEmptyGraph` issue.

Every non-empty initialized builder SHALL return a non-nil report. It SHALL return nil error exactly when that report has no issues and otherwise SHALL return a `*ValidationError` whose issues equal the report issues. Validation SHALL accept both open builders and builders sealed by a later build stage; sealed state alone SHALL NOT produce an issue or `ErrBuilderSealed`. Validation SHALL preserve the receiver's existing open or sealed state.

#### Reject nil and zero-value builders
- `Validate` is called on a nil `*Builder` or an uninitialized zero-value `Builder`
- it returns a nil report and one `ValidationCodeInvalidBuilder` issue without panicking

#### Reject an initialized empty builder
- `Validate` is called on a builder returned by `NewBuilder` before any node is registered
- it returns a non-nil empty report and one `ValidationCodeEmptyGraph` issue

#### Validate a non-empty open builder
- `Validate` is called on a non-empty initialized open builder
- it returns a point-in-time report and either nil error or a matching `ValidationError` according to that report's issues

#### Validate a sealed builder
- `Validate` is called on a builder sealed by the later immutable build stage
- it analyzes the fixed topology without returning `ErrBuilderSealed` or changing sealed state


### Reusable deterministic validation report

`ValidationReport` SHALL have unexported backing storage and SHALL expose defensive-copy `Issues() []ValidationIssue`, `TopologicalOrder() []NodeID`, `ForwardAdjacency() []ValidationAdjacency`, and `ReverseAdjacency() []ValidationAdjacency` methods. `ValidationAdjacency` SHALL contain `NodeID` and `Edges []ValidationAdjacentEdge`; `ValidationAdjacentEdge` SHALL contain `EdgeID` and `AdjacentNodeID`.

Each adjacency snapshot SHALL contain one row per registered node, including an empty row for a node with no edges, in ascending `NodeID` order. Forward rows SHALL list destination nodes and reverse rows SHALL list source nodes, with each row sorted by `AdjacentNodeID` then `EdgeID`. Snapshots SHALL contain ordinary and connected error edges and SHALL own their row and edge storage independently of builder metadata.

For every acyclic non-empty topology, `TopologicalOrder` SHALL contain every node exactly once in canonical Kahn order even when other validation issues exist. For cyclic topology it SHALL be empty rather than exposing a partial Kahn prefix. The report SHALL contain sorted issues, and its accessors SHALL deep-copy issues, cycle witnesses, topological order, adjacency rows, and adjacency edges. It SHALL be a point-in-time analysis unaffected by later mutation of an open builder and SHALL provide the complete graph analysis that a later immutable `Build` can copy without rerunning validation algorithms.

#### Snapshot a connected error edge
- an error port is connected to a matching ordinary input
- forward and reverse adjacency contain that edge in canonical order and the report remains independent of later builder mutation

#### Snapshot an acyclic invalid graph
- an initialized non-empty graph is acyclic but has static, connectivity, edge-configuration, or reachability issues
- its report contains those sorted issues, complete adjacency snapshots, and a full canonical topological order

#### Suppress a partial cyclic order
- ordinary and error edges together leave residual cyclic nodes in Kahn processing
- the report retains complete adjacency snapshots and returns an empty topological order

#### Keep a report independent of builder mutation
- a caller validates an open builder and then mutates that builder
- the previously returned report's issues, order, and adjacency snapshots remain unchanged

#### Protect nested report storage
- a caller changes any issue, witness, order, adjacency row, or edge slice returned by a report accessor
- subsequent accessors and formatting of the associated validation error remain unchanged


### Structured aggregate validation diagnostics

The graph package SHALL expose exported `ValidationCode`, `ValidationIssue`, and `ValidationError` types. `ValidationIssue` SHALL contain exported `Code ValidationCode`, `NodeID NodeID`, `PortID PortID`, `EdgeID EdgeID`, `GroupID GroupID`, `CPU int`, and `Cycle []NodeID` fields; its code SHALL determine which entity fields apply, and unused fields SHALL be zero. `ValidationError` SHALL expose `Issues() []ValidationIssue`, be discoverable with `errors.As`, contain every independently discoverable issue, and deep-copy issue and cycle storage. Its structured data and text SHALL remain identical across repeated validation of unchanged topology.

Issues SHALL sort by fixed validation-code order, then ascending `NodeID`, node-local `PortID`, `EdgeID`, `GroupID`, and `CPU`, with any cycle witness compared lexicographically as a final key. `ValidationCode` SHALL reserve an invalid zero value that validation does not emit, followed in sort order by `ValidationCodeInvalidBuilder`, `ValidationCodeEmptyGraph`, `ValidationCodeMissingFactory`, `ValidationCodeMissingHandler`, `ValidationCodeMissingSelector`, `ValidationCodeUnconnectedInput`, `ValidationCodeUnconnectedOutput`, `ValidationCodeInvalidEdgeCapacity`, `ValidationCodeInvalidEdgePolicy`, `ValidationCodeCycle`, `ValidationCodeUnreachableFromSource`, `ValidationCodeCannotReachSink`, `ValidationCodeInvalidGroupPlacement`, `ValidationCodeRemoteQueueFactory`, `ValidationCodeMissingGroupCPU`, `ValidationCodeInvalidGroupCPU`, `ValidationCodeDuplicateGroupCPU`, and `ValidationCodeIneffectiveGroupCPU`.

#### Aggregate independently discoverable failures
- a builder contains multiple missing static contracts, disconnected ordinary ports, topology failures, and CPU-binding failures
- `Validate` returns one structured `ValidationError` containing all independently discoverable issues in the specified deterministic order

#### Repeat validation without diagnostic drift
- `Validate` is called repeatedly without an intervening builder mutation
- every call returns equivalent issue fields, cycle witnesses, ordering, and error text

#### Match report and error issues
- an initialized builder returns a report with one or more issues
- the returned `ValidationError` contains exactly the same sorted issue values and owns separate backing storage

#### Identify a CPU-binding failure
- validation finds a missing, invalid, duplicate, or ineffective CPU binding
- its issue contains the affected GroupID and applicable CPU value without using a NodeID, PortID, EdgeID, or cycle witness


### Registration-produced static validation descriptors

Every generated graph registration path SHALL store a non-generic static validation descriptor containing `uint16` required and present masks in node structural metadata. Bit 0 SHALL represent the factory, bit 1 SHALL represent the selector, and bits 2 through 9 SHALL represent handler positions zero through seven. Source handler position zero SHALL represent the source handler; for a node with inputs, position `i` SHALL represent input handler `i`.

Explicit-state registration SHALL require its factory, every source or input handler, and a selector only when ordinary input arity exceeds one. Closure registration SHALL require only its factory because its handlers and selector do not exist until the factory runs. Generated generic registration SHALL determine present bits directly from statically typed function fields. Validation SHALL derive missing requirements from the stored masks and SHALL NOT use reflection, runtime type assertions, or `any` callback storage to rediscover them.

Validation SHALL report one missing-factory, missing-handler, or missing-selector issue for each applicable missing bit. It SHALL NOT invoke a state factory, closure factory, handler, selector, lifecycle callback, or other user callback. Callbacks that can exist only in a future closure-factory result SHALL NOT be inspected during builder validation.

#### Report all missing static node contracts
- registered nodes have nil required factories, nil statically visible handlers, or a missing statically visible multi-input selector
- `Validate` returns one entity-specific issue for every missing required static contract

#### Accept absent single-input selector
- a node has zero or one ordinary input and all of its other static contracts are present
- `Validate` does not report a missing-selector issue for that node

#### Keep factories and callbacks dormant
- validation examines explicit-state and closure-based nodes whose factories or callbacks have observable side effects
- validation completes without executing any factory or callback

#### Validate from homogeneous static metadata
- validation examines heterogeneous registered node types and arities
- it reports static contract issues solely from registration-produced masks without reflection, type assertions, or `any` callback storage


### Ordinary connectivity and edge configuration validation

Validation SHALL require exactly one connection for every declared ordinary input and every declared ordinary output under the existing single-consumer connection model. An ordinary input connected from either an ordinary or error output SHALL satisfy its one-producer requirement. A node SHALL be allowed to declare zero ordinary inputs or zero ordinary outputs. Error outputs SHALL be optional and SHALL not produce an unconnected-output issue.

`Connect`, `ConnectError`, and validation SHALL call the same unexported non-generic pure edge-configuration validator for capacity, batch capacity, and queue-full policy; validation SHALL NOT duplicate those predicates. Validation SHALL map every violation to the associated edge identity and validation code. The shared helper SHALL NOT mutate topology or invoke an optional typed queue factory.

#### Accept an input fed by an error edge
- a declared ordinary input has exactly one matching error-edge producer
- validation reports no unconnected-input issue for that input

#### Report disconnected ordinary ports
- one or more declared ordinary inputs or ordinary outputs have no connection
- `Validate` reports a separate issue identifying each disconnected node and node-local port

#### Exclude typed error outputs
- validation analyzes an unconnected typed error output
- the error output contributes no connectivity issue, edge, degree, adjacency entry, cycle, or reachability path

#### Permit an unconnected error output
- one or more registered error outputs have no consumer
- validation reports no connectivity issue for those error ports

#### Recheck invalid stored edge configuration
- builder metadata contains an ordinary edge with invalid capacity, batch capacity, or policy
- the shared pure validator reports the applicable code and edge identity without invoking its queue factory

#### Recheck invalid stored error-edge configuration
- builder metadata contains an error edge with invalid capacity, batch capacity, or policy
- the shared pure validator reports the applicable code and edge identity without invoking its queue factory


### Deterministic DAG ordering and cycle witness

Validation SHALL use the report's forward adjacency snapshot to run Kahn's algorithm with the smallest available `NodeID` selected at each step and outgoing edges traversed by ascending destination `NodeID` then `EdgeID`. For an acyclic graph, the report SHALL contain all nodes exactly once in canonical order, and every edge SHALL lead from an earlier node to a later node.

If Kahn's algorithm leaves residual nodes, validation SHALL report a cycle issue containing one deterministic closed witness. Residual depth-first search SHALL start nodes in ascending `NodeID`, traverse outgoing residual edges by destination `NodeID` then `EdgeID`, and use the first back edge; every consecutive witness pair SHALL be an edge, the first and last node SHALL match, and a self-loop witness SHALL be `[N, N]`.

#### Compute canonical order for a DAG
- multiple nodes are simultaneously ready in a valid acyclic graph
- the validation report orders the smallest ready `NodeID` first and produces an order respected by every edge

#### Report a stable cycle witness
- Kahn's algorithm cannot consume every node
- `Validate` reports one cycle issue whose closed witness follows the deterministic residual DFS rules

#### Report a self-loop witness
- a node has a stored edge to itself
- the cycle issue carries that node twice as the complete witness


### Bidirectional source-to-sink reachability

A structural source SHALL be a node with zero declared ordinary inputs, and a structural sink SHALL be a node with zero declared ordinary outputs. Validation SHALL traverse the report's forward snapshot from every structural source and its reverse snapshot from every structural sink. Because the snapshot includes connected error edges, an error consumer reachable only through a potential failed result SHALL participate in reachability. It SHALL report an unreachable-from-source issue for each node not found by the forward traversal and an unable-to-reach-sink issue for each node not found by the reverse traversal, including when cycle issues also exist.

Source and sink classification SHALL use declared ordinary arity rather than current edge degree. Unconnected error outputs SHALL be absent from both traversals.

#### Reach an error-only consumer
- a producer's error port connects to a consumer whose only input is that error edge
- validation treats the consumer as reachable from the producer and includes it in source-to-sink analysis

#### Reject a component without a structural source
- a component's nodes are not reachable from any zero-input node
- validation reports an unreachable-from-source issue for every node in that component

#### Reject a component without a structural sink
- a component's nodes cannot reach any zero-output node
- validation reports an unable-to-reach-sink issue for every node in that component

#### Accept an isolated zero-arity node
- a statically complete node declares zero ordinary inputs and zero ordinary outputs and has no edges
- validation treats it as both source and sink and reports no connectivity, reachability, or cycle issue


### Validation does not mutate or seal the builder

Validation SHALL use only report-owned snapshots and local scratch state and SHALL NOT add, remove, or reorder nodes, ports, or edges; consume identities; cache topology results; open or seal the builder; or alter an existing sealed state. This SHALL hold after both successful and failed validation. Subsequent valid registration and connection operations on an open builder SHALL retain the same identity allocation behavior they would have had without validation.

#### Correct topology after failed validation
- validation fails and the caller adds valid missing topology
- the same builder remains mutable and can validate successfully after correction

#### Continue building after successful validation
- validation succeeds on an open builder and the caller performs another otherwise valid builder mutation
- the mutation is accepted and a later validation evaluates the updated topology

#### Preserve sealed state
- validation succeeds or fails on an already sealed builder
- it performs no topology mutation and the builder remains sealed by the preexisting build operation

#### Preserve identity allocation
- a caller validates before adding the next node or edge
- validation consumes no identity and the mutation receives the same identity it would have received without validation


### Validation robustness under generated topologies

Validation SHALL terminate without panic for nil, zero-value, empty, open, sealed, and every topology constructible through the builder API. For unchanged initialized topology it SHALL return deterministic reports and diagnostics, and whenever it reports no issue its canonical report order SHALL contain every node once and respect every ordinary edge.

#### Validate generated operation sequences
- fuzz or property tests generate arbitrary valid builder operation sequences and repeatedly validate the resulting topology
- validation terminates without panic and produces deterministic diagnostics or a complete edge-respecting canonical order


## immutable-graph-definition

Define transactional graph building and immutable reusable graph definitions.


### Transactional graph build

The graph package SHALL expose exactly `func (b *Builder) Build() (*Definition, error)`. A nil receiver or zero-value or otherwise uninitialized builder SHALL return `(nil, ErrInvalidBuilder)` without panic, owner allocation, or receiver mutation. Build on an initialized open builder SHALL consume the complete `ValidationReport` from `add-graph-dag-validation` before changing builder state. A report containing issues SHALL return `(nil, *ValidationError)`, SHALL NOT cache a partial definition or seal the builder, and SHALL leave valid subsequent registration and connection corrections available. An issue-free report SHALL produce and atomically cache one definition, seal all topology mutation, and return that definition with a nil error.

`ErrInvalidBuilder` and `ErrBuilderSealed` SHALL be the shared sentinels supplied by the `implement-typed-graph-builder` prerequisite; this capability SHALL NOT define duplicate sentinels. Registration and connection mutation after successful Build SHALL return `ErrBuilderSealed`. Build on that sealed builder SHALL return its cached definition and nil rather than `ErrBuilderSealed`.

#### Reject nil and zero builders
- Build is called through a nil builder pointer or a zero-value builder
- it returns a nil definition and `ErrInvalidBuilder` without panic or mutation

#### Correct topology after failed Build
- Build reports validation issues and the caller makes valid registration or connection corrections on the same builder
- the corrections are accepted and a later Build can validate the corrected topology

#### Seal mutation after successful Build
- Build succeeds and the caller attempts to register a node or connect an edge
- the mutation is rejected with the prerequisite's `ErrBuilderSealed` sentinel and the built definition is unchanged


### Idempotent successful definition

After the first successful Build, every subsequent Build call on that sealed builder SHALL return the same cached non-nil `*Definition` pointer and a nil error without rebuilding, revalidating, or changing topology metadata. Successful Build SHALL be the only sealing transition, and every sealed builder SHALL therefore have that cached pointer.

#### Repeat successful Build
- a caller invokes Build repeatedly after the first successful Build
- every call returns the identical cached definition and the same canonical topology


### Validation report is the build topology source

The `ValidationReport` consumed by Build SHALL contain sorted validation issues, canonical forward adjacency, canonical reverse adjacency, and canonical topological order. Each adjacency outer slice SHALL be dense by `NodeID-1`; each forward inner slice SHALL contain `EdgeID` values ordered by destination `NodeID` then `EdgeID`, and each reverse inner slice SHALL contain `EdgeID` values ordered by source `NodeID` then `EdgeID`. An issue-free report SHALL contain every node exactly once in canonical topological order. Build SHALL copy these report values and SHALL NOT recompute adjacency or topological order.

#### Consume canonical report topology
- Build receives an issue-free `ValidationReport` for a DAG with multiple valid traversal or topological orders
- the definition stores copies of the report's canonical forward adjacency, reverse adjacency, and topological order without deriving alternatives

#### Reject a report with issues transactionally
- the `ValidationReport` contains one or more sorted validation issues
- Build returns the corresponding `ValidationError`, retains no report topology in a definition, and leaves the builder open


### Opaque structurally immutable topology

`Definition` SHALL own copied dense node/edge metadata, canonical adjacency/order, selected queue plans, automatic constraints, and inactive NUMA annotations. It SHALL expose only NodeCount, EdgeCount, TopologicalOrder, EdgeQueuePlans, Constraints, GroupNUMA, and Start; no mutable runtime resource accessor is added. Every returned slice SHALL be an independent copy; nil and zero-value definitions return zero counts and nil snapshots. GroupNUMA rows SHALL be ordered by physical group identity and include logical group, replica index, and explicit-presence NUMA placement.

#### Mutate returned topology data
- a caller mutates TopologicalOrder, EdgeQueuePlans, Constraints, or GroupNUMA results
- subsequent inspection and execution retain the original definition metadata

#### Mutate returned topology and constraints
- a caller changes any rows returned by TopologicalOrder, EdgeQueuePlans, or Constraints
- later reads and execution preserve the original immutable definition plan

#### Mutate returned edge queue plans
- a caller changes a non-empty slice returned by `EdgeQueuePlans`
- later reads return the original model and physical cardinalities without altering Build or Start behavior

#### Inspect nil and zero definitions
- inspection methods are invoked on nil or zero-value definitions
- they return zero counts and nil slice snapshots

#### Preserve validated canonical order
- Build succeeds for a DAG with multiple valid topological orders
- the stored canonical order is the validator-provided order

#### Detach all metadata from builder storage
- a definition is built and the original builder placement/NUMA inputs are changed
- returned group and edge annotations remain the copied values

#### Preserve the validated canonical order
- Build succeeds for a DAG with more than one valid topological ordering
- the definition stores the validator's canonical topological order

#### Detach from builder storage
- a definition is built from topology held in mutable builder storage
- every slice-backed topology value is owned by the definition and cannot be changed through a builder alias


### Persistent graph identity

Every successfully built definition SHALL retain the exact non-zero private owner identity established for its builder by `NewBuilder`. The owner SHALL be immutable identity only and SHALL NOT reference mutable builder topology or lifecycle state. Build SHALL NOT substitute numeric node or edge identities or a newly allocated owner for that identity, and definition APIs SHALL NOT expose a way to read, construct, replace, mutate, or forge it. A nil receiver or zero-value definition SHALL have no valid owner and SHALL never be returned by successful Build.

#### Retain handle provenance after Build
- a future consumer checks a builder-created typed handle against the successfully built definition
- the handle remains attributable to that definition while a handle from another builder does not


### Runtime-neutral reusable definition

Build SHALL copy optional pool/group/queue NUMA annotations and typed pool/queue allocator descriptors as inactive metadata. Build and sequential Start SHALL never invoke those allocator descriptors, discover host NUMA topology, select local memory, change affinity, or alter portable allocation. Annotations SHALL be visible only through copied immutable GroupNUMA/EdgeQueuePlans and pool metric metadata. The definition SHALL still contain no factory-produced state, live allocator-owned storage, queue, pool, worker, affinity adapter/outcome, or lifecycle state. Factories, error resources, queues, and metrics remain dormant at Build and fresh at each Start.

#### Keep error factories and pools dormant during Build
- a valid graph with typed error configuration is built
- no error factory, pool, queue, batch, or error value is created

#### Keep factories and CPU plan dormant during Build
- a graph has observable factories and complete CPU/NUMA metadata
- Build invokes no callback and stores no runtime resources or affinity outcome

#### Reuse a CPU-bound multi-group definition sequentially
- a definition starts again after stop
- runtime resources are fresh while topology, CPU bindings, and NUMA annotations remain unchanged

#### Reuse a definition with error routing sequentially
- a stopped error-routed definition restarts
- fresh error pools and queue operations are created without changing copied metadata

#### Inspect during executor lifetime
- group NUMA metadata is inspected during a running executor
- only copied annotations are exposed, never mutable allocator/worker/queue resources

#### Inspect during executor lifetime
- a caller inspects a running definition
- accessors expose only copied logical topology and plan metadata

#### Keep factories dormant through finalization
- a complete preview is assigned CPUs and finalized
- no state, closure, or queue factory has run and the definition contains only immutable physical plans and dormant bindings

#### Reuse a finalized automatic definition
- its executor stops and the same automatic definition starts again
- fresh resources and affinity outcomes are created while logical topology and physical plan inspection remain unchanged

#### Keep error factories and pools dormant during Build
- a graph contains a typed error factory before automatic finalization
- Build invokes no error factory and stores no live pool or queue

#### Keep factories and CPU plan dormant during Build
- an automatic preview is built with dormant node factories
- no callback or runtime state is created before finalization and Start

#### Reuse a CPU-bound multi-group definition sequentially
- a CPU-bound physical definition restarts
- it creates fresh resources and affinity results without changing topology or bindings

#### Reuse a definition with error routing sequentially
- an automatic definition with typed error edges restarts
- error pools and queue operations are fresh

#### Keep factories and resource construction dormant during Build
- a valid graph has observable factories, lifecycle callbacks, queues, and constraints
- Build leaves callback counts at zero and stores only immutable plans, not runtime resources

#### Reuse fixed and replicated definitions sequentially
- an executor stops and its definition starts again
- state, queues, pools, workers, and affinity outcomes are fresh while topology and numeric previews remain unchanged

#### Reuse a node-pool definition sequentially
- a stopped node-pool executor starts again from the same definition
- each physical worker receives fresh state/resources and logical topology/worker plan storage remains unchanged

#### Inspect constraints during executor lifetime
- a caller reads Constraints during execution
- only an independent numeric snapshot is returned and no runtime resource is exposed

#### Inspect logical topology with physical expansion
- a caller reads topology accessors on a replicated definition
- logical counts/order remain unchanged, returned storage is independent, and physical expansion remains inaccessible except through copied read-only execution-plan/affinity/metrics identities


### Dormant copied logger configuration

Build SHALL retain copied optional logger configuration and its callback without invoking mappers or logging handlers, creating event rings, or starting a logger goroutine. Each successful sequential Start SHALL materialize fresh per-physical-worker rings and counters, and SHALL start one unpinned logger only after every data-node Init succeeds. Startup rollback before activation SHALL leave no logger goroutine running. These dormant callback/configuration fields extend the runtime-neutral definition contract without exposing mutable runtime resources.

#### Fail initialization before logger activation
- a configured definition's node Init fails during Start
- its owned lifecycle rolls back and no logger callback or running logger is created

#### Restart a configured definition
- one executor stops and the same configured definition starts again
- logging uses fresh empty event transport and counters without changing the stored configuration


## node-factory-contracts

Define typed node state and closure factory contracts, including replica lifecycle and factory dormancy requirements.


### Generated handler contracts are a prerequisite

The node factory contracts SHALL depend on the explicit-state and closure handler-set families supplied by `generate-typed-node-arities`. `StateFactory[S]`, `InitFunc[S]`, `CloseFunc[S]`, `Stateful[S, H]`, `ClosureReplica[H]`, and `ClosureFactory[H]` SHALL compose with those generated types without adding graph builders, ports, registration, connections, or topology behavior.

#### Compose factories with generated handlers
- a developer instantiates explicit-state and closure factory declarations with generated handler sets
- the declarations compile with the exact generated handler types as `H` and require no graph API


### Exact explicit state factory declarations

The `node` package SHALL expose `StateFactory[S]` with underlying function signature `func() (S, error)`, `InitFunc[S]` with underlying function signature `func(*S) error`, and `CloseFunc[S]` with underlying function signature `func(*S) error`. It SHALL expose `Stateful[S, H]` as a struct with `Factory StateFactory[S]`, `Handlers H`, `Init InitFunc[S]`, and `Close CloseFunc[S]` fields. `Factory` is required while `Init` and `Close` are optional. A future runtime SHALL store each successfully returned `S` as one replica's state and SHALL pass its address only to that replica's explicit handlers and lifecycle hooks.

#### Declare explicit replica state
- a developer constructs `Stateful[S, H]` with a state factory and a generated explicit-state handler set
- the factory result, handlers, and optional lifecycle hooks remain statically typed to `S` and `H`

#### Assign exact explicit callback types
- a developer assigns functions matching `func() (S, error)` and `func(*S) error` to the named explicit callback types
- those values populate the corresponding `Stateful[S, H]` fields without adapters or runtime type assertions

#### Omit explicit lifecycle hooks
- `Init` and `Close` are nil on a valid `Stateful[S, H]` declaration
- the declaration remains valid and startup and shutdown require no lifecycle callback invocation


### Exact closure replica factory declarations

The `node` package SHALL expose `ClosureReplica[H]` as a struct with `Handlers H`, `Init func() error`, and `Close func() error` fields. It SHALL expose `ClosureFactory[H]` with underlying function signature `func() (ClosureReplica[H], error)` so one successful call produces a coherent handler set and lifecycle callbacks sharing only that replica's closure environment. `Init` and `Close` are optional.

#### Create a closure-bound replica
- a `ClosureFactory[H]` successfully creates handlers and lifecycle callbacks that capture one mutable environment
- all returned functions belong to the same typed `ClosureReplica[H]`

#### Omit closure lifecycle callbacks
- a closure factory returns a replica with nil `Init` and `Close`
- the replica remains valid and startup and shutdown require no lifecycle callback invocation


### Fresh factory result per replica and start

Each successful explicit or closure factory invocation SHALL create state owned by exactly one physical node instance, including each fixed node-worker-pool worker and each whole-group replica. Build SHALL remain dormant. Start SHALL invoke a factory once per physical instance, and successful lifecycle ownership SHALL be initialized/closed exactly once on that instance's locked worker, including rollback. Framework-created mutable state and closure environments SHALL not be shared between workers; deliberately shared user captures remain caller-owned and require user synchronization.

#### Mutate separate node worker states
- one logical node starts with three workers whose factories create mutable maps
- each worker's handler mutates only the map produced for its own physical instance

#### Observe independent mutable state
- a logical node is expanded to multiple group replicas and each factory creates a mutable map
- each physical handler observes and mutates only its own map unless application code deliberately shares a captured map

#### Start multiple replicas
- one node is started with multiple logical replicas
- the runtime calls its factory once per replica and each successful result has independent mutable state

#### Restart a node
- a previously stopped node starts again with the same declaration
- the runtime calls its factory again and does not reuse mutable state from the prior start


### Lifecycle cleanup follows successful construction

A factory result SHALL transfer lifecycle ownership to the future runtime only when the factory returns a nil error. For each successfully returned lifecycle, the runtime SHALL invoke optional `Init` at most once and SHALL invoke a provided `Close` exactly once on normal shutdown or startup rollback. A failed `Init` SHALL count as partial initialization and SHALL cause `Close` for that lifecycle and every other successfully created lifecycle included in the rollback. A failed factory return SHALL NOT cause `Init` or `Close` from that result to be invoked.

#### Close after normal operation
- a factory succeeds, optional initialization succeeds, and the replica later stops normally
- the runtime invokes its provided `Close` exactly once

#### Roll back partial initialization
- a factory succeeds but that replica's `Init` returns an error after partially acquiring resources
- startup fails and the runtime invokes the replica's provided `Close` exactly once as part of rollback

#### Roll back previously created replicas
- startup fails after other replica factories have already returned successfully
- the runtime invokes each provided `Close` exactly once for every successfully created lifecycle included in the rollback

#### Factory fails before ownership transfer
- an explicit or closure factory returns a non-nil error
- the runtime does not invoke `Init` or `Close` from that failed result


### Captured configuration remains immutable

Configuration captured by state factories, closure factories, handlers, or lifecycle callbacks SHALL be treated as immutable after the declaration is handed to the framework and for as long as a graph build or runtime start can use it. Mutable per-replica data SHALL be created by each factory call and SHALL reside only in that call's explicit state result or closure environment. The framework SHALL NOT claim to deep-copy or freeze caller-owned captures.

#### Reuse immutable configuration
- multiple replica factories read the same captured immutable configuration
- they may share that configuration while placing all runtime mutation in their independent factory results

#### Avoid shared mutable captures
- a developer needs mutable counters, buffers, maps, or slices for handler execution
- the developer creates them independently inside each factory result rather than mutating captured configuration


### Deferred graph integration preserves factory dormancy

The node factory contracts SHALL require the future graph bridge to record static validation facts without invoking factories. Future registration SHALL record whether each required factory is nil and SHALL record statically visible explicit handler facts; for closure declarations it SHALL NOT invoke the closure factory to inspect returned handlers or lifecycle callbacks. Future graph Build SHALL consume those recorded facts without invoking `Stateful.Factory` or `ClosureFactory`. Implementation and executable tests for this bridge belong to the next graph change, not this capability.

#### Record explicit static facts in a future bridge
- future graph registration receives a node backed by `Stateful[S, H]`
- it records factory presence and statically visible handler facts without calling the state factory

#### Record closure factory presence in a future bridge
- future graph registration receives a node backed by `ClosureFactory[H]`
- it records whether the factory is nil without calling it to inspect returned handlers or lifecycle callbacks

#### Preserve dormancy through future Build
- future graph Build validates the static facts recorded by registration
- it reports any missing required factory without invoking any node factory


## typed-node-output-ownership

Define typed handler output acquisition, commit, and automatic ownership cleanup.


### Typed output acquisition and commit

The node package SHALL expose `type Output[T any]` with exactly `func (o *Output[T]) Acquire() (batch.Batch[T], bool)` and `func (o *Output[T]) Commit(*batch.Batch[T]) bool`. Runtime-created output values SHALL be the only usable values. Acquire SHALL return at most one owned batch for an output during one handler invocation and SHALL return an empty batch and false when its preallocated source pool is exhausted or the output already has an acquired or committed batch. Commit SHALL accept exactly one valid `Batch[T]`, move and clear the caller handle, and stage it for executor publication; it SHALL return false without taking ownership for a nil receiver, empty batch, or second commit. `Outputs1` through `Outputs8` SHALL contain exported `Out1` through `Out8` fields of the corresponding `Output` types, while `Outputs0` SHALL remain empty.

#### Acquire and publish a transformed output
- a handler acquires a batch from `outputs.Out1`, fills it, and commits it
- Commit clears the local handle and the executor can publish that typed batch on output one after the handler returns

#### Move a compatible input to an output
- a handler passes its `batch.Batch[int]` input to `outputs.Out1.Commit`
- the input handle is cleared and the staged output retains ownership without a conversion or runtime type assertion


### Automatic output cleanup boundary

For each generated handler invocation on an explicit-state or closure physical worker, an output acquisition SHALL be released at finalization only if its handler-owned by-value handle was neither moved, explicitly released, nor committed. A successful output Commit SHALL transfer exactly the committed batch; if a different compatible input is committed to the output, the original scratch acquisition SHALL be reclaimed independently. Explicit release/move state SHALL be invocation-local, detached before cross-worker publication, and race-free across node workers. No allocation or reflective type lookup SHALL be required. Drop, Wait, failed result, forced Close, and rollback SHALL release each owned slot exactly once.

#### Release explicitly returned output
- a node-pool worker explicitly releases an acquired output before returning
- finalization does not release the retained copy or corrupt a subsequent owner of the same slot

#### Move input instead of scratch output
- a node-pool handler acquires scratch output storage and commits its compatible input instead
- scratch storage is reclaimed separately and the input remains owned by its queue path until delivery or discard

#### Abandon an acquired output
- a handler acquires an output batch and returns without committing it
- the executor returns that batch to its original pool and later acquisition can reuse it

#### Reject a duplicate output commit
- a handler commits one batch to an output and attempts a second Commit in the same invocation
- the second call returns false and leaves ownership of its argument with the handler

#### Respect an explicitly released acquisition
- an acquired output copy is released and the slot is acquired by a new owner before finalization
- cleanup leaves the new owner's slot and utilization count unchanged

#### Commit input instead of scratch acquisition
- a handler acquires scratch output storage but commits its compatible input to that output
- finalization releases the unused scratch acquisition while preserving the committed input until publication or discard


### Runtime-owned typed error transport batch

A connected `ErrorPort[E]` SHALL be backed at runtime by a framework-owned typed source pool. Only the generated executor binding may acquire its `Batch[E]`; user handlers receive it only through the target `InPort[E]`. For each failed result, the binding SHALL acquire at most one batch, populate exactly one element, and either transfer it to the edge queue, retain it as the runner's one Wait pending output, or release it. It SHALL not expose an error `node.Output[E]` to a handler.

#### Preserve origin ownership through an error consumer
- a connected error batch crosses a local or remote edge and the consumer leaves it un-moved
- executor cleanup returns it to the error edge's original typed pool without a runtime pool lookup or type erasure

#### Discard a pending error batch on close
- Close observes a runner holding a Wait-pending typed error batch
- the executor releases that batch exactly once and clears the pending handle without invoking another handler


### Invocation-local input ownership cleanup

For every explicit-state and closure input handler at arities one through eight, the executor SHALL release an input at invocation completion only if the handler has neither moved it nor explicitly released it. A move from the handler's by-value input copy SHALL relinquish automatic input cleanup even though the runner retains a separate handle copy. Untouched inputs SHALL be returned exactly once to their originating pool. Explicit Batch.Release or Pool.Release of the handler input SHALL relinquish automatic cleanup. This bookkeeping SHALL not introduce reflection, runtime type assertions, boxed batch values, or steady-state heap allocation, and SHALL not change existing handler signatures.

#### Preserve a forwarded input
- an input handler commits its three-element input to a compatible output and returns success
- subsequent runner cleanup does not release the forwarded slot, and the downstream consumer observes the original length and items

#### Release an untouched input
- an input handler returns without moving or explicitly releasing its input
- invocation cleanup returns the batch to its original pool exactly once

#### Respect explicit release
- an input handler explicitly releases its input through Batch.Release or Pool.Release before returning
- invocation cleanup clears its retained handle without releasing or resetting that slot again


### Accepted and rejected input commits preserve ownership boundaries

A successful compatible input Commit SHALL clear the handler's argument and transfer ownership to output finalization. A rejected Commit SHALL leave the argument and invocation ownership unchanged. Committed input batches SHALL remain unavailable for source-pool reacquisition while queued, pending Wait publication, or owned downstream. A failed handler result, queue Drop, or forced Close SHALL release a committed input through its output ownership path exactly once, and input cleanup SHALL not release it again.

#### Reject a duplicate commit without consuming input
- a handler attempts to commit its input to an output that has already committed another batch
- Commit returns false, leaves the input owned and unchanged, and normal cleanup releases it unless the handler subsequently moves or releases it

#### Retain an input across Wait
- a committed input batch cannot enter a full Wait queue and the handler returns
- the output retains the original batch length and contents, the source pool cannot reacquire its slot, and input cleanup does not release it

#### Discard a committed input on failed result
- a handler commits its input to an ordinary output and then returns a failed Result
- failed-output cleanup releases the original slot once and automatic input cleanup does not release it again


### Ownership transfer is isolated across invocations and workers

Transfer bookkeeping SHALL be scoped to the invoking worker and handler call. A moved batch SHALL not carry a mutable reference to the producing invocation's bookkeeping when it crosses a queue or becomes independently owned. Reusing invocation bookkeeping SHALL not affect batches previously transferred. Local, custom-local, and remote forwarding, including a typed error input moved to an ordinary output, SHALL preserve originating-pool ownership and exact logical contents across multiple hops and sequential executor starts.

#### Forward across groups repeatedly
- successive invocations move typed inputs through a multi-hop graph with a remote boundary
- each downstream batch retains its own contents and length, no prior transferred slot is released by a later invocation, and execution remains race-free

#### Forward a typed error input
- an error consumer moves its one-element error batch to a compatible ordinary output
- downstream receives the original failure value and eventually releases the batch to its error-edge source pool
