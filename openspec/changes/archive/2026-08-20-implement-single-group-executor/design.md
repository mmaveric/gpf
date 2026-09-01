## Context

Stage 2 seals an issue-free `Builder` into a reusable `Definition`. The definition owns dense topology and dormant monomorphized node and edge closures, but its `normalizationTarget` and `edgeBindingTarget` are empty and generated normalizers do not invoke a factory or expose an executable handler. `node.Outputs0` through `Outputs8` are currently phantom shape markers, so a handler cannot acquire or commit an output batch. `Batch[T]` identifies a slot but not its owning pool, which is insufficient when a compatible input batch moves across an edge.

The repository already has fixed-size batch pools, generated arities, per-edge queue configuration, and a typed `QueueOps[T]` continuation. This stage turns only those existing single-thread seams into one execution group. It must preserve the definition's structural immutability and Build dormancy, avoid data-plane type erasure, and leave all cross-group and observability work for later stages.

## Goals / Non-Goals

**Goals:**

- Start one executor from a validated immutable definition and materialize fresh state, closure replicas, pools, and queue operations for that start.
- Execute every supported generated explicit and closure node arity through monomorphized bindings on exactly one worker goroutine.
- Provide bounded local edge buffers, typed output acquisition and commit, deterministic ownership cleanup, and useful `Drop` and `Wait` behavior without data-plane synchronization.
- Define startup rollback, normal drain, forced close, sequential definition reuse, selector validation, and `Result` failure behavior.
- Establish zero-allocation steady-state tests and benchmarks after startup for framework-owned built-in paths.

**Non-Goals:**

- External ingress `Acquire`, `Commit`, or `Submit`; current node registration has no typed ingress endpoint to identify a safe external producer target.
- Cross-group or remote queues, atomics, multi-producer or concurrent queues, worker replicas, multiple execution groups, CPU affinity, or thread placement.
- Fan-out, broadcast, sharding, remote transport, dynamic topology, metrics, event logs, error ports, logging, retries, or error routing.
- Concurrent `Start`, concurrent lifecycle-method calls, custom callback allocation guarantees, panic recovery, or a persisted runtime format.

## Decisions

### Keep the executable boundary in `graph`

The public entry point is deliberately configuration-free:

```go
func (d *Definition) Start() (*Executor, error)

func (e *Executor) Drain() error
func (e *Executor) Close() error
```

`Executor` belongs to `graph`, alongside the private typed node and edge binding targets it must populate. A valid definition has its non-zero private owner and an issue-free Build snapshot. A nil or zero-value definition returns `ErrInvalidDefinition`; it never starts partially. `Start` performs all factory construction and initialization before publishing a running executor. It then owns one worker goroutine and one execution group. No runtime configuration, group ID, replica count, CPU selection, or timeout is accepted in this stage.

Definitions retain no active executor pointer or lifecycle status. Consequently, the existing sequential-reuse guarantee is implemented by constructing and fully stopping one executor before starting the next; concurrent starts from one definition remain unsupported rather than adding mutable state to immutable topology.

Putting `Start` in the existing `runtime` package was rejected because runtime must consume graph-private monomorphized binding targets, which would require exported type-erased hooks or a package cycle. Adding a public `Start` configuration was rejected because every proposed field describes a later multi-group concern.

### Materialize resources transactionally at startup

Startup proceeds in deterministic identity order:

1. For each edge by ascending `EdgeID`, construct its typed batch pool using `BatchCapacity` and `Capacity + 1` slots, then construct the configured queue operation. A nil `Factory` installs the built-in local bounded queue; a non-nil factory receives `queue.Config` and must return non-nil `TryPush`, `TryPop`, and `Len` functions.
2. For each node in the definition's canonical topological order, invoke exactly one explicit state or closure factory and use the generated normalizer to bind its typed handlers, outputs, input polls, and optional lifecycle callbacks into a runner.
3. Invoke each optional `Init` in the same canonical node order. Only after every init succeeds does the worker begin executing.

