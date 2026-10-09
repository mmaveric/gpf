# Go Pipeline Framework

This repository is building a typed, batch-oriented Go pipeline framework. The
typed batch pool is executable, and fixed node arities are generated as public
contracts. Graph builders validate immutable topology definitions, and a valid
definition starts one locked worker per fixed manually placed CPU-bound group through `Definition.Start`.

Steady-state paths use concrete generic batches and generated typed function
closures. They do not use reflection, runtime type assertions, or boxed
data-plane values.

The module path is `github.com/asharipov/gpf` and the minimum supported Go
version is Go 1.22.

## Generated Node Contracts

The `node` package provides sealed `Outputs0` through `Outputs8`, explicit-state
`Node0` through `Node8`, and closure-based `ClosureNode0` through
`ClosureNode8`. Every input has its own typed handler. Multi-input selectors use
zero-based `Port1` through `Port8` positions and must choose a ready input when
called.

Generated node files are committed. Regenerate and verify them with:

```bash
go generate ./...
go test ./internal/genarity ./node
```

`Outputs1` through `Outputs8` expose typed `Out1` through `Out8` values.
Handlers use `Acquire` and `Commit` to produce or move batches. An uncommitted
acquisition is released automatically. Moving a by-value input into a compatible
committed output transfers its original pool slot; the executor does not release
that slot again from its retained input copy. Untouched inputs are released
automatically. A failed `Result` discards ordinary outputs and any un-moved
inputs before optionally publishing one typed error batch.

## Benchmarks

Run the benchmark matrix with:

```bash
go test -run '^$' -bench . -benchmem ./...
```

The pool, remote-queue, and two-group end-to-end entries execute after setup
and report allocations. Throughput and latency are non-blocking comparison
signals; interpret them alongside allocation results rather than as thresholds.
The node-call entry directly invokes a concrete generated handler, and the
local-edge entry measures request-to-delivery through the one-group executor;
both process a logical 64-element batch after setup.

The benchmark matrix reserves these stable names:

- `node-call/batch=64`
- `pool-acquire-release/batch=64`
- `local-edge/batch=64/group=1`
- `remote-queue/batch=64/groups=2`
- `local-error/batch=64/groups=1`
- `remote-error/batch=64/groups=2`
- `end-to-end-dag/batch=64/groups=2/depth=3`

Throughput and latency results are recorded for manual comparison rather than
enforced as merge-blocking thresholds. A passing zero-allocation assertion does
not rule out instruction-count, cache-miss, throughput, or tail-latency
regressions.

## Typed Connection Check

Matching ordinary ports connect through `graph.Connect[T]`; a registered typed
failure port connects only through `graph.ConnectError[E]`:

```go
_, err := graph.ConnectError(builder, producer.Errors(), errorSink.Inputs().In1, config)
```

An error output is optional. An unconnected failure, an exhausted error pool,
or a full Drop edge discards the failure value. A Wait edge retains one typed
error batch and pauses only that producing runner until it publishes. Error
batches have length one, retain their originating pool through local and remote
transport, and can be moved by a consumer to a compatible ordinary output.

## Fixed-Group Execution

`graph.Builder.Build` validates an open graph, then returns a cached immutable
definition on success. The definition reports node and edge counts and a copied
canonical topological order. Successful Build seals registration and connection;
failed validation leaves the builder open for correction.

Build retains state, closure, and queue factory configuration without invoking
factories, handlers, or lifecycle callbacks. `Definition.Start` materializes
fresh state, pools, and queues for one worker per fixed group. `graph.Place`
assigns a registered node once to a nonzero opaque group ID; unplaced nodes use
`graph.DefaultGroupID` (1), preserving the existing one-worker graph shape.
Before Build, every effective group must receive one unique nonnegative CPU with
`graph.BindCPU`; this is graph-construction metadata, not Start configuration.
Every ordinary or error edge requires both `Capacity` and `BatchCapacity`. Same-group edges use
the built-in local ring or a fresh custom `QueueOps`; cross-group edges use the
built-in typed bounded SPSC ring, and Build rejects a remote custom factory.
`Drop` releases full-edge outputs and `Wait` retries one pending output on later
scheduler passes without blocking a consumer group.

