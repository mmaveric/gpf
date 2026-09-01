## Context

Stage 3 builds an immutable DAG and starts one configuration-free executor with one local worker. Its node and edge bindings are monomorphized, batches retain origin-pool ownership, and every start rematerializes factories. The current local ring and custom `QueueOps` are called by one worker, so neither establishes cross-goroutine memory ordering.

The original plan calls for hybrid groups, queue-model selection, CPU affinity, replicas, observability, and later automatic grouping. This stage deliberately extracts only the prerequisite manual fixed-placement and one-producer/one-consumer path. It preserves the existing `Definition.Start`, `Executor.Drain`, and `Executor.Close` surface and leaves `queue.Factory` unchanged.

## Goals / Non-Goals

**Goals:**

- Declare fixed node-to-group placement while the builder is open, compile it into deterministic immutable metadata, and preserve implicit group 1 for unplaced nodes.
- Start exactly one worker goroutine for every planned group, preserving one worker and local queues for a wholly default-group graph.
- Use a bounded typed atomic SPSC ring only for an edge whose source and target groups differ.
- Preserve edge-ID and canonical-node ordering for startup, rollback, lifecycle ownership, factory freshness, batch ownership, and deterministic shutdown.
- Establish allocation, benchmark, functional, and race coverage for the supported remote path.

**Non-Goals:**

- Automatic grouping, group merging, placement changes after declaration, public plan editing, CPU IDs, `runtime.LockOSThread`, affinity, NUMA, or timeouts.
- Group or node replicas, worker pools, MPSC/SPMC/MPMC queues, ingress, fan-out, fan-in ordering, sharding, broadcast, or distribution.
- Remote custom queues, remote transport, metrics, logging, error ports, error routing, retries, panic recovery, or concurrent `Start`, `Drain`, and `Close` calls.

## Decisions

### Add only fixed placement at construction

`graph` will expose exactly these additions:

```go
const (
    InvalidGroupID GroupID = 0
    DefaultGroupID GroupID = 1
)

func Place(builder *Builder, node NodeID, group GroupID) error

var ErrInvalidGroupID
var ErrUnknownNode
var ErrNodeAlreadyPlaced
```

`Place` is valid only on an open initialized builder and records one nonzero explicit assignment for an existing node. Its error precedence is invalid builder, sealed builder, invalid group ID, unknown node ID, then duplicate placement. It does not alter registration, connections, edge IDs, callbacks, or factories. A node without a placement has the effective `DefaultGroupID`; a placement cannot be overwritten. Sparse positive IDs are valid and have no affinity meaning.

No `Group`, `GroupConfig`, `StartConfig`, placement accessor, or public group-plan value is added. The definition retains a private plan because the runtime, not a caller, is its sole consumer. Requiring every existing graph to add a placement was rejected because it would break equivalent Stage 3 one-group behavior. Allowing reassignment was rejected because it makes a supposedly fixed placement mutable and adds correction/removal API surface.

### Validate and compile a deterministic private group plan in Build

`Validate` and therefore `Build` will report `ValidationCodeInvalidGroupPlacement` for malformed stored placement and `ValidationCodeRemoteQueueFactory` for a cross-group edge retaining a non-nil factory. Build remains transactional: any issue returns the existing `*ValidationError`, leaves the builder open, does not invoke a factory, and publishes no partial definition.

For an issue-free graph, Build resolves defaults, sorts distinct effective group IDs in ascending numeric order, and records each group's nodes in the existing canonical topological order. It classifies each edge by comparing the source and target effective groups: equal is local; different is remote. The copied plan is private, immutable, and contains no queue, pool, worker, factory result, or lifecycle state. The stage does not create missing groups, renumber sparse IDs, merge groups, or choose placements automatically.

An execution-plan accessor was rejected because it would expose a mutable-looking configuration object without a use beyond runtime construction. Recomputing classification at Start was rejected because Build is the control-plane boundary and must reject unsupported remote factories before any runtime allocation.

### Use framework-owned SPSC rings for remote edges only

Every remote edge creates a preallocated typed bounded SPSC ring from its existing capacity and batch-capacity configuration. The source group worker is its sole producer and the target group worker is its sole consumer: fan-out is already rejected, and this stage creates exactly one worker per group. The ring publishes slot contents with acquire/release atomic cursor operations; it contains no locks, channels, allocation, or type erasure in its steady-state path.

The existing local ring and custom `queue.QueueOps[T]` remain available only for local edges. A non-nil `queue.Factory[T]` on a remote edge is invalid at Build and never called. `Drop` releases a rejected remote output; `Wait` leaves one pending output on its producing runner and retries it on later passes while the consumer worker continues independently. The public `queue` API remains unchanged, avoiding an unsupported assertion that arbitrary custom operations are safe under concurrent SPSC use.

