# Typed data and graph

This contract covers typed storage, node declarations, graph construction, and
immutable definitions. Execution, transport, instrumentation, and planning are
specified in the [topic index](README.md). **Must**, **must not**, and **may** are
normative; examples illustrate rules rather than add separate obligations.

## Typed boundary

The module must require Go 1.22 or newer. Public `batch`, `node`, `graph`, `queue`,
and runtime configuration contracts must compile independently of a running
graph; batch pools must be usable without an executor. Data-plane values must
remain concrete generic types, without reflection, runtime type assertions, or
`any`/`interface{}` value registries. Control-plane graph algorithms are not
subject to a blanket ban on interfaces.

Dependencies must remain acyclic: `graph` consumes `node`; `node` must not import
`graph`. Graph handles, registration results, and topology APIs belong to `graph`.

## Batches and pools

`NewPool[T](PoolConfig) (*Pool[T], error)` must preallocate every slot and its typed
fixed-capacity storage. `PoolConfig` has `Capacity uint32`, `BatchCapacity uint32`,
`ClearOnRelease bool`, `NUMA numa.Placement`, and `OwnerID uint32`. Construction
must reject invalid NUMA representation, zero capacities, and size arithmetic
that cannot fit Go slice lengths; no usable pool may escape on failure. Pools
must never grow or allocate replacement storage after construction.

| Operation | Required behavior |
| --- | --- |
| `(*Pool[T]).Acquire() (Batch[T], bool)` | Return exclusive ownership of one available slot; exhaustion returns an empty handle and false without allocation. |
| `Batch[T].Mutable() []T` | Expose the full fixed-capacity writable storage. Fill by index; do not depend on capacity-growing append. |
| `Batch[T].Items() []T` | Expose only the explicit logical length, with no spare slice capacity. |
| `(*Batch[T]).SetLen(int) bool` | Accept zero through `Cap()`; reject other lengths without changing the current length or allocating. |
| `(*Batch[T]).Move() Batch[T]` | Transfer ownership and clear the source handle. |
| `(*Batch[T]).Release()` | Return the slot to its original allocating pool, including after compatible multi-hop forwarding, without type erasure or runtime pool lookup; clear the handle. |
| `(*Pool[T]).Release(*Batch[T])` | Release an owned slot to that pool and clear the caller handle; empty-handle release is a no-op. |

Acquire/fill/SetLen/release and exhaustion reporting must allocate no framework
heap memory after setup. Slot ownership must support producer acquisition and
remote consumer release; the obsolete single-goroutine-only pool restriction
does not apply to that framework handoff. This is not a blanket guarantee for
arbitrary concurrent public-pool use or shared batch access. Applications must not access storage after transfer or
release, release through the wrong pool, or duplicate ownership by copying a
handle. Arbitrary handle misuse need not be dynamically detected.