`Executor.Drain` uses a deterministic all-worker pause barrier, disables
sources, and processes accepted local and remote work to global quiescence.
`Executor.DrainContext(ctx)` uses the same lifecycle with a cooperative deadline:
cancellation switches to forced discard at the next acknowledged worker barrier.
Already committed source outputs may retry, but source handlers are not invoked
again during drain. Stop errors preserve cancellation/cause and cleanup errors;
repeated lifecycle calls retain that result. A deadline cannot interrupt a user
handler, queue callback, logger callback, or lifecycle hook that does not return.
`Executor.Close` discards queued and pending work from that barrier. Both close
node lifecycles in reverse canonical order on their locked owner workers, and
definitions can be reused sequentially after a stop. Linux attempts per-worker
CPU affinity with a pure-Go syscall. Unsupported platforms and rejected Linux
requests keep the worker locked and report the fallback through the copied,
ascending `Executor.AffinityStatus` result; affinity failure does not fail
Start. Automatic grouping, CPU discovery or CPU-set policy, NUMA placement,
handler retries, panic recovery, error fan-out/distribution, and
concurrent starts remain deferred. Export backends remain deferred.

`Definition.EdgeQueuePlans()` returns copied, edge-ordered model and physical
producer/consumer counts. `graph.ResolveQueueModel` selects Local for one shared
worker, SPSC for cross-worker 1:1, MPSC for N:1, SPMC for 1:N, and MPMC for N:M.
The current fixed topology produces Local/SPSC plans. Fixed group replicas expand
these to paired local edges and shared SPMC/MPSC boundary edges. Node worker pools
expand only the selected node while adjacent nodes keep their existing placement.
Build retains only numeric plans and
dormant typed constructors; Start allocates fresh queues from those plans.
Custom factories remain restricted to a Local plan.

## Fixed Group Replicas

`graph.ReplicateGroup(builder, groupID, count)` configures fixed whole-group
replication. Replica zero uses the group's existing CPU binding; every additional
replica requires a unique CPU via `graph.BindReplicaCPU`. Internal edges are
paired by replica. Boundary queues distribute each accepted batch once; they do
not broadcast. Use `Definition.EdgeQueuePlans()` for copied physical endpoint
cardinalities and queue models while `NodeCount`, `EdgeCount`, and
`TopologicalOrder` continue to describe logical topology.

`graph.EnableAutomaticPlanning(builder)` opts an inspection preview into leaving
unplaced nodes free; it does not change Build/Start placement or merge anything.
`Definition.Constraints()` returns copied numeric rows containing explicit
labels and must-link/must-separate boundaries after replica and node-pool
expansion. Distinct explicit labels, ingress, replica crossings, and pooled
worker cuts remain hard boundaries. The preview is input to a later minimizer;
automatic execution remains unavailable until an explicit CPU-complete plan is
finalized.

`graph.MinimizeRemoteEdges(constraints)` is a pure preview algorithm. It uses
canonical edge/node order, preserves explicit labels and mandatory cuts, rejects
cyclic quotient contractions, and reports before/after remote-edge counts plus
rejection reasons. The result is a deterministic local minimum, not a global
optimality claim; it does not alter Builder placement or enable auto execution.
`Builder.PlanGroups()` composes replica/pool expansion, constraints,
minimization, and queue selection into a copied `AutomaticGroupPreview` with
stable IDs and a freshness revision. Enable it with
`graph.EnableAutomaticPlanning(builder)`. Missing CPUs appear as
`CPUBound=false`; `CPUComplete` is false until all groups have assignments.
Registering nodes, connecting ports, or changing placement makes that preview
stale. The preview never seals the builder and cannot start workers.
To execute one, bind every preview group with
`graph.BindAutomaticPlanCPU(builder, &preview, groupID, cpu)`, then call
`graph.FinalizeAutomaticPlan(builder, preview)`. Finalization rejects missing,
duplicate, negative, stale, or foreign bindings, seals the builder, and makes
subsequent `Build()` return that CPU-complete definition. Manual Build/Start is
unchanged when no automatic finalization is requested.

Ingress adapters cannot be in a replicated group yet; an unreplicated ingress
can feed a replicated group. Factories run once per physical node instance and
must create independent state. `Executor.Counter` rejects ambiguous logical
counter lookups on replicated nodes; use `CounterForReplica(nodeID, replica,
counterID)` for a specific zero-based instance. Affinity, metrics, logger, and
mapped-error identities include replica indices. Replica scaling is fixed for
one definition; dynamic resizing and broadcast composition with expanded
endpoints remain unavailable.

## Fixed Node Worker Pools

`graph.SetNodeWorkerPool(builder, nodeID, count)` expands only one logical node
to a fixed worker count. Every worker needs a unique CPU via
`graph.BindNodePoolCPU`. Inputs share bounded typed queues and each accepted batch
is consumed by one worker; outputs use queue models selected from worker
cardinality and each producer has its own origin pool. Heterogeneous input
handlers and selectors remain statically typed and execute against that worker's
state. `CounterForReplica` selects a node worker by its zero-based worker index.
Node pools cannot overlap a replicated group or an ingress adapter in this
version; dynamic resizing, ordering, and scheduling fairness are not promised.

