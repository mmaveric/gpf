## Context

The node contract already returns `node.Result[E]`, and graph registration reserves output `PortID(9)`, but that handle is currently an ordinary `OutPort[E]` rejected by `Connect`. The executor discards `Result.Value` on every failed result. Ordinary edges already retain monomorphized pool and queue bindings, use immutable fixed group placement and CPU bindings, and select a local one-worker queue or framework-owned remote SPSC ring at Build.

This change activates only the reserved per-node error path. It must keep the node error type concrete from registration through generated execution binding, pool, queue, and input handler. Control-plane metadata can remain non-generic, but no data-plane value may enter `any`, `interface{}`, reflection, a type assertion, or a runtime type registry.

## Goals / Non-Goals

**Goals:**

- Make failure routing explicit and compile-time typed for each node's existing `E` parameter.
- Preserve one optional, single-consumer error output per node and one producer per target input.
- Use the current bounded pool, local queue, remote SPSC, Drop, Wait, fixed-group, CPU-affinity, startup, rollback, Drain, and Close semantics for error edges.
- Define exact cleanup when a result fails, including no-connected-edge drop, pool exhaustion, queue Drop, Wait retry, rollback, Drain, and Close.
- Add generated-binding, functional, allocation, benchmark, and race coverage for local and cross-group error delivery.

**Non-Goals:**

- `ErrorMapper`, error logging, event logs, metrics, retries, panic recovery, error fan-out, error distribution, multiple error-port consumers, ingress, auto grouping, replicas, sharding, or any other queue model.
- Error aggregation or batching: one failed handler invocation produces at most one one-element `batch.Batch[E]`.
- Changes to public Start, placement, CPU-binding, lifecycle, or queue-factory APIs beyond the typed error connection surface.

## Decisions

### Use a distinct opaque error handle and an explicit connection function

Registration will return `ErrorPort[E]` from `NodePorts.Errors()` instead of `OutPort[E]`. `ErrorPort[E]` has the same read-only identity accessors as other port handles, retains the private owner and `PortID(9)`, and cannot be converted to `OutPort[E]` by callers. The graph will expose:

```go
func ConnectError[E any](
    builder *Builder,
    output ErrorPort[E],
    input InPort[E],
    config EdgeConfig[E],
) (EdgeID, error)
```

Ordinary `Connect` remains `OutPort[T]`-only. `ConnectError` shares port provenance, builder state, cardinality, config validation, dense edge-ID allocation, and factory dormancy rules with `Connect`, but records an error-edge kind. Its generic type parameter binds the handler's `E` to the target input type at compile time. A type mismatch cannot compile; an error route is also visually explicit at the call site.

Allowing ordinary `Connect` to accept the reserved port was rejected because an error route would be indistinguishable from an ordinary output connection. Keeping `Errors() OutPort[E]` plus a runtime metadata check was rejected because it cannot prevent accidental routing in the public type system. The access type changes even though it is technically breaking because the old error handle had no usable connection path.

### Treat error edges as typed batch transport with one error per batch

An error edge uses its existing `EdgeConfig[E]` to construct a typed `batch.Pool[E]` and `QueueOps[E]` exactly as an ordinary edge does. On a failed result with a connected error port, the generated binding acquires one `batch.Batch[E]` from that edge's source pool, stores `Result.Value` at index zero, sets length one, and publishes it. `BatchCapacity` still configures the preallocated storage but this stage never aggregates failures; each delivered batch has length one.

When the error port is unconnected, the executor discards `Result.Value` without allocating an error batch or creating a runtime error resource. When its pool is exhausted, there is no owned error batch and the failure value is dropped. Once acquired, the error batch follows the existing exact ownership rules: successful queue push transfers ownership, Drop releases once, Wait retains one pending batch and blocks only that runner from a new handler invocation, and rollback, Close, and queue draining release it to its originating pool. The consuming error handler releases an un-moved batch normally or may move it to a compatible ordinary output.

Introducing an unpooled direct `E` queue was rejected because it would fork queue and cleanup semantics, weaken pool-origin ownership, and add a distinct remote transport. Holding a failed `Result` itself until a pool becomes available was rejected because `Result` has no ownership or retry contract and would allow unbounded user-value retention.

### Generated bindings own failure finalization and remain fully typed