Release must reset logical length. With `ClearOnRelease`, it must zero the
**entire capacity** before reuse, not just the former logical items. Without it,
old values remain until overwritten and may retain pointers. NUMA and typed
allocator descriptors are [inactive metadata](05-planning-performance.md#numa-metadata-and-inactive-allocators).

## Node shapes and factories

### Generated type families

| Family | Contract for arity N = 0…8 |
| --- | --- |
| `node.OutputsN` | Sealed `node.OutputSet`; `OutputCount() uint8` returns N. `Outputs0` is empty; other shapes have heterogeneous `Out1`…`OutN` fields of type `node.Output[T]`. External arbitrary output shapes must not satisfy the seal. |
| `node.NodeN` | Explicit-state handlers. N=0 has `Handle SourceHandler[S,O,E]`; N>0 has a separate `Handlei InputHandler[S,Ii,O,E]` for each input. |
| `node.ClosureNodeN` | Equivalent per-input types with closure-captured state rather than a `*S` parameter. |
| Selectors | N>1 exposes `Selector[S]` or `ClosureSelector`; `ReadyMask uint8` covers eight positions and `PortIndex uint8` constants `Port1`…`Port8` are **zero-based**. |

The exact explicit signatures are:

```go
type Result[E any] struct { Value E; Failed bool }
type SourceHandler[S any, O OutputSet, E any] func(*S, *O) Result[E]
type InputHandler[S, I any, O OutputSet, E any] func(*S, batch.Batch[I], *O) Result[E]
type Selector[S any] func(*S, ReadyMask) PortIndex
type ClosureSelector func(ReadyMask) PortIndex
```

Every handler returns concrete `Result[E]`. E is a distinct optional graph error
output, not a ninth ordinary `node.Output` and not a boxed Go error. All eight
ordinary outputs remain available when error routing is configured. Handler
sets also permit typed `MapError` and `BindCounter` callbacks, specified under
[observability](04-errors-observability.md).

### Factory declarations and ownership

```go
type StateFactory[S any] func() (S, error)
type InitFunc[S any] func(*S) error
type CloseFunc[S any] func(*S) error
type Stateful[S, H any] struct {
    Factory StateFactory[S]
    Handlers H
    Init InitFunc[S]
    Close CloseFunc[S]
}
type ClosureReplica[H any] struct {
    Handlers H
    Init func() error
    Close func() error
}
type ClosureFactory[H any] func() (ClosureReplica[H], error)
```

Factories are required; Init and Close are optional. Each Start must invoke the
appropriate factory once per **physical node instance**, including whole-group
replicas and node-pool workers. A successful factory transfers lifecycle ownership;
a failed return transfers none and must not have its Init/Close invoked. A
successful result gets at most one Init and exactly one provided Close, including
rollback after failed or partial Init. Explicit handlers/hooks receive only their
instance's state address; closure handlers/hooks share that instance's coherent
environment. See [startup and rollback](02-execution-lifecycle.md#startup-and-rollback)
for ordering and worker ownership.

Mutable per-instance maps, buffers, and counters must be created in each factory
result, not reused across starts or workers. Caller-owned captured configuration
must remain immutable while builds or starts can use it. The framework does not
deep-copy or freeze captures; deliberately shared mutable captures require caller
synchronization.

## Invocation ownership

Only runtime-installed `node.Output[T]` values are usable. Their public handler
operations are exactly `Acquire() (batch.Batch[T], bool)` and
`Commit(*batch.Batch[T]) bool` on pointer receivers.

| Boundary | Ownership rule |
| --- | --- |
| Output acquisition | At most one acquisition per output per invocation. Exhausted pool, an outstanding acquisition, or an existing commit returns empty/false. |
| Output commit | Accept one non-empty **handle** of matching T, move it, clear the argument, and stage publication until the handler returns. A logical length of zero is not an invalid handle. |
| Rejected commit | Nil/uninstalled output, nil/empty batch handle, or second commit returns false without consuming the argument or changing input cleanup ownership. |
| Untouched input | Executor returns it to its origin pool exactly once after the handler returns. |
| Moved or explicitly released input | Moving/releasing the handler's by-value copy relinquishes automatic cleanup; the retained runner copy must not release or reset a slot now owned elsewhere. |
| Abandoned output acquisition | Finalization releases it only if the handler neither moved, released, nor committed it. If a different compatible input was committed instead, reclaim the scratch acquisition independently. |
| Successful result | Publish committed ordinary outputs; release untouched inputs and unused acquisitions. |
| Failed result | Publish no ordinary output. Release acquired/committed ordinary work and untouched inputs; handle E only through [failure routing and mapping](04-errors-observability.md#failure-routing-and-mapping). |

Queued, Wait-pending, and downstream-owned moved batches remain unavailable to
their source pool. Drop, failed result, forced Close, and rollback must release
each owned slot exactly once through its current ownership path. A typed error
consumer may forward its input to an ordinary output of matching E; eventual
release still returns to the error source pool.

Transfer bookkeeping must be invocation-local, allocation-free, and isolated per
worker. Independently moved batches must not carry mutable references to a prior
invocation's bookkeeping. Reusing that bookkeeping must not affect earlier
transfers. These rules apply across local, custom-local, remote, multi-hop, and
sequential-start paths without changing handler signatures.

## Builder identity and registration

`NewBuilder() *Builder` must return an open builder with initialized registration
state and a fresh private nonzero-sized owner object. Nil/zero builders are
invalid; builders must not be copied. Invalid, open, and sealed are distinct
states. Rejected registration must not mutate metadata or consume an ID;
`ErrInvalidBuilder`, `ErrBuilderSealed`, and `ErrNodeIDExhausted` distinguish those
failures in that order. Node identity exhaustion must reject rather than wrap.

| Identity / handle | Exact representation or accessors |
| --- | --- |
| Nodes | `type NodeID uint32`; `InvalidNodeID = 0`; dense stable successful registrations start at 1. |
| Ports | `type PortID uint8`; `InvalidPortID = 0`; separate node-local input/output namespaces. Ordinary positions are 1…8; error output is 9. |
| Edges | `type EdgeID uint32`; `InvalidEdgeID = 0`; one dense stable commit-order sequence shared by ordinary and error connections. |
| `InPort[T]`, `OutPort[T]`, `ErrorPort[E]` | Opaque owner/node/port; value accessors `NodeID() NodeID`, `PortID() PortID`, `Valid() bool`. ErrorPort is not assignable to OutPort. |
| `OutputRef[O node.OutputSet]` | Opaque owner/node/count; value accessors `NodeID() NodeID`, `OutputCount() uint8`, `Valid() bool`; no singular PortID. |
| `NodePorts[I, O, E]` | O constrained to `node.OutputSet`; value accessors `NodeID() NodeID`, `Inputs() I`, `Outputs() OutputRef[O]`, `Errors() ErrorPort[E]`. All refer to one owner/node. |

Zero handles/results must expose invalid components without panic. Singular
validity requires an owner and nonzero node/port; ErrorPort additionally requires
port 9. OutputRef validity requires an owner, nonzero node, and count 0…8. External
callers must have no way to construct/replace owners or forge private identities.

Generated graph families must be committed as `InPorts0`…`InPorts8` (`In1`…`InN`),
`OutPorts0`…`OutPorts8` (`Out1`…`OutN`), and `OutputPorts0`…`OutputPorts8`.
`OutputPortsN` accepts only `OutputRef[node.OutputsN[…]]`, preserves owner/node,
and derives ports 1…N; invalid references or stored-count mismatch return the
zero invalid bundle. Registration derives output count by calling the zero O's
`OutputCount`, without reflection or assertions.

`AddNode0`…`AddNode8` accept a builder and matching
`node.Stateful[S,node.NodeN[…]]`; `AddClosureNode0`…`AddClosureNode8` accept a builder
and matching `node.ClosureFactory[node.ClosureNodeN[…]]`. Each returns
`(NodePorts[InPortsN[…],O,E], error)`; N=0 omits input type parameters. Output
arity is independent of input arity: generate linear families, not an input/output
cross-product. Successful registration creates error port 9 and atomically stores
kind, node ID, arities, static descriptor, and dormant typed normalizer.

The private bridge is `type nodeNormalizer func(*normalizationTarget) error`;
concrete registration closures must retain monomorphized factory/handler bindings
without dynamically typed callback storage. Registration invokes no factory,
handler, selector, Init, Close, mapper, or counter binder.

### Static validation and generation

Registration must record static required/present facts without invoking user code;
the private mask layout is not a public contract. Explicit declarations require
factory, all applicable handlers, and a
selector only for N>1. Closure registration requires only the observable factory;
produced handlers/selectors are checked during Start, not by executing the factory
at validation time. Each missing statically observable factory, handler, or
required selector must produce its own entity-specific diagnostic. Source-handler
diagnostics use invalid PortID; input-handler diagnostics identify their one-based
input port.

The shared standard-library-only arity generator must own a graph-specific
manifest, generated headers, and byte-for-byte temporary regeneration checks for
every listed graph file. Missing, changed, or unexpected generated graph files
must fail independently of node-generation checks. Compile coverage must exercise
all supported arities, heterogeneous inputs/outputs, the sealed output set, and
compile-time ordinary/error type mismatches.

## Connections and local error precedence

```go
func Connect[T any](*Builder, OutPort[T], InPort[T], EdgeConfig[T]) (EdgeID, error)
func ConnectError[E any](*Builder, ErrorPort[E], InPort[E], EdgeConfig[E]) (EdgeID, error)
type EdgeConfig[T any] struct {
    Capacity uint32
    BatchCapacity uint32
    Policy queue.Policy
    Factory queue.Factory[T]
    NUMA numa.Placement
    Allocators AllocatorDescriptors[T]
}
type AllocatorDescriptors[T any] struct {
    Pool batch.Allocator[T]
    Queue queue.Allocator[T]
}
```

Matching generic types are required at compile time; ordinary Connect cannot
accept ErrorPort. Both APIs must validate entirely before committing one edge,
occupying endpoints, or retaining a future binding. Every failure returns edge
zero and leaves topology, IDs, ports, and the rejected factory untouched.

Select the first applicable error in this order for either API:

1. Invalid builder (`ErrInvalidBuilder`), then sealed builder (`ErrBuilderSealed`).
2. Invalid output, then input handle; unknown same-owner endpoint metadata
   (output then input) is also `ErrInvalidPort`.
3. Foreign output, then input (`ErrForeignPort`).
4. Already-produced input (`ErrInputConnected`).
5. Already-consumed output (`ErrFanOutUnsupported`).
6. Invalid edge configuration (`*EdgeConfigError`).

Each ordinary/error input has one producer and each ordinary/error output one
consumer under these APIs. [Explicit topologies](03-topologies-transport.md)
provide exceptions without weakening ordinary Connect or adding implicit fan-out.

One pure `validateEdgeConfig(queue.Config) error` must be shared by connection
and graph validation. It accepts only positive Capacity/BatchCapacity, `queue.Drop`
or `queue.Wait`, and valid NUMA representation. `queue.Config` carries those fields
plus NUMA. Capacity counts queued batch-handle slots; BatchCapacity counts elements
in each newly allocated source slot, not total in-flight pipeline elements.
`EdgeConfigError` exposes `Config queue.Config`, `Error() string`, and
`Unwrap() error` returning `ErrInvalidEdgeConfig`; categories must be matchable by
`errors.Is`, configuration details by `errors.As`.

`type queue.Factory[T any] func(queue.Config) (queue.QueueOps[T], error)` must
remain dormant until Start, as must its retained monomorphized edge binding.
Nil chooses built-in transport. Only physically local single-owner edges may
use custom factories; [queue selection](03-topologies-transport.md#queue-models-and-selection)
defines remote/shared rejection and fresh per-start construction.

## Validation and diagnostics

`(*Builder).Validate() (*ValidationReport, error)` must analyze open **or sealed**
builders without mutation, identity consumption, caching, sealing, reordering, or
user callbacks. Nil/zero builders return nil report and `*ValidationError` with
exactly `ValidationCodeInvalidBuilder`; initialized empty builders return a
non-nil empty report and exactly `ValidationCodeEmptyGraph`. Other initialized
builders return a report and nil error iff it has no issues. Report/error issues
must agree but own independent storage.

| Analysis | Required result |
| --- | --- |
| Static contracts | Use stored masks, not reflective rediscovery; closure-produced callbacks remain uninspected. |
| Connectivity | Every ordinary port must have its required connection, recognizing explicit fan-in/broadcast/shard metadata. Error-fed inputs count; unconnected error outputs are optional and contribute no edges or paths. Zero-input/zero-output nodes are allowed. |
| Edge configuration | Reuse the connection validator; identify invalid capacity/batch capacity or policy by EdgeID, without invoking factories. |
| Placement/CPU | Validate malformed placement, remote custom factories, and missing/negative/duplicate/ineffective group CPU bindings; see [manual placement](02-execution-lifecycle.md#manual-placement-and-cpu-bindings). |
| Adjacency | One row per node in ascending NodeID, including empty rows; forward destinations/reverse sources sorted by AdjacentNodeID then EdgeID. Include ordinary and connected error edges. |
| DAG order | Kahn with smallest available NodeID first. Every acyclic nonempty graph gets the complete order even if other issues exist. Cyclic graphs get **no partial prefix**. |
| Cycle | One deterministic closed witness: ascending residual DFS roots, outgoing residual edges by destination then EdgeID, first back edge. Each pair must be an edge; self-loop is `[N,N]`. |
| Reachability | Traverse forward from zero-declared-input sources and reverse from zero-declared-output sinks, not current degree. Report each missing node in both directions even if cycles also exist. Connected error paths count. An isolated statically complete zero/zero node is both source and sink. |

Report accessors are `Issues() []ValidationIssue`, `TopologicalOrder() []NodeID`,
`ForwardAdjacency() []ValidationAdjacency`, and
`ReverseAdjacency() []ValidationAdjacency`. `ValidationAdjacency` has `NodeID` and
`Edges []ValidationAdjacentEdge`; each edge has `EdgeID` and `AdjacentNodeID`.
All accessors must deep-copy nested storage, including cycle witnesses, so neither
caller edits nor subsequent builder edits change a report or error text.

`ValidationCode` is `uint8`. `ValidationIssue` has `Code ValidationCode`, `NodeID NodeID`, `PortID PortID`,
`EdgeID EdgeID`, `GroupID GroupID`, `CPU int`, and `Cycle []NodeID`; unused entity
fields are zero. `ValidationError.Issues()` deep-copies and is discoverable with
`errors.As`. Aggregate every independently discoverable issue, sorting by code,
NodeID, PortID, EdgeID, GroupID, CPU, then lexicographic cycle. Fixed code order
after reserved non-emitted `ValidationCodeInvalid` is:

| Order | `ValidationCode` suffixes |
| --- | --- |
| 1–5 | `InvalidBuilder`, `EmptyGraph`, `MissingFactory`, `MissingHandler`, `MissingSelector` |
| 6–10 | `UnconnectedInput`, `UnconnectedOutput`, `InvalidEdgeCapacity`, `InvalidEdgePolicy`, `Cycle` |
| 11–14 | `UnreachableFromSource`, `CannotReachSink`, `InvalidGroupPlacement`, `RemoteQueueFactory` |
| 15–18 | `MissingGroupCPU`, `InvalidGroupCPU`, `DuplicateGroupCPU`, `IneffectiveGroupCPU` |

Repeated validation must preserve structured diagnostics and text. Property/fuzz
tests must cover nil, zero, empty, open, sealed, and API-constructible topologies;
validation must terminate without panic and issue-free orders must include every
node once and respect every connected edge.

## Build and immutable definitions

`(*Builder).Build() (*Definition, error)` is transactional. Invalid builders
return `(nil, ErrInvalidBuilder)` without mutation or owner allocation. An open
builder must consume its complete validation report, copy its canonical adjacency
and order rather than recomputing them, compile placement/physical expansion and
constraints, and publish nothing if any step fails. Failed Build leaves the
builder correctable. Successful Build atomically caches a definition and seals
mutation; later Build returns the **same pointer** without revalidation.
[Explicit automatic finalization](05-planning-performance.md#cpu-assignment-and-finalization)
is the other successful publication/sealing path; preview alone is not.

The definition must retain the builder's exact private owner as immutable identity
only, without mutable builder topology/lifecycle references. It owns copied dense
node/edge metadata, report adjacency/order (dense by NodeID−1), physical queue/group
plans, CPU assignments, constraints, NUMA annotations, and dormant typed bindings
and optional instrumentation configuration. No live factory result, batch, pool,
queue, worker, logger, affinity outcome, or mutable metrics belong to Build.

`NodeCount() int`, `EdgeCount() int`, and `TopologicalOrder() []NodeID` describe
**logical** topology even when execution expands it. `EdgeQueuePlans()`,
`Constraints()`, and `GroupNUMA()` return copied physical-plan metadata with
applicable logical/replica identities, not mutable resources. GroupNUMA rows are
physical-group ordered `GroupNUMAMetadata{GroupID, Replica, Placement}`; placement
presence distinguishes explicit NUMA zero. Nil/zero definitions return zero
counts and nil/empty snapshots, and [Start](02-execution-lifecycle.md) rejects
them. `WriteDOT(io.Writer) error` exports deterministic logical topology, not a
live executor or physical placement view.

Inspection must not invoke callbacks or mutate execution. Each sequential Start
after completed stop must create fresh runtime state while definition topology,
CPU/queue plans, and configuration remain unchanged. Configuration is copied,
but callback captures remain caller-owned under the immutable-capture rule.