## Inactive NUMA Metadata

`numa.Placement{}` means unspecified; `numa.On(0)` explicitly selects metadata
node zero. `graph.SetGroupNUMA`, `graph.EdgeConfig.NUMA`, and
`batch.PoolConfig.NUMA` and `graph.IngressConfig.NUMA` preserve annotations in copied group/edge/pool views.
Typed `batch.PoolOptions[T].Allocator` and
`graph.AllocatorDescriptors[T]` and the optional `graph.IngressAllocator[T]`
record optional allocator descriptors. Both
placement and allocator descriptors are inactive: there is no NUMA discovery,
local allocation, or affinity change, and descriptors are not called during
registration, Build, or Start. Adding exported struct fields preserves keyed
zero-value configuration but can affect callers using unkeyed struct literals;
prefer keyed literals for exported configuration structs.

Standalone `queue.NewMPSC[T]`, `queue.NewSPMC[T]`, and `queue.NewMPMC[T]`
construct preallocated typed concurrent queues. Try operations can return false
on contention or an unpublished reservation as well as full/empty state; callers
retain ownership on failed pushes. Their bounded Len values are observations,
not globally consistent snapshots.

For an explicit homogeneous merge, use
`graph.ConnectFanIn(builder, []graph.OutPort[T]{...}, input, config)`. It
atomically connects distinct producer outputs to one unconnected input and
returns branch EdgeIDs. Each producer has a separate source pool; batches retain
their origin when the shared bounded MPSC/MPMC transport releases them. Drop and
Wait decisions are local to the publishing branch. Cross-producer order and
fairness are unspecified. Ordinary Connect remains one-producer-per-input and
one-consumer-per-output; FanIn does not copy or broadcast data.

`graph.ConnectBroadcast(builder, output, inputs, config)` explicitly connects an
ordinary output to two or more distinct inputs. It creates one shallow typed
copy in a branch-owned pooled slot per destination; pointer referents are shared,
not cloned. Branch queue policies and pool exhaustion are independent. Wait
retains and retries only its own branch copy, while Drop releases that copy.
Every branch must accept the configured batch capacity; oversized moved batches
are rejected and counted separately. Broadcast endpoints cannot participate in
whole-group replica or node-worker-pool expansion in this version.

`graph.ConnectShard(builder, output, inputs, selector, config)` routes each
element to one destination, where the selector's `uint32` result is an index into
`inputs` in the order supplied. A selector result outside that range is a
contract violation and panics with `graph.ErrInvalidShardIndex`. The runtime
uses fixed per-branch staging batches, carries at most one source batch/cursor,
and flushes partial branch batches at source-batch completion. Wait retains a
full branch batch and resumes from the next element without rerunning the
selector; Drop counts dropped elements. Pool exhaustion drops only the current
element and is reported separately as `EdgeMetrics.PoolDroppedElements`.
`EdgeMetrics.ShardShutdownElements` counts source elements not yet assigned to a
branch when forced shutdown discards a pending sharding operation. Sharding
endpoints cannot participate in whole-group replica or node-worker-pool
expansion in this version, and cross-branch global order is unspecified.

## Runtime Metrics

Every executor exposes an independent numeric snapshot:

```go
snapshot := executor.Metrics()
for _, node := range snapshot.Nodes {
	// HandlerCalls, input batches/elements, failed results, unconnected errors.
}
for _, edge := range snapshot.Edges {
	// Accepted/dequeued work, queue/pool element losses, retries, and discards.
}
```

Rows are ordered by their graph IDs and include zero-valued planned groups,
nodes, and connected edges. Snapshots copy their slices; changing a returned
snapshot cannot alter executor state. Calls are safe while workers process and
while one supported Drain or Close proceeds. Each atomic field is read
independently, so live snapshots are not a transaction; after stop, cumulative
counters are exact. Counters wrap modulo 2^64.

Node input counts are captured before handler ownership changes. Edge accepted
counts mean successful queue ownership transfer; dequeued counts mean batches
given to normal handlers. Queue-policy drops, failed committed ordinary outputs,
pool acquisition exhaustion, Wait retry attempts, and forced-close discards are
separate counters. WaitRetries counts subsequent push attempts, including the
successful retry, but not the first push. PoolExhaustions count unavailable
acquisition attempts and do not invent a lost batch.

