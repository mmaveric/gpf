## Context

See proposal.md for motivation. `graph/executor.go` materializes typed edge pools and queue operations before normalizing generated runners, then starts one locked worker per immutable group. Generated handlers know the concrete input and error types; the scheduler sees only callbacks and numeric metadata. Pool acquisition and output finalization are the ownership boundaries, and shutdown already pauses every worker before discarding pending and queued batches.

## Goals / Non-Goals

**Goals:**
- Make outcome counts precise at completed operations while allowing nonblocking, race-safe live observations.
- Keep metrics storage runtime-local, bounded by graph size, and independent of batch contents.
- Preserve queue callback affinity, cleanup order, generated typing, and zero-allocation processing.

**Non-Goals:**
- Timing, timestamps, custom user metrics, exporters, logging, error mapping, pool-usage gauges, and an enable/disable configuration.
- A globally transactional live snapshot, reliable error delivery, handler retries, or new lifecycle concurrency guarantees.

## Decisions

### Numeric snapshots belong to the executor

Expose `func (e *Executor) Metrics() MetricsSnapshot` with `Groups []GroupMetrics`, `Nodes []NodeMetrics`, and `Edges []EdgeMetrics`. Each slice is copied and sorted by its numeric identity. Nodes carry NodeID and GroupID; edges carry EdgeID, source/target NodeID and GroupID, and ErrorEdge/Remote flags. Group rows contain scheduler Passes and IdlePasses. Other exact fields and semantics are defined in the runtime-metrics spec.

No aggregate total is exposed: summing node input elements and edge transfer elements mixes different boundaries and can double-count a moved batch. Callers choose the aggregation appropriate to their graph. A separate metrics package or export library would add a seam without hiding meaningful complexity at this stage.

### Separate update ownership and use atomic numeric publication

Start allocates metric records before binding callbacks. Generated runners capture their node record; typed edge closures capture producer and consumer records. Each hot counter has one logical writer during processing. Use Go typed atomics for race-safe publication, with padded producer/consumer storage so remote workers do not repeatedly write the same cache region. Padding is an implementation isolation aid, not a portable assertion about every machine's cache-line size.

Snapshots load atomics only; they do not send worker commands, acquire locks, inspect batch contents, or call queue operations. Live loads may describe different instants. Exact cumulative counts are guaranteed once processing and cleanup have completed. Counters wrap modulo 2^64 rather than add saturation branches. Atomic instrumentation is always on so the behavior and allocation gates are exercised by default.

Worker pause/copy snapshots were rejected because repeated observations would interrupt busy-poll processing and compete with lifecycle commands. Unsynchronized plain counters were rejected because live reads would race.

### Count at semantic boundaries, not scheduler attempts

Generated code increments HandlerCalls after a handler returns; input Batches/Elements are captured before a handler can move or release its handle. A source invocation contributes a handler call but no synthetic input batch. FailedResults increments once for each failed return, and UnconnectedErrors once when that failure has no route.

Each edge counts accepted publications, handler dequeues, queue-policy drops, Wait retry attempts, pool-exhaustion acquisition attempts, and forced-discard batches/elements separately. WaitRetries counts every subsequent TryPush attempt for a retained output, including the successful one, but not the initial attempt. Error publication counts are visible on error-edge AcceptedBatches; no second node-level routed-error count is necessary.

Acquisition wrappers count only genuine pool unavailability, not duplicate acquisition rejected by Output's per-invocation guard. Generated failure finalizers distinguish unconnected failures from connected acquisition failures. Capture batch lengths before finalizers clear handles. A failed handler's staged ordinary output is not a queue drop; add edge FailedOutputBatches/Elements only for committed ordinary outputs discarded by failed finalization. Uncommitted scratch acquisitions are releases, not delivered-work losses.

Force Close counts queued and pending committed outputs as ShutdownDiscardedBatches/Elements, including pending error batches, exactly once. Dequeue for cleanup is not a handler dequeue. Successful Drain keeps normal processing counts and finishes with empty queues. Startup rollback has no externally published executor snapshot.

### Queue occupancy is sampled, not transactional

Sample local/custom queues only on the owner worker after push/pop operations. For remote queues, producer samples after pushes and consumer samples after pops; readers see bounded published samples, not reconstructed subtraction of independently read totals. Clamp samples to configured capacity. Maintain a sampled high-water maximum from these samples, using atomic max publication when both workers update it. Sampling can miss peaks; names explicitly say QueueDepthSample and QueueHighWaterSample.

Post-stop queue samples are set to zero at the existing all-worker cleanup barrier; high-water samples remain retained. Snapshots never call custom Len from another goroutine. A new exact concurrent occupancy counter would add cross-worker contention and publication ordering complexity without making live snapshots globally consistent.

### Preserve snapshots through lifecycle without expanding lifecycle synchronization

Metrics records and immutable row metadata survive runner teardown. Metrics is safe concurrently with worker processing, other Metrics calls, and the single supported Drain or Close operation; it reads only retained atomic records and immutable metadata, never the non-atomic stopped flag or mutable runner slices. Shutdown accounting occurs only after workers pause, followed by atomic publication. Definitions retain no records; sequential Start creates new zeroed records while the prior executor retains final values.

## Risks / Trade-offs

- [Atomic updates increase hot-path instruction count] → Run existing local/remote/error benchmarks before and after instrumentation; report throughput and allocations without inventing a threshold.
- [Moved input handles or retried outputs get double-counted] → Record input length before dispatch and tie publication accounting to successful ownership transfer; test failed, moved, Drop, Wait, and shutdown paths.
- [Live snapshot fields appear inconsistent] → Document independently sampled values and verify exact totals after stop rather than demand cross-field live equations.
- [Custom queue Len is unsafe outside its worker] → Publish samples from existing typed callbacks only; verify callback ownership in tests.
- [Cache contention undermines the performance goal] → Separate producer and consumer records, avoid a global aggregate counter, and benchmark remote handoff.

## Migration Plan

Add numeric types and runtime-local records, wire generated dispatch and typed transport, add focused correctness/race/allocation coverage, then update documentation. Public processing and lifecycle signatures are unchanged. Rollback removes the additive snapshot API and instrumentation; there is no stored graph format or dependency migration.
