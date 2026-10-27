# Topologies and transport

Ordinary [Connect and ConnectError](01-typed-graph.md#connections-and-local-error-precedence)
retain one producer per input and one consumer per output. Parallel delivery must
be explicit: broadcast copies, sharding divides elements, fan-in merges producers,
and physical expansion distributes each accepted batch to one consumer.

## Queue models and selection

`ResolveQueueModel(producers, consumers uint32, sameWorker bool) (QueueModel, error)`
must be pure and select the least concurrent valid built-in transport:

| Physical ownership | Producers | Consumers | Model |
| --- | --- | --- | --- |
| All operations on one worker | 1 | 1 | `QueueModelLocal` |
| Cross-worker | 1 | 1 | `QueueModelSPSC` |
| Cross-worker | >1 | 1 | `QueueModelMPSC` |
| Cross-worker | 1 | >1 | `QueueModelSPMC` |
| Cross-worker | >1 | >1 | `QueueModelMPMC` |

Zero endpoint counts or sameWorker with counts other than 1:1 must return
`QueueModelInvalid` and `ErrInvalidQueueCardinality`. Count **workers**, not logical
ports: multiple producers on one worker do not require MPSC. Select after replica,
node-pool, or automatic-group physical planning, without invoking factories or
creating queues. Start must materialize fresh typed storage from the frozen plan.

`Definition.EdgeQueuePlans()` must return independent ordered physical rows,
including EdgeID, LogicalEdgeID, Replica, MergeID/BroadcastID/ShardID where
applicable, Model, ProducerWorkers, ConsumerWorkers, Remote, and inactive NUMA.
Copied plan metadata must not expose runtime resources or private binding pointers.

Custom `queue.Factory[T]` is local-only. Manual cross-group ordinary/error edges
must report `ValidationCodeRemoteQueueFactory`; incompatible physical shared
ownership must reject with `ErrIncompatibleQueueFactory` before factory invocation.
Nil factories select built-in queues. QueueOps is exactly:

```go
type QueueOps[T any] struct {
    TryPush func(batch.Batch[T]) bool
    TryPop func(*batch.Batch[T]) bool
    Len func() uint32
}
```

Local rings use preallocated handles and worker-owned cursors/count; cross-worker
SPSC uses atomic cursor publication. `queue.NewMPSC[T](uint32)`, `NewSPMC[T]`, and
`NewMPMC[T]` must return validated preallocated queues compatible with QueueOps
through `Ops()`. Zero/unrepresentable capacities reject without usable storage.
TryPush/TryPop must not wait for another caller: failure retains caller ownership,
success publishes or returns one fully initialized exclusive batch exactly once.
Concurrent `Len` is an observational, non-linearizable value bounded by capacity.
Cursors must wrap modulo capacity without loss/duplication, including non-power-
of-two capacities and counter boundaries. Failed operations consume no storage;
steady-state operations allocate no framework memory.

Queue callbacks supplied by callers must be nonblocking and FIFO and must respect
the same ownership contract. All queue models use the shared
[Drop/Wait and stop rules](02-execution-lifecycle.md#scheduling-and-publication):
Drop releases once, Wait retains producer-local pending work without blocking
downstream or reinvoking its handler. No fairness or global ordering guarantee is
added for parallel paths.

## Explicit topology registration

```go
func ConnectBroadcast[T any](*Builder, OutPort[T], []InPort[T], EdgeConfig[T]) ([]EdgeID, error)
func ConnectShard[T any](*Builder, OutPort[T], []InPort[T], func(T) uint32, EdgeConfig[T]) ([]EdgeID, error)
func ConnectFanIn[T any](*Builder, []OutPort[T], InPort[T], EdgeConfig[T]) ([]EdgeID, error)
```

Each API must register its fixed homogeneous endpoint list atomically: validate
builder state, typed provenance, ordinary-port eligibility, distinct/unoccupied
endpoints, configuration, and cycle constraints before committing branch IDs or
occupying ports. Invalid/foreign/duplicate/connected/cyclic requests must leave
all endpoints and IDs unchanged. Lists require at least two branches/producers;
sharding also requires a non-nil selector. ErrorPort is not an ordinary output
and these APIs do not add error-output fan-out. Returned dense branch EdgeIDs
identify the explicit topology with its first branch ID as topology identity.

| Topology | List semantics / diagnostics |
| --- | --- |
| Broadcast | Canonical destination order by node/port. Invalid destination sets use `ErrInvalidBroadcast`; source occupancy retains `ErrFanOutUnsupported`. |
| Shard | **Supplied destination order** defines selector index, not sorted node order. Invalid setup uses `ErrInvalidShard`; out-of-range runtime selection panics with `ErrInvalidShardIndex`. |
| Fan-in | Canonical producer order by node/port. Invalid producer cardinality/set uses `ErrFanInUnsupported`; cycle-producing registration uses `ErrFanInCycle`. |

### Broadcast copies

Each delivered branch must own a **different pooled slot** containing the source's
logical elements with matching configured batch capacity. Copy of pointer-bearing
T is shallow: slot exclusivity does not deep-copy pointed-to objects. Pool
exhaustion loses only that branch copy. Queue Drop releases that branch; Wait
retains and retries its already-made copy without recopying or redelivering
completed branches. The source remains owned until all branches finish or discard.

The source logical length must fit the broadcast BatchCapacity, including when a
larger-capacity input was moved into the output. Oversized publication must reject
every branch before copying and increment each branch's CapacityRejectedBatches/
Elements, separately from pool exhaustion and queue Drop. Drain/Close must account
for all source, queued, and branch-pending ownership exactly once.

### Element sharding

For each processed source element, call the typed selector once and assign it to
the selected supplied-list branch without boxing. Invalid indices are contract
violations, not silently remapped. Wait resumption must retain progress/choice,
not call the selector again for an already-assigned element.

Staging is bounded to per-branch batches and at most one active source batch.
Fill and flush full staging batches, split as needed, and flush partial batches at
input completion; no capacity growth is permitted. Branch pool exhaustion drops
only the selected element and increments PoolExhaustions plus PoolDroppedElements;
queue Drop loses that staged batch's elements, a distinct outcome. Wait retains
staging/source progress until publication can continue. Cross-branch global order
is unspecified. Forced Close releases source, partial, pending, and queued batches
once and attributes remaining **unassigned** source elements to the first branch's
ShardShutdownElements.

### Unordered fan-in

All producers feed one bounded shared queue for the target logical input. Accepted
batches must reach one consumer exactly once with their originating pool identity
intact; releasing downstream returns to the producing pool, not a merge-wide pool.
There is no cross-producer delivery order. Each producer owns its pool and pending
output: a full-queue Drop/Wait outcome affects that publishing branch, not other
producers' ownership. Stop must drain/discard the shared queue once and each
producer's pending batch once.

## Physical parallelism

Expansion must be validated and frozen at Build without node/queue factories,
worker creation, or affinity calls. Every physical instance receives a fresh
factory result per Start and [owner-worker lifecycle](02-execution-lifecycle.md#startup-and-rollback).
Shared queues distribute accepted batches once, never implicitly broadcast them.

| Property | Whole-group replicas | Node worker pool |
| --- | --- | --- |
| Declaration | `ReplicateGroup(*Builder, GroupID, uint32) error` | `ReplicateNode(*Builder, NodeID, uint32) error`; alias `SetNodeWorkerPool` |
| CPU assignment | Replica 0 uses BindCPU; `BindReplicaCPU(*Builder, GroupID, replica uint32, cpu int) error` binds zero-based replicas. | `BindNodePoolCPU(*Builder, NodeID, worker uint32, cpu int) error` binds **every** zero-based worker. |
| Valid count / CPU | Positive fixed count; complete unique nonnegative physical CPUs. | Positive fixed count; complete unique nonnegative worker CPUs, separate from retained manual bindings. |
| Scope | Replicate all nodes in a logical group; internal edges pair corresponding replicas. Cross-group boundaries use shared cardinality-selected transport. | Expand only the selected node, leaving neighbors in their existing groups. Each logical input has one shared bounded queue; each worker has its own typed handlers/selectors. |
| Identity | Deterministic logical group/node plus replica identity in copied plans, affinity, and metrics. | Stable logical NodeID plus zero-based worker identity. ReadyMask belongs to the worker making the call. |
| Invalid configuration | `ErrInvalidReplicas`; groups containing ingress reject count >1 with `ErrReplicatedIngress`. | `ErrInvalidWorkerPool`; ingress adapters cannot become node pools. |

Reject missing CPUs, invalid counts, duplicate physical CPUs, identity overflow,
and node-pool/whole-group replication overlap before execution. Broadcast endpoints
must not participate in replica/node-pool expansion (`ErrBroadcastParallelExpansion`);
sharding endpoints have the same boundary (`ErrShardParallelExpansion`). Do not
invent ambiguous parallel branch semantics. Unreplicated ingress **may** feed a
replicated downstream group through a shared SPMC boundary.

Every producer worker must acquire from its own typed source pool, including
shared MPSC/MPMC output transport. No framework-created mutable state or output
pool may be shared between producer instances. Multi-input heterogeneous dispatch
must remain typed per worker. No global output order or fairness is promised.
Drain/Close and startup rollback must include all physical workers and reclaim
every queued/pending batch once.

Registered counters must have independent replica/worker-local slots.
`CounterForReplica` selects a zero-based instance; logical-only Counter on a
multi-instance node must return `ErrAmbiguousCounter`, not an arbitrary slot.
Automatic planning must preserve expansion's hard worker boundaries; its pure
[constrained minimizer](05-planning-performance.md#constraint-preserving-coalescing)
may reduce legal remote edges but cannot erase explicit parallelism.