Using a custom remote factory was rejected because `QueueOps` has no producer/consumer concurrency or memory-ordering contract. Selecting MPSC, SPMC, or MPMC was rejected because replicas, pools, sharding, and fan-out are absent, so each remote edge is structurally SPSC.

### Keep startup global and workers group-local

Start processes all edges in ascending `EdgeID`, creates every pool, chooses local or remote queue binding from the compiled classification, and calls local custom factories once. It then invokes every node factory and binds one runner to its planned group in canonical node order, followed by Init in that same order. Each logical node has one fresh factory result per start; this stage adds no replica multiplication.

Only after all resources and Init calls succeed does Start create one worker goroutine for each group in ascending `GroupID` order. Before worker publication, any failure drains created queues and pending output, releases framework-owned batches, and closes successful node lifecycles in reverse canonical construction order with `errors.Join`. A successful later sequential Start repeats all factory construction. Keeping lifecycle ownership at the executor coordinator, rather than independently closing workers, preserves the existing reverse global order.

Per-group startup was rejected because a later group failure would expose a partially running executor. Per-group factory replicas were rejected because that is a later replica feature, not a consequence of placement.

### Coordinate shutdown with a deterministic pause barrier

The caller of `Drain` or `Close` acts as the lifecycle coordinator. It asks every group worker, in ascending group order, to pause at a runner boundary and waits for all acknowledgements; control channels are outside the data plane. It then disables all sources. Drain resumes all workers and repeats the pause barrier until, while every worker is paused, every runner has no pending output and every local and remote queue is empty. No new source work can appear, so that barrier proves global quiescence even when the group-level dependency graph is cyclic.

Drain then stops workers, defensively releases remaining queue entries, and invokes owned Close callbacks once in reverse canonical construction order. Close uses the first pause barrier, discards pending and queued local and remote batches without further handlers, stops workers, and runs the same global reverse close sequence. Repeated calls after stopping return the cached result; concurrent lifecycle calls remain unsupported. There is no timeout, CPU pinning, logger flush, or ingress handshake.

Independent worker-local drain was rejected because a downstream group could declare itself empty before an upstream group publishes a remote batch. A pure atomic polling detector was rejected because it cannot establish a stable cross-group quiescence snapshot without a control-plane barrier.

### Preserve typed, measurable data-plane boundaries

Generated bindings remain responsible for typed input polling, output acquisition, commit, publication, and input cleanup. A batch has the same origin-aware ownership across local and remote edges. The built-in remote enqueue/dequeue and two-group source-to-sink path join the established zero-framework-allocation guarantee after Start; factory calls, custom local queues, lifecycle coordination, handlers, selectors, and shutdown remain excluded.

Tests will cover sparse IDs, defaults, duplicate and malformed placement, deterministic plans, remote-factory rejection without invocation, factory order and freshness, SPSC FIFO/capacity/Drop/Wait/ownership, rollback, drain/close barriers, single-group equivalence, allocation gates, benchmarks, and `go test -race`. Throughput remains a reported benchmark signal, not a merge-blocking threshold.

## Risks / Trade-offs

- [A remote edge is accidentally backed by a non-concurrent custom queue] -> Build rejects every remote configured factory before Start.
- [A group becomes idle before an upstream group publishes] -> Drain uses all-worker pause barriers and only completes from a stable globally empty snapshot.
- [Atomic cursor changes regress the local path] -> Local edges retain the existing non-atomic ring and one-group allocation/behavior coverage.
- [A handler or local custom queue violates ownership or non-blocking contracts] -> Preserve the existing documented caller responsibility and test framework-managed cleanup paths.
- [Sparse manual IDs or cross-group DAG patterns produce surprising schedules] -> Preserve supplied IDs, use deterministic ascending group and canonical node order, and add no auto-planning heuristic.
- [Control-plane channels are mistaken for data-plane synchronization] -> Restrict channels to startup and lifecycle barriers; remote batches use only typed SPSC storage.

## Migration Plan

1. Add placement metadata, errors, validation codes, and immutable plan compilation while preserving default group 1.
2. Generate or adapt group-local typed bindings; retain local queue binding and add the internal SPSC remote ring.
3. Generalize startup, rollback, scheduling, and pause-barrier lifecycle control without changing existing lifecycle signatures.
4. Add focused tests, allocation gates, benchmarks, documentation, formatting, generation verification, unit/race/vet runs, and strict OpenSpec validation.

There is no deployed runtime format or persisted graph definition. Rollback removes placement and multi-group execution; unplaced graphs continue using the existing Stage 3 path.

## Open Questions

None. CPU placement, automatic planning, richer queue models, replicas, observability, error routing, and concurrent lifecycle semantics are intentionally deferred to dedicated changes.
