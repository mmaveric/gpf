# Execution and lifecycle

An immutable [definition](01-typed-graph.md#build-and-immutable-definitions) starts
fresh fixed-plan resources. Logical nodes may expand into
[replicas or node workers](03-topologies-transport.md#physical-parallelism), but
placement and CPU choices are settled before execution, not discovered by Start.

## Manual placement and CPU bindings

The exact control-plane APIs are:

```go
func Place(builder *Builder, node NodeID, group GroupID) error
func BindCPU(builder *Builder, group GroupID, cpu int) error
```

`GroupID` is `uint32`; `InvalidGroupID = 0`, `DefaultGroupID = 1`. Group IDs are
opaque sparse positive identities, not CPU numbers, replica counts, or NUMA nodes.
Every registered node without explicit Place has effective DefaultGroupID. Place
records a node's intended group once; there is no reassignment/removal operation.
NodeID is builder-local; unlike port handles, numeric IDs carry no owner token.

Select the first applicable stable sentinel, with no mutation on failure:

| API | Error precedence |
| --- | --- |
| Place | `ErrInvalidBuilder`, `ErrBuilderSealed`, `ErrInvalidGroupID`, `ErrUnknownNode`, `ErrNodeAlreadyPlaced` |
| BindCPU | `ErrInvalidBuilder`, `ErrBuilderSealed`, `ErrInvalidGroupID`, `ErrInvalidCPU` (negative), `ErrUnknownGroup`, `ErrGroupCPUAssigned`, `ErrCPUAlreadyBound` |

A group is effective if at least one registered node has that effective placement.
Each effective manual group, including DefaultGroupID, must have exactly one
explicit nonnegative CPU binding before Build. CPUs must be unique across
effective groups; bindings to ineffective groups are invalid. Neither operation
may invoke callbacks, consume graph IDs, change connections, or create resources.
No host CPU upper bound or allowed-cpuset policy is enforced at Build.

Validate must report malformed placement by NodeID, remote ordinary/error custom
factories by EdgeID, and missing/invalid/duplicate/ineffective CPU binding by
GroupID and applicable CPU, using the [ordered diagnostics](01-typed-graph.md#validation-and-diagnostics).
Build failure leaves the builder open and factories/affinity untouched. Successful
manual Build must resolve defaults, order effective groups by GroupID and each
group's nodes by canonical topology, retain supplied CPUs, and classify endpoint
locality without merging/renumbering groups or inventing unused ones.

## Startup and rollback

`func (d *Definition) Start() (*Executor, error)` takes **no runtime configuration**.
Nil, zero, or malformed definitions must return `(nil, ErrInvalidDefinition)`
before callbacks, runtime resources, thread locking, or affinity. A definition
retains no executor pointer or lifecycle state. Sequential reuse after completed
stop is supported; concurrent Starts from one definition are unsupported.

Successful startup must follow this order:

1. Materialize typed edge transport/source pools in physical EdgeID order, using
   the immutable selected queue model. Invoke an eligible local custom factory
   separately per runtime queue; never reuse a prior Start's operations. Its
   `TryPush`, `TryPop`, and `Len` must all be non-nil or return
   `ErrInvalidQueueOps`. Shared fan-in owns one shared queue, not one per branch.
2. In canonical physical node construction order, call the applicable explicit
   state or closure factory once per instance. Validate closure-produced handlers
   and selectors for that arity before dispatch is enabled. Successful factory
   results transfer lifecycle ownership even if normalization or Init later fails.
3. Create one worker per immutable physical group, in ascending physical GroupID
   order. Every worker locks its OS thread, attempts its assigned affinity, records
   an outcome, and reports ready before any Init.
4. Dispatch optional Init callbacks **one at a time** in global canonical
   construction order to their locked owner workers. No data handler may run
   before every Init succeeds. Activate the optional unpinned logger only after
   successful Init, then enable processing.

On edge/node-factory, closure normalization, or Init failure, Start must return no
executor, release framework-owned queued/pending resources, and Close every
successfully owned lifecycle exactly once in reverse successful-construction
order. Failed factory returns transfer no lifecycle and must not be closed.
Init-failure rollback must Close on still-locked owner workers, then stop/join
workers before they unlock and exit. Join the trigger and cleanup errors with
`errors.Join`; affinity failure alone is not a startup failure. No logger goroutine
may survive failed startup. The implementation's **pre-worker** failure cleanup
is a limitation of the owner-worker rollback guarantee; see the check below.

A custom queue factory owns cleanup of resources beyond batches exposed through
QueueOps; there is no queue-close callback. Each subsequent sequential Start must
obtain fresh state, closure environments, queues, pools, workers, counters, and
affinity outcomes while preserving immutable plans.

### Startup conformance check

Factory/normalization errors can occur before owner workers exist. Current
rollback closes those owned lifecycles on the startup caller, not a locked worker
(`graph/executor.go`, `rollback`). The intended locked-owner lifecycle contract
therefore remains unfulfilled for that failure phase; this specification does
not assert that all startup failures already meet it. The owner accepts this
limitation for `v0.1.0 experimental`; callbacks that require owner-worker
execution are not guaranteed that execution context for pre-worker rollback.
Changing startup construction/cleanup to provide such a guarantee is deferred.

## Locked workers and affinity

The default CPU-bound manual graph has one physical group and exactly one locked
worker. Expansion adds explicitly planned workers, not dynamic work stealing.
Every worker runs only its own canonical-order runners.

Affinity uses an internal adapter: Linux builds (`linux && !unsupported`) attempt
the requested per-thread CPU affinity; complementary builds return
`ErrAffinityUnavailable`. Any adapter error selects a mandatory locked-thread
fallback, **does not fail Start**, and must not trigger another affinity attempt.
CPU-affinity success does not imply NUMA memory placement. The adapter test seam
is internal, not application configuration, and installations/restorations must
be serialized against parallel tests.

`(*Executor).AffinityStatus() []AffinityStatus` must return an independent
physical-worker-ordered slice available after stop. Rows contain logical
`GroupID`, zero-based group `Replica`, optional node-pool `NodeID`/`NodeReplica`,
explicit `CPU`, `Outcome`, and `Warning`. `AffinityOutcomeApplied` has no warning;
`AffinityOutcomeLockedThreadFallback` preserves `ErrAffinityUnavailable` and the
adapter cause. A logical group/node can appear in multiple rows; do not infer
unique workers from logical GroupID alone. Mutating returned rows must not change
later reads.

## Scheduling and publication

Dispatch must use preconstructed monomorphized bindings and numeric metadata for
all explicit/closure arities 0…8 and concrete error type E, with no reflection,
runtime data type assertions, boxed batches/errors, data-plane mutexes, or
blocking data-plane channels. Control/lifecycle coordination may use channels.

| Runner state | Dispatch rule |
| --- | --- |
| Zero-input source | Eligible once per scheduler pass while sources are enabled. Ingress is an adapter, not a user handler. |
| One input | Invoke its typed handler only after successfully obtaining a batch. |
| Multiple inputs | Form a worker-local ReadyMask. One ready position needs no selector; two or more call the typed selector and dispatch exactly its selected handler. |
| Invalid selector | Out-of-range or not-ready position is a contract violation: panic, without executor recovery. |
| Pending Wait output | Retry publication before accepting new input or invoking the source again. Other runners/workers must remain able to progress. |

Handler return is the [ownership finalization boundary](01-typed-graph.md#invocation-ownership).
Successful results publish ordinary commits; failed results discard ordinary
outputs and use only the distinct typed error route, plus independently configured
logging. Untouched input always returns to its origin pool, including sinks.

For every queue model, successful push transfers ownership to the queue, and
successful pop to the consuming handler path. Drop releases a rejected batch
once; Wait retains owned pending work for later scheduler passes, never busy-spins
inside a handler and never blocks downstream progress. Ordinary and error
cross-worker 1:1 paths use preallocated atomic SPSC rings and preserve publication
FIFO. Multi-producer/consumer models follow the
[physical cardinality matrix](03-topologies-transport.md#queue-models-and-selection).
Local built-in/custom operations have one group-worker owner; snapshot readers
must never invoke their callbacks.

## Drain, deadline, and Close

```go
func (e *Executor) Drain() error
func (e *Executor) DrainContext(context.Context) error
func (e *Executor) Close() error
```

Drain delegates to DrainContext with an unbounded background context.
DrainContext requires a non-nil context. Lifecycle calls are sequential;
concurrent Drain/Close calls are unsupported. After completed shutdown repeated
calls must return the retained final error without duplicate cleanup.

| Phase | Required behavior |
| --- | --- |
| Close admission | Close runtime-local ingress admission; finish admitted nonblocking commits before accounting accepted work. Stop new zero-input source invocations. |
| Graceful Drain | Repeatedly pause all workers at safe boundaries, inspect every queue and pending runner/topology state, and resume until one global paused snapshot is empty. Process accepted ingress work and pending source commits without reinvoking disabled source handlers. |
| Canceled DrainContext | At the next acknowledged all-worker boundary, force-discard remaining framework-owned work exactly once. Preserve both `ctx.Err()` and `context.Cause(ctx)` plus cleanup errors. |
| Close | Pause/stop production and discard queued, staged, and pending work without normal processing; no further handlers after the acknowledged boundary. |
| Participants | Ingress and logger use the original shared drain context, not independent timeout budgets. Forced shutdown discards unconsumed work and joins participants. |
| Lifecycles | Close provided callbacks once, in reverse global successful-construction order, on still-locked owner workers. Stop and join every worker so it unlocks before exit. |
| Final outcome | Join cleanup errors, retain exact final counters and affinity metadata, and release each framework-owned batch once. |

Cancellation is cooperative: it cannot interrupt nonreturning handlers,
selectors, queue callbacks, lifecycle callbacks, or logger handlers. A deadline
is therefore not a hard upper bound on completed shutdown.

## External typed ingress

```go
func AddIngress[T any](*Builder, IngressConfig, ...IngressAllocator[T]) (IngressDefinition[T], error)
func BindIngress[T any](*Executor, IngressDefinition[T]) (*Ingress[T], error)
```

IngressConfig carries `Capacity, BatchCapacity uint32`, `Policy queue.Policy`, and
inactive `NUMA numa.Placement`; the optional typed allocator descriptor is dormant.
AddIngress must validate before registering one dormant source NodeID and one
ordinary typed output. `IngressDefinition.NodeID()` and `.Output()` expose those
identities. Invalid configuration/provenance/stopped binding must reject with
`ErrInvalidIngress`; invalid/sealed builder errors retain their usual categories.
BindIngress resolves only the owning Start's fresh endpoint. Producer operations
must be serialized by one external caller, while admission closure may overlap it.

| Producer operation / outcome | Ownership |
| --- | --- |
| `Acquire() (batch.Batch[T], bool)` | Return caller-owned preallocated source storage; closed/exhausted returns no batch. |
| `Commit(*batch.Batch[T]) IngressStatus`: `IngressAccepted` | Publish exactly one owned batch, clear caller handle, and include it in stop accounting. |
| `IngressFull` under Wait | No publication; caller retains the unchanged handle for retry/release. |
| `IngressDropped` under Drop | Consume, clear, and release only that full-queue batch. |
| `IngressClosed`, `IngressInvalidBatch`, `IngressTooLarge` | Reject without consuming caller ownership. |
| `Submit([]T) IngressStatus` | Copy at most one bounded batch; reject oversize atomically, return `IngressExhausted` if storage is unavailable, and release its private acquisition on every nonaccepted/non-Drop outcome. Empty input is an accepted no-op. |

An overlapping Commit either admits one batch that stop must process/reclaim or
rejects without transfer (apart from configured Drop). Uncommitted caller-owned
batches remain releasable after stop; stale endpoints cannot publish into a later
Start. Adapter polling does not count as user HandlerCalls. Ingress pool metrics
use its registered source NodeID with EdgeID zero, rather than borrowing the
downstream edge's origin identity.

## Required execution checks

Tests must cover placement/binding precedence and defaulting, canonical physical
plans, exact worker counts, local/remote ordinary and error delivery, FIFO SPSC,
Drop/Wait, factory freshness, rollback and reverse lifecycle order, lock-before-Init,
owner-worker Init/Close, deadline and forced cleanup, ingress/stop overlap, and
immutable snapshots. Linux and unsupported-tag adapter tests must not depend on
host topology/permissions; injected outcomes must cover applied and fallback.
Concurrent handoff, snapshot, affinity, and lifecycle tests must run race-free and
without leaked workers. Built-in steady-state allocation gates and their exclusions
are centralized in [performance conformance](05-planning-performance.md#benchmark-and-allocation-conformance).
