# Errors and observability

Failure delivery is typed and bounded; numeric observation must not change batch
ownership or scheduling. Logging and timing are explicit opt-ins, independent of
always-enabled runtime counters. Construction and immutable configuration follow
the [typed graph contract](01-typed-graph.md).

## Failure routing and mapping

Each registered E has one optional `ErrorPort[E]` at output port 9, separate from
all eight ordinary outputs. Only `ConnectError` connects it to `InPort[E]`; type
mismatch and passing it to ordinary Connect must fail compilation. Provenance,
single-consumer cardinality, configuration, and deterministic error precedence
are defined once under [connections](01-typed-graph.md#connections-and-local-error-precedence).

For each normally returned `Result[E]{Failed:true}`, the executor must:

1. Publish **no ordinary outputs**; reclaim ordinary acquisitions/commits and
   untouched input under the [invocation ownership rules](01-typed-graph.md#invocation-ownership).
2. If logging is installed, map/attempt one numeric event independently of whether
   the error edge exists, its pool exhausts, or its queue drops/waits.
3. If error output is unconnected, discard E without creating an error batch or
   retaining failure state. Otherwise acquire at most one framework-owned typed
   source batch, set `Mutable()[0] = Value` and length one, and publish it.

| Typed error transport outcome | Required ownership |
| --- | --- |
| Pool exhausted | Discard E immediately; retain no pending result. |
| Push accepted | Transfer the one-element batch to the typed queue. |
| Queue Drop | Release that batch once. |
| Queue Wait | Retain one pending error batch, retry later, and do not reinvoke the handler until publication finishes. |
| Consumer / stop | Consumer receives ordinary `Batch[E]`; untouched release, compatible forwarding, drain/discard, and rollback preserve its origin pool. |

Only the generated binding may acquire error-transport batches; handlers receive
no error `node.Output[E]`. E must stay unboxed throughout. This transport adds no
application-level retry policy or panic recovery, and no implicit error fan-out.
Separately configured logging, metrics, replicas, and planning are not prohibited
by the older error-routing stage boundary.

`node.ErrorMapper[E]` is `func(E) node.ErrorEvent`. ErrorEvent is a fixed numeric
payload: `NodeID, GroupID, Replica, Worker uint32`, `Code uint32`, `Severity uint8`,
and `Attributes [4]uint64`; it carries neither formatted strings nor boxed E.
Generated explicit/closure handlers expose optional typed `MapError` of matching
E. `ErrorMapper.Map(E)` is an independently usable conversion helper: preserve its
numeric result; nil returns a zero event. Runtime bindings overwrite identity
fields with framework-owned identity, not caller-supplied values.

Registration/Build never call mappers. Start without a logger leaves runtime
mapping dormant, even when a mapper is configured. Mapper captures must be
immutable; worker-invoked callbacks must be concurrent-safe as shared,
nonblocking, and nonallocating. The framework neither certifies those properties
nor recovers callback panics.

## Bounded asynchronous logging

```go
type LoggerConfig struct {
    Capacity uint32
    Handle func(node.ErrorEvent)
}
func SetLogger(*Builder, LoggerConfig) error
```

One initialized open builder may accept one copied valid configuration. Error
precedence is invalid builder, sealed builder, invalid configuration
(`ErrInvalidLogger`: zero/unrepresentable per-worker ring capacity or nil Handle),
then duplicate valid configuration (`ErrLoggerConfigured`). Rejection stores
nothing; caller captures remain immutable caller-owned configuration.

Build/finalization must not invoke mapper/Handle, create rings/counters, or start
the logger. Each successful Start creates fresh bounded per-physical-worker event
rings and one **unpinned** logger goroutine after all data-node Init callbacks
succeed. Each failed return attempts one event, including a zero numeric payload
for nil MapError. Full rings drop that event with an independent counter, never
block a data worker or typed error route, and never box E/format an event there.
Handle runs outside data workers and may format/allocate, but must return
cooperatively for shutdown to complete. Disabled logging creates no worker/rings,
invokes no mappers, and yields no logger metric rows.

Graceful drain must finish accepted events within the original shared drain
budget; canceled/forced shutdown discards queued leftovers and waits for an
in-flight Handle to return before joining. Every accepted event must end delivered
or shutdown-discarded exactly once; a nonreturning callback prevents completed
shutdown. Startup rollback before activation must not leave a logger running.

Per-worker copied `LoggerMetrics` rows contain GroupID/Replica and
`AttemptedEvents`, `AcceptedEvents`, `QueueDroppedEvents`, `DeliveredEvents`, and
`ShutdownDiscardedEvents`. After completed stop, modulo uint64 arithmetic:

```text
AttemptedEvents = AcceptedEvents + QueueDroppedEvents
AcceptedEvents  = DeliveredEvents + ShutdownDiscardedEvents
```

## Numeric runtime snapshots

`func (e *Executor) Metrics() MetricsSnapshot` returns independently owned slices
`Groups`, `Nodes`, `Edges`, `Ingresses`, `Pools`, `Counters`, and `Loggers`. Nil/zero
executors return zero snapshots with nil slices. Every planned group/node/edge
must be represented even with zero counts, in deterministic physical-plan order;
unexpanded entities follow ascending corresponding IDs. Expanded rows retain
logical IDs plus zero-based instance/replica identity rather than pretending
logical IDs are unique physical workers. Edge rows retain source/target node and
group IDs, ErrorEdge/Remote, and applicable merge/broadcast/shard identity.

Metrics must be safe concurrently with processing, other readers, and one
supported stop operation, without pausing workers or invoking custom queue/user
callbacks on the reader. Values are individually race-safe, **not a globally
atomic transaction**. Snapshot allocation is allowed in the control plane.
Completed stop retains exact cumulative counts, metadata, zero sampled queue
depth, and historical high-water samples; repeated stop must not add cleanup
counts. Each Start gets fresh counters and old executors retain their final rows.
Definition/Build contain no mutable metric resources or collection callbacks.

### Counters and pressure semantics

All cumulative uint64 counters wrap modulo 2^64. Capture input/output lengths
before handles are cleared or user code changes ownership.

| Row / fields | Increment or observation boundary |
| --- | --- |
| Node: `HandlerCalls` | Once per normally returned user source/input handler; never selector, ingress adapter poll, empty poll, or Wait retry. |
| Node: `InputBatches`, `InputElements` | Supplied input and its length **before** handler changes, for either success or failure. Sources have no synthetic input. |
| Node: `FailedResults`, `UnconnectedErrors` | Failed returns once; UnconnectedErrors only when no error edge, not connected pool exhaustion or queue Drop. |
| Edge: `AcceptedBatches/Elements` | Successful queue transfer only. |
| Edge: `DequeuedBatches/Elements` | Normal delivery from queue to handler path, not forced queue discard. |
| Edge: `QueueDroppedBatches/Elements` | Committed work rejected under queue Drop, distinct from pool/capacity loss. |
| Edge: `FailedOutputBatches/Elements` | Committed ordinary output discarded on failed result; abandoned scratch is not committed-work loss. |
| Edge: `ShutdownDiscardedBatches/Elements` | Committed queued/pending work discarded once at forced stop. |
| Edge: `PoolExhaustions` | Failed acquisition **attempts**, not fabricated lost batches. |
| Edge: `WaitRetries` | Subsequent push attempts, including successful retry, excluding initial push. Two failed retries plus a successful retry count three. |
| Broadcast: `CapacityRejectedBatches/Elements` | Oversized moved source rejected separately on every branch before copy. |
| Shard: `PoolDroppedElements` | Selected elements lost because no staging slot; distinct from queued-batch Drop. |
| Shard: `ShardShutdownElements` | Unassigned source suffix at forced stop, attributed to first branch. |
| Group: `Passes`, `IdlePasses` | Each completed scheduler pass; additionally idle if scheduler reports no runner progress. |
| Edge gauges: `QueueCapacity`, `QueueDepthSample`, `QueueHighWaterSample` | uint32 bounded owner-observed samples after transport operations. High-water is the largest **observed** sample, not every transient peak; live depth may lag. |
| Pool: `Capacity`, `InUseSample`, `HighWaterSample` | Bounded independently observed utilization, race-safe with remote releases and allocation-free updates. No transactional gauge claim. |
| Ingress: accepted/drop batch/element counts, `PoolExhaustions` | External admission outcomes, separately from downstream edge work. |

Pool rows preserve producer/source identity, EdgeID and applicable topology/replica
identity, and may report inactive NUMA intent. Ingress source pools use source
NodeID with EdgeID zero. Pool high-water must survive exhaustion/recovery. Current
runtime snapshots omit configured NUMA annotations on some ordinary-edge and
broadcast/shard scratch-pool paths; this limitation is accepted for `v0.1.0
experimental` and is tracked for post-release work. These annotations do not imply
that memory placement is active.

`SetGroupMetricsFlushEvery(*Builder, every uint64) error` may batch publication
of group pass/timing counts: default one, zero rejected with
`ErrInvalidMetricsFlushEvery`, repeat configuration with
`ErrGroupMetricsFlushConfigured`. Live publication may lag by at most every−1
passes; worker barriers flush remaining counts. This does not change final totals.

### Fixed application counters

`RegisterCounter(*Builder, NodeID, CounterID) error` reserves slots before Build.
`CounterID` aliases `node.CounterID uint32`; zero is invalid and IDs are unique
within a node, reusable between nodes. Error precedence is invalid/sealed builder,
unknown node (`ErrUnknownNode`), zero ID (`ErrInvalidCounterID`), duplicate
(`ErrCounterRegistered`); failure adds no slots.

Generated explicit `BindCounter node.CounterBinder[S]` and closure
`BindCounter node.ClosureCounterBinder` run during normalization, not Build, once
per registered ID/physical instance with its bound `func(uint64)` updater. The
signatures are `func(*S, CounterID, func(uint64))` and
`func(CounterID, func(uint64))`. Each slot is fixed, atomic, replica-local, and
allocation-free to update; no labels, runtime registry growth, or worker lookups.
Updates wrap uint64 and become no-ops after completed stop.

Control-plane `Executor.Counter(NodeID, CounterID) (func(uint64), error)` resolves
one slot; `CounterForReplica(NodeID, uint32, CounterID)` selects an explicit
zero-based instance. Invalid executor uses ErrInvalidDefinition, absent slots
ErrUnknownCounter, ambiguous logical-only lookup ErrAmbiguousCounter. Snapshot
counter rows must be independently copied in deterministic physical-instance/counter
order.

## Processing and worker timing

```go
type TimingConfig struct {
    Clock func() uint64
    ClockSource TimingClockSource
    Mode TimingMode
    Every uint64
}
func SetTiming(*Builder, TimingConfig) error
type WorkerTimingConfig struct {
    Every uint64
    Clock func() uint64
    ClockSource TimingClockSource
}
func SetWorkerTiming(*Builder, WorkerTimingConfig) error
```

Both configurations are copied before Build and remain independent. After
invalid/sealed builder checks, SetTiming must reject unsupported modes, sampled
Every=0, and nonzero Every in full/disabled modes with ErrInvalidTiming. Unsupported
ClockSource is invalid; supported-enumeration TSC without processor support and
without a custom Clock uses ErrUnsupportedTimingClock. A second valid timing
configuration uses ErrTimingConfigured; invalid attempts store nothing.

| Collection | Sampling and measurement |
| --- | --- |
| `TimingDisabled` (default) | No handler timestamp reads, even with a configured custom Clock. |
| `TimingFull` | Every returned source/input handler, including failed returns. |
| `TimingSampled` | Calls 1, 1+Every, … independently per physical node. Every=3 measures 1,4,7. Unsampled calls read no timestamps. |
| Handler fields | `MeasuredCalls`, `MeasuredNanoseconds`, `MaxMeasuredNanoseconds`; entry/return clock pair around handler only, excluding selector, publication, retries, and logger work. |
| Worker Every=0 (default) | Disabled independently of handler timing; no worker clock reads. |
| Worker Every>0 | Sample full scheduler-pass wall duration; increment MeasuredPasses and add elapsed to IdleNanoseconds or WorkNanoseconds based on reported progress. Not OS CPU usage. |

Never extrapolate unmeasured work or enable exporters/logger timing as a side
effect. A custom Clock overrides ClockSource; nil uses the chosen built-in source.
Build and snapshot reads must not invoke it. Each measured returned handler uses
exactly two calls (100 then 140 means 40 ns); disabled/unsampled dispatch uses
none. Custom clocks must return modular monotonic nanoseconds with intervals
below 2^63, be concurrent-safe when shared, and not block or allocate. Duration is
unsigned subtraction, preserving modular wraparound; no reflection/boxed timestamps
or steady-state framework allocation is allowed.

## Clock-source decision and measured evidence

`TimingClockStandard` (zero/default) uses Go's monotonic clock. Optional
`TimingClockTSC` uses `hrtime.TSC`/`TSCSince(...).ApproxDuration()` and warms lazy
calibration before worker measurement. The current module **does depend on
`github.com/loov/hrtime v1.0.3`**; this is not a package-wide replacement for `time`
or its `time.Duration` APIs. A custom Clock still takes precedence.

Historical local measurements on **2026-10-26**, Go **1.26.8**, linux/amd64,
**Intel Core Ultra 7 255U**, used six one-second isolated samples summarized with
benchstat:

| Isolated clock closure | ns/op | allocs/op | Interpretation |
| --- | ---: | ---: | --- |
| `time.Since(epoch).Nanoseconds()` | 13.08 ±1% | 0 | Standard monotonic default. |
| `hrtime.Since(epoch).Nanoseconds()` | 23.76 ±1% | 0 | Slower here; researched non-Windows v1.0.4 implementation used wall time and lost monotonic semantics. |
| Calibrated TSC approximate nanoseconds | 6.478 ±1% | 0 | Faster isolated read, approximate and hardware-specific. |

Graph timing microbenchmarks measured about **46.0 ns/op** full/time versus
**32.8 ns/op** full/TSC, and sampled-64 about **14.5 versus 14.3 ns/op**, all
allocation-free. These are host/toolchain-specific historical results, not a new
run, application-level speedup, or portable suitability proof. The isolated
research inspected hrtime v1.0.4, distinct from the current v1.0.3 module pin.
Prefer standard timing unless representative workload evidence justifies full
TSC timing's approximate conversion; sampling already amortizes most clock cost.
Broader platform, cross-core monotonicity, units/calibration, wraparound, and
concurrency validation remain necessary. Do not substitute wall timestamps or
coarse cached clocks for monotonic elapsed measurement.

Primary references: [Go time/monotonic semantics](https://pkg.go.dev/time),
[time implementation](https://go.dev/src/time/time.go),
[testing benchmarks](https://pkg.go.dev/testing#B),
[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat), and
[hrtime v1.0.4 research source](https://github.com/loov/hrtime/tree/v1.0.4).

Instrumentation must preserve typed ownership, affinity, queue policies, and zero
framework allocations on the approved built-in steady-state paths. User callbacks
retain their stated obligations; snapshot creation, setup, and teardown are outside
those gates. [Performance conformance](05-planning-performance.md#benchmark-and-allocation-conformance)
defines the shared measurement rules.