QueueDepthSample and QueueHighWaterSample are bounded observations made by queue
owners after transport operations; they can miss short-lived peaks. Snapshot
readers never call custom queue operations. After Drain or Close, sampled depth
is zero while the observed high-water sample remains. `Metrics().Pools` reports
edge-origin pool capacity and sampled in-use/high-water observations without
reading batch contents or invoking custom queue operations.

Handler timing is opt-in through `graph.SetTiming(builder, graph.TimingConfig{
Mode: graph.TimingSampled, Every: 64})` or `TimingFull`. The default is off.
Sampled mode measures calls one, 65, 129, and so on independently per node.
Node snapshot fields report measured calls, measured nanoseconds, and maximum
measured nanoseconds; unsampled work is not extrapolated. Selectors, output
finalization, logging, and Wait retries are outside the measured interval.
`TimingConfig.Clock` optionally supplies `func() uint64` nanosecond timestamps;
nil keeps the built-in monotonic clock. The same callback may run on multiple
workers and must be concurrency-safe, nonblocking, and nonallocating. Readings
must advance monotonically modulo uint64 with measured intervals below 2^63.
Build, snapshots, and disabled/unsampled calls never invoke the custom source.

Register fixed numeric user counters with `graph.RegisterCounter(builder,
nodeID, counterID)` before Build. Generated handler sets' `BindCounter` hooks
install update functions into fresh state/closures during normalization.
`Executor.Counter` provides a control-plane lookup alternative. Bound updates
allocate no framework memory, add modulo uint64, and become no-ops after stop.
`Metrics().Counters` returns independent rows ordered by node and counter ID.

`graph.SetWorkerTiming` optionally measures complete scheduler passes using a
separate `WorkerTimingConfig{Every, Clock}`. Group rows expose measured passes,
idle polling nanoseconds, and progressing polling nanoseconds. These are sampled
wall durations, not OS CPU time or estimates of unsampled work.

## Typed Error Mapping

Explicit-state and closure handler sets have an optional `MapError` field of
type `node.ErrorMapper[E]`. Its `Map(value)` helper converts concrete E to a
fixed `node.ErrorEvent`: numeric code/severity, four numeric attributes, and
runtime-owned numeric node/group/replica/worker identity. A nil mapper converts
to a zero event. Error events contain no strings, slices, or pointer payloads;
formatting belongs in the configured asynchronous logger callback.

Mapper configuration is captured with the handler set. Build does not invoke
it, and runtime failures do not invoke it without an installed logging sink.
An installed sink receives one mapped event per failed return, independently of
whether the typed error edge accepts, drops, exhausts its pool, or is absent.
Worker mapper callbacks must return without blocking or allocating; the
framework cannot certify user callbacks and does not recover their panics.

Configure logging once on an open builder with `graph.SetLogger(builder,
graph.LoggerConfig{Capacity: 64, Handle: callback})`. Build stores copied dormant
configuration. Start creates one bounded event ring per physical worker and an
unpinned logger goroutine after all node initialization succeeds. An omitted
mapper produces a metadata-only zero payload; full rings drop immediately.
Callbacks run outside data workers and may format or allocate there.

`Metrics().Loggers` reports attempted, accepted, full-ring dropped, delivered,
and shutdown-discarded events per group. Drain flushes accepted events with the
same context budget as data processing; cancellation discards queued leftovers.
Stop joins the logger after its in-flight callback returns. A callback that does
not return prevents completed shutdown; the framework does not detach it.

## External Typed Ingress

Register an external source with `graph.AddIngress[T](builder, config)`, connect
its `Output()` to a matching input, and obtain a producer after Start with
`graph.BindIngress(executor, definition)`. Each Start owns fresh ingress storage;
an endpoint from a stopped executor cannot publish into a later Start.

The single external producer may Acquire/fill/Commit an ingress batch or Submit
a slice for copying. Accepted and Dropped statuses clear the committed handle;
Full under Wait and Closed retain caller ownership for retry or release.
Oversized slices are rejected without partial publication. Producer operations
must be serialized, while admission closure may overlap them. Drain closes
admission, finishes admitted commits, then processes accepted ingress work.
Uncommitted acquisitions remain caller-owned and releasable after shutdown.

## Runnable Showcase

[`examples/showcase`](examples/showcase/README.md) is a dependency-free order
pipeline demonstrating typed ingress/errors, broadcast, customer-key sharding,
fan-in, fixed CPU-bound groups, graceful drain, and runtime metrics. Run it with
`go run ./examples/showcase`.