The arity generator will extend each explicit and closure normalizer with a typed error-edge finalizer for its concrete `E`. After every handler return, that finalizer first discards all ordinary acquired or committed outputs on failure, preserves already moved input batches, and releases all remaining input batches. It then either drops the unconnected result value or stages/publishes exactly one typed error batch.

A pending ordinary Wait output continues to be retried before another handler call. A pending error Wait batch has the same priority and also prevents another call to that runner. This preserves input release and avoids producing a second failure while the first has not transferred ownership. Generated code will use typed `batch.Batch[E]`, pool acquisition, queue operations, and finalizer closures directly. The implementation must remove or avoid reflection and runtime type assertions from these data paths rather than adapting a dynamically typed output-binding registry.

Giving handlers an error-output `node.Output[E]` was rejected because a handler already returns its sole failure value and adding a second error-output ownership protocol permits contradictory outcomes. Publishing ordinary outputs alongside a failure was rejected to preserve the current all-or-nothing failure boundary.

### Error edges participate in structural execution analysis but remain optional outputs

An error edge is a structural edge from its source node to its target node. Validation and Build include connected error edges in edge IDs, adjacency snapshots, DAG ordering and cycle detection, reachability, copied immutable topology, fixed group classification, and runtime materialization. This ensures an error consumer is scheduled after its potential producer and prevents error-routing cycles from creating uncontrolled failure feedback.

An error port is optional: validation never reports an unconnected error output. Ordinary output connectivity remains unchanged, and an input connected by either an ordinary or error edge satisfies its exactly-one-producer requirement. Error edges may target only ordinary typed inputs; a node cannot consume errors through a separate input kind in this stage.

Excluding error edges from topology was rejected because an error-only consumer would appear unreachable and a cross-group handoff would lack a deterministic group classification. Requiring every error port to connect was rejected because failure observation must be opt-in and most graphs have no error sink.

### Reuse fixed-plan transport and lifecycle semantics unchanged

Build classifies each error edge local or remote using its endpoint groups. Local error edges retain nil-factory built-in queues or a fresh `queue.Factory[E]`; remote error edges use only the framework-owned typed SPSC ring and reject custom factories before invocation. All workers stay bound to their immutable CPUs, and no worker, pool, queue, factory result, or error value is retained by `Definition`.

Startup materializes ordinary and error edges in ascending shared `EdgeID` order. Rollback, Drain, and Close inspect and release ordinary and error queues and pending outputs through the existing global pause barrier and reverse canonical lifecycle ordering. This preserves one producer and one consumer for every remote SPSC edge, including error routes.

Adding a dedicated error worker, queue policy, lifecycle, or CPU binding was rejected because it would undermine the established fixed-plan ownership and create new concurrency models.

## Risks / Trade-offs

- [A failed value is lost when its error port is unconnected, its pool is exhausted, or a Drop queue is full] -> The behavior is explicit, non-blocking, and tested; reliable delivery and retry are deferred.
- [A connected error route makes an otherwise valid topology cyclic] -> Validation includes error edges in canonical DAG analysis and rejects the graph before Build.
- [Generic generated code regresses to reflection or boxed values] -> Generated-source audits, compile-fail tests, allocation gates, and byte-for-byte generation tests cover every arity and both registration forms.
- [An error Wait edge stalls normal processing unexpectedly] -> Only its producing runner pauses until publication; same-group and remote consumers continue under existing scheduler rules.
- [Remote error transport violates SPSC ownership] -> Each error port has one consumer, each edge has its source group as sole producer and target group as sole consumer, and race tests cover the handoff and lifecycle barriers.

## Migration Plan

1. Add `ErrorPort[E]`, update generated registration return values, and replace the unavailable error-route API with `ConnectError[E]`.
2. Extend validation, immutable copying, fixed-plan classification, and generated typed bindings for error-edge metadata and transport.
3. Implement failed-result cleanup and error publishing, then extend startup, rollback, Drain, and Close resource cleanup.
4. Add compile, unit, integration, allocation, benchmark, race, documentation, generated-output, and strict OpenSpec validation gates.

There is no persisted graph format or deployed definition to migrate. Rollback removes the new connection API and routing behavior; applications that did not use the formerly unavailable error handle retain ordinary graph behavior.

## Open Questions

None. Reliability, mapping, observability, and recovery policies remain intentionally deferred.
