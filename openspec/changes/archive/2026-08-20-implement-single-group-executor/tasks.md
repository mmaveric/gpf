## 1. Batch, Edge, and Output Contracts

- [x] 1.1 Add origin-pool metadata and `func (b *Batch[T]) Release()` so a moved typed batch can return to its allocating pool and clear its handle without type erasure.
- [x] 1.2 Preserve direct `Pool.Release` behavior, update batch documentation, and add lifecycle tests for origin release after Move and zero or empty handles.
- [x] 1.3 Add required `BatchCapacity uint32` to `graph.EdgeConfig[T]` and `queue.Config`, propagate it through Connect and future edge binding, and update the shared edge validator to reject zero batch capacity.
- [x] 1.4 Update connection validation, compile fixtures, examples, and tests for the breaking edge configuration while preserving dense IDs and atomic rejected connections.
- [x] 1.5 Add `node.Output[T]` with exact `Acquire` and `Commit` methods, private runtime installation hooks, single-acquisition and single-commit state, and zero-value failure behavior.
- [x] 1.6 Extend the deterministic arity generator so `Outputs1` through `Outputs8` expose correctly typed `Out1` through `Out8` fields and regenerate committed node output code.
- [x] 1.7 Add generated-shape compile tests and output ownership tests for typed acquisition, input move, duplicate commit rejection, and automatic abandonment cleanup.

## 2. Typed Runtime Binding Generation

- [x] 2.1 Expand private edge binding targets to install edge-owned pool acquisition, origin-aware release, local or custom queue operations, and endpoint metadata without exposing erased data-plane values.
- [x] 2.2 Expand private node normalization targets to register persistent runners, typed input polls, typed output handles, lifecycle callbacks, and source enablement state.
- [x] 2.3 Generate explicit-state normalizers for arities zero through eight that invoke one factory at Start, bind every typed handler, and retain state only in the executor.
- [x] 2.4 Generate closure normalizers for arities zero through eight that invoke one closure factory at Start and reject nil required returned handlers or selectors before the worker starts.
- [x] 2.5 Generate per-arity source, one-input, and multi-input runner behavior, including ready-mask construction, selector-only-on-contention behavior, and panic on invalid selector results.
- [x] 2.6 Regenerate all generated files and extend generator determinism and arity-boundary tests to cover executable binding output.

## 3. Single-Worker Edge and Publication Path

- [x] 3.1 Implement a typed preallocated local ring with single-worker `TryPush`, `TryPop`, `Len`, and queue-drain operations using no atomics, mutexes, channels, or steady-state allocations.
- [x] 3.2 Construct each edge's pool with checked `Capacity + 1` slot arithmetic and configured batch capacity, then bind a nil factory to the local ring.
- [x] 3.3 Invoke non-nil custom queue factories once per started edge, pass complete queue configuration, validate all required QueueOps callbacks, and retain no custom operations on the definition.
- [x] 3.4 Implement post-handler output staging and publication: transfer on successful push, release on Drop, retain one pending output on Wait, and retry pending work before a runner accepts new input.
- [x] 3.5 Implement automatic release for un-moved inputs, uncommitted acquisitions, failed results, dropped outputs, queue-drain entries, and forced-close entries through `Batch.Release`.
- [x] 3.6 Add focused local-ring and custom-QueueOps tests for capacity, FIFO delivery, callback validation, Drop release, Wait retry progress, and no duplicate release.

## 4. Executor Startup and Lifecycle

- [x] 4.1 Add `ErrInvalidDefinition`, `ErrInvalidQueueOps`, opaque `Executor`, and exact `Definition.Start`, `Executor.Drain`, and `Executor.Close` public APIs in `graph`.
- [x] 4.2 Implement deterministic startup in edge-ID then canonical-node order: pool and queue construction, node factory normalization, and lifecycle Init before worker publication.
- [x] 4.3 Implement startup rollback that drains framework queues, releases pending batches, closes every successfully owned lifecycle in reverse construction order, and returns joined triggering and close errors.
- [x] 4.4 Implement the one-worker scheduler scan with source activation, input readiness, per-arity dispatch, pending-output retries, and no data-plane reflection, assertions, `any`, interfaces, channels, mutexes, or atomics.
- [x] 4.5 Apply `Result{Failed: true}` behavior by suppressing ordinary output publication and releasing owned batches without error routing, logging, events, retries, or metrics.
- [x] 4.6 Implement graceful Drain by disabling sources, processing accepted local work through quiescence, draining leftovers, and closing lifecycles once in reverse order.
- [x] 4.7 Implement forced Close by disabling sources, releasing queued and pending work without further handler execution, closing lifecycles once, and making stopped calls idempotent.

## 5. Contract Coverage and Performance Gates

- [x] 5.1 Add end-to-end explicit-state source, transform, and sink tests that verify typed output acquire/commit, moved-input ownership, source-pool release, and sequential definition reuse.
- [x] 5.2 Add end-to-end closure node tests covering fresh replica state, required returned handler validation, init and close ownership, and rollback after factory or init failure.
- [x] 5.3 Add multi-input heterogeneous dispatch tests for each selector boundary, only-ready selector invocation, invalid selector panic, and preservation of unselected ready input.
- [x] 5.4 Add lifecycle tests for invalid definitions, Build factory dormancy, successful Start materialization, custom queue factory invocation, graceful drain, forced close, close errors, and no concurrent-start promise.
- [x] 5.5 Add allocation gates with `testing.AllocsPerRun` and post-start benchmarks for built-in source-to-sink, one-input, multi-input, acquire/commit, Drop, and Wait retry paths.
- [x] 5.6 Update README and graph, node, batch, queue, and runtime documentation to state exact APIs, ownership, Result failure, rollback, queue policy, drain/close, sequential reuse, and deferred ingress and hybrid features.
- [x] 5.7 Run `gofmt` and generation verification on changed Go files; run `go test ./...`, `go test -race ./...`, `go vet ./...`, allocation tests, benchmarks, and `git diff --check`.
- [x] 5.8 Run `openspec validate implement-single-group-executor --strict` and `openspec validate --all --strict`, resolving validation failures attributable to this change.