A factory error transfers no lifecycle ownership and prevents its `Init` or `Close`. A successful factory result transfers lifecycle ownership even when its later `Init` fails. Any queue-factory, node-factory, normalization, invalid-closure-replica, or init failure stops startup before a worker is visible, releases every constructed framework pool and queued/pending batch, and invokes every owned node `Close` once in reverse successful-construction order. The triggering error and cleanup errors are returned with `errors.Join`. `QueueOps` has no close callback in its current public contract, so a custom queue factory is responsible for any resources it creates; the executor drains its returned operations but cannot close opaque custom resources.

Calling Build-time factories early was rejected because it violates Stage 2 dormancy and would retain stale state across starts. Storing live resources on `Definition` was rejected because it breaks immutability and restart isolation. Adding a queue close callback was rejected as an unnecessary public contract expansion before remote custom queues have a lifecycle design.

### Normalize generated types into typed runners and typed outputs

The arity generator changes generated `Outputs1` through `Outputs8` from phantom-only structs into structs containing `Out1` through `Out8` values of a new generic `node.Output[T]`; `Outputs0` remains empty. `Output[T]` exposes exactly:

```go
func (o *Output[T]) Acquire() (batch.Batch[T], bool)
func (o *Output[T]) Commit(*batch.Batch[T]) bool
```

Runtime initialization installs private typed acquire, publish, and cleanup continuations into each output value. `Acquire` obtains at most one batch from that output edge's preallocated pool for an invocation. `Commit` accepts one valid compatible batch, moves and clears the caller's handle, and records it for post-handler publication; a second commit or invalid batch returns false without taking ownership. An acquired batch not committed by handler return is released automatically. This gives handlers a typed way both to construct transformed outputs and to transfer a compatible input without reflection, casts, or boxed batches.

Generated explicit and closure normalizers create one persistent typed runner per node. A zero-input runner invokes its source handler once per scheduler pass while sources are enabled. A one-input runner invokes its only handler when its input edge has a batch. A multi-input runner forms a `ReadyMask`, invokes its selector only if two or more positions are ready, and dispatches exactly the selected typed handler. A selector result outside the declared input arity or absent from the ready mask is a contract violation and panics; panics are not recovered. The runner owns no generic `any` data-plane registry and all per-invocation values are stack-resident or fields preallocated during startup.

Using pre-acquired output batches was rejected because handlers need a way to produce no batch and could otherwise publish unused capacity. Giving handlers direct pool references was rejected because it exposes runtime-owned allocation policy and makes typed output routing unclear. A dynamic dispatcher over erased batch values was rejected because it would negate the generated handler contract.

### Preserve origin-aware batch ownership and publish after handler return

`Batch[T]` gains `func (b *Batch[T]) Release()`, which returns a non-empty handle to the pool that originally allocated its slot and clears the handle. Pool slot metadata retains that private pool association. `Pool.Release` remains the direct owner-pool API, while the executor uses `Batch.Release` whenever it must release an un-moved input, a dropped output, an abandoned acquired output, or a drained queue entry. This follows a moved batch across compatible edges without asking the executor to recover a generic source pool.

Every normal handler invocation follows one ownership boundary. On a successful `Result`, each committed output is offered to its one outgoing edge, and an un-moved input is released after output publication is staged. On `Result{Failed: true}`, the executor releases the un-moved input and every committed or acquired output, publishes none of them, and performs no error routing. `Result.Value` remains the typed value returned by the handler but is not boxed, logged, exported, or sent to the reserved error port. A sink has no ordinary outputs and therefore releases every un-moved input after its handler returns.

Extending `EdgeConfig[T]` and `queue.Config` with required `BatchCapacity uint32` supplies the missing fixed batch size. Validation rejects zero queue capacity, zero batch capacity, and invalid policy. Per edge, the built-in path creates a pool with `Capacity + 1` slots: enough for every bounded queued batch plus one producer-held batch, with overflow checked before allocation.

Returning an input to the edge it is currently traversing was rejected because a handler can move an upstream batch into a downstream compatible edge. Leaving cleanup to every handler was rejected because an omitted move would leak an owned pool slot and violate the existing automatic-release contract.

### Use bounded local queues and progress-preserving policy handling

The nil-factory queue is a typed ring with preallocated `[]batch.Batch[T]`, head, tail, and count. Only the executor worker calls it, so it uses no atomics, mutexes, channels, or allocations. A configured custom `QueueOps[T]` is called only by that same worker and must uphold its existing non-blocking, allocation-free contract.

