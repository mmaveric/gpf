## 1. Typed Error Connection Contracts

- [x] 1.1 Add opaque `ErrorPort[E]` with the required identity accessors, update `NodePorts.Errors()` and zero-value behavior, and remove the unavailable-routing API and sentinel.
- [x] 1.2 Extend the graph arity generator and its manifest so every explicit and closure `AddNode0` through `AddNode8` registration creates the distinct `ErrorPort[E]` at `PortID(9)`.
- [x] 1.3 Regenerate and commit graph outputs; extend generated-output stability tests and compile tests for all registration arities and zero-value error handles.
- [x] 1.4 Add compile-fail fixtures proving `ErrorPort[E]` cannot be passed to ordinary `Connect` and `ConnectError` rejects mismatched `E` and input types at compile time.
- [x] 1.5 Implement `ConnectError[E]` with shared typed `EdgeConfig[E]` validation, dense cross-kind edge IDs, dormant monomorphized bindings, and no factory invocation.
- [x] 1.6 Implement deterministic `ConnectError` validation precedence, shared input/output cardinality, atomic rejection, stable errors, and removal of `ErrErrorRoutingUnavailable` handling.
- [x] 1.7 Add focused connection tests for valid local error routes, invalid and foreign handles, cross-kind input conflicts, error fan-out rejection, configuration failures, error precedence, factory dormancy, and edge-ID density.

## 2. Validation and Immutable Planning

- [x] 2.1 Add an explicit private error-edge classification to structural metadata without storing an error value, dynamic data-plane value, reflection value, or type-erased queue operation.
- [x] 2.2 Extend validation snapshots, canonical ordering, cycle witnesses, and source-to-sink reachability to include connected error edges while keeping error outputs optional and ordinary output connectivity unchanged.
- [x] 2.3 Extend validation tests for error-fed inputs, unconnected error outputs, ordinary/error topology ordering, error-edge cycles, reachability, sorted diagnostics, and dormant local queue factories.
- [x] 2.4 Copy error-edge metadata and monomorphized bindings into `Definition`; preserve owner identity, structural immutability, cached Build behavior, and no Build-time pool, queue, worker, factory, or error-value creation.
- [x] 2.5 Classify local and remote error edges from immutable placements, reject remote error queue factories during Validate/Build without invocation, and preserve sparse groups and CPU-binding policy.
- [x] 2.6 Add Build and definition tests for shared edge ordering, immutable ordinary/error topology copies, local-versus-remote error classification, remote factory rejection, sequential definition reuse, and CPU-affinity-plan preservation.

## 3. Generated Typed Runtime Binding

- [x] 3.1 Refactor or extend generated runtime bindings so ordinary and error data paths use concrete typed closures and numeric metadata only; remove reflection, runtime type assertions, `any`, and `interface{}` data-plane transport from the affected binding and dispatch paths.
- [x] 3.2 Bind each connected error edge to a fresh typed `batch.Pool[E]` and local built-in/custom `QueueOps[E]` or framework-owned remote SPSC queue using its existing `EdgeConfig[E]`.
- [x] 3.3 Generate explicit-state and closure finalizers for every input arity that discard failed ordinary outputs, preserve moved inputs, release remaining inputs, and publish exactly one typed error batch only on failure.
- [x] 3.4 Implement exact error-result ownership: unconnected and pool-exhausted errors drop without retained state; accepted queue ownership transfers; Drop releases once; Wait retains one typed error batch and blocks only that runner until retry succeeds.
- [x] 3.5 Preserve batch origin-pool release for local and remote error batches, including moved consumer inputs, startup rollback, queue drains, pending Wait cleanup, Drain, and Close.
- [x] 3.6 Preserve startup materialization in shared EdgeID order, global lifecycle ordering, one locked CPU-bound worker per group, affinity outcomes, and fixed group-local scheduling with error edges.

## 4. Functional and Lifecycle Coverage

- [x] 4.1 Add explicit-state and closure end-to-end tests for local typed error delivery, unconnected failure drop, successful-result non-delivery, and failed-result ordinary output/input cleanup.
- [x] 4.2 Add tests for one-element error batch contents, configured batch capacity without aggregation, source-pool exhaustion drop, origin-aware release, consumer move to a compatible ordinary output, and exact once-only release.
- [x] 4.3 Add local queue tests for error Drop and Wait, pending-runner suppression, retry ordering, downstream progress, custom local `Factory[E]` freshness, and invalid custom queue operations.
- [x] 4.4 Add remote SPSC tests for typed error FIFO transfer, capacity, Drop, Wait retry, no remote factory invocation, two-or-more fixed groups, CPU-affinity fallback, and one-group equivalence.
- [x] 4.5 Add startup failure, Init failure, Drain, Close, and sequential Start tests proving error pools, queues, and pending batches are released and lifecycle callbacks retain reverse global canonical owner-worker order.
- [x] 4.6 Run affected local/remote transport, startup, affinity, pause-barrier, Drain, and Close suites with `go test -race ./...`; resolve data races and goroutine leaks without adding unsupported concurrent Start or lifecycle semantics.

## 5. Performance, Documentation, and Verification

- [x] 5.1 Add post-Start `testing.AllocsPerRun` gates for local failure-to-error-sink delivery, error Drop, error Wait retry, remote SPSC error transfer, and a two-group error-routing DAG.
- [x] 5.2 Extend the benchmark matrix with `ReportAllocs` local and remote error-routing cases whose names encode batch capacity and group count and exclude Build and Start setup from timed work.
- [x] 5.3 Review generated and handwritten affected data paths to prove no reflection, runtime type assertion, boxed data value, dynamic type registry, blocking data-plane channel, mutex, or allocation was introduced.
- [x] 5.4 Update README and graph, node, batch, queue, and runtime documentation with explicit `ErrorPort`/`ConnectError` use, failure drop and ownership semantics, local/remote behavior, and deferred reliability and observability features.
- [x] 5.5 Run generation verification, `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, relevant allocation tests and benchmarks, and `git diff --check`.
- [x] 5.6 Verify every proposal, design, spec delta, and task is complete; run `openspec validate implement-typed-error-routing --strict` and `openspec validate --all --strict`, resolving failures attributable to this change.