Publishing an output calls `TryPush`. On success ownership transfers to the edge queue. With `queue.Drop`, a false return releases the output immediately and that output is discarded. With `queue.Wait`, the runner retains the one pending output and is not invoked again until it publishes it. The scheduler continues scanning other runners, so a downstream consumer can free capacity; it retries each retained output on later passes instead of tight-spinning inside the producing handler. This is bounded, single-threaded busy polling without the deadlock that an immediate wait loop would cause on a full local edge.

`Drain` first disables zero-input source activation, then lets the worker process and retry all queued and pending batches until no runner has work. It drains and releases every remaining batch from every queue before closing lifecycles. `Close` also disables sources but discards queued and pending work by releasing it, then closes lifecycles without waiting for normal completion. Both methods wait for the worker teardown and are idempotent after stop; concurrent lifecycle calls are unsupported. There is no timeout because a timeout policy without ingress, remote queues, or metrics would add a misleading control surface.

Using a channel for local edges was rejected because it would make capacity and ownership policy opaque and could introduce data-path allocation or blocking. Immediate busy-spin on `Wait` was rejected because a single worker could never run the consumer that frees the full queue. Cross-group queue implementations are expressly deferred.

### Deliberately defer ingress and preserve a measurable data plane

The current public graph exposes only `AddNode0` source nodes and no typed ingress registration handle. Adding `Acquire`/`Commit`/`Submit` now would require choosing which source or edge owns an external producer and adding control-plane provenance absent from the current API. Source handlers therefore remain the only producer mechanism in this stage; they acquire output batches through their typed `node.Output[T]` values.

After `Start` setup, the built-in local source-to-sink, single-input, multi-input selector, output acquire/commit, release, queue-pop/push, drop, and wait-retry paths must have zero framework allocations. Tests use `testing.AllocsPerRun`; benchmarks measure only the steady-state worker path after setup. User handlers, selectors, factories, and custom queues are outside this guarantee because the framework cannot prove their allocation behavior.

Adding an ingress-only API without a typed binding was rejected because it would either route by erased runtime IDs or create an unvalidated ownership path. Adding metrics to observe drops was rejected because metrics belong to the later observability stage; tests can observe release and handler effects directly.

## Risks / Trade-offs

- [A continuously runnable source can produce indefinitely] -> `Drain` and `Close` disable all zero-input runners before draining or discarding work; source authors must use bounded output acquisition correctly.
- [A custom `QueueOps` factory allocates, blocks, leaks resources, or violates FIFO semantics] -> Validate required callbacks at startup, call it from one worker only, document its responsibility, and keep the built-in path as the zero-allocation reference.
- [A handler abandons an acquired output] -> Track one acquisition per output and release it automatically at the post-handler ownership boundary.
- [A handler panics or double-moves a batch] -> Preserve the existing no-runtime-ownership-check and no-panic-recovery contracts; tests cover correct ownership only.
- [A `Wait` queue never drains because downstream logic never consumes] -> Busy polling continues by design; no timeout is introduced in this scope.
- [Generated normalization drifts across arities] -> Generate all runner and output bindings from the existing deterministic generator and add arity boundary and regeneration tests.

## Migration Plan

1. Add origin-aware batch release, output ownership types, `BatchCapacity`, and updated documentation; regenerate and verify committed arity files.
2. Extend the private node and edge binding targets and generate monomorphized explicit and closure runner normalizers.
3. Implement local queues, edge pools, executor startup rollback, dispatch, publish/release behavior, and lifecycle control entirely in `graph`.
4. Add focused ownership, lifecycle, dispatch, queue-policy, generated-code, and allocation tests and benchmarks, then update project documentation to state the exact Stage 3 boundary.
5. Run formatting, generation verification, unit and race tests, vet, allocation gates, diff checks, and strict OpenSpec validation.

There is no deployed executor or persisted runtime state. Rollback removes the executor and its additive output/lifecycle APIs; the Stage 2 definition remains buildable and factory-dormant. Callers must add `BatchCapacity` to every `EdgeConfig` before adopting the executable API.

## Open Questions

None. Ingress is intentionally deferred until registration introduces a typed ingress binding, and timeout, affinity, custom-queue teardown, metrics, and error routing remain later dedicated designs.
