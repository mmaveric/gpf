## Why

Stage 2 produces a validated immutable `Definition`, but it cannot materialize its dormant factories or execute a batch through its typed handlers. A deliberately narrow single-group executor is the first executable data-plane boundary and validates the ownership and generated-binding design before any cross-group concurrency is introduced.

## What Changes

- Add one single-worker executor created from a valid immutable `Definition`, with startup, graceful drain, forced close, deterministic rollback, and sequential restart behavior.
- Materialize explicit-state factories, closure factories, and per-edge queue factories only at executor startup; run optional initialization and close callbacks with defined ownership and rollback ordering.
- Replace inert generated normalization with monomorphized runner bindings for every generated node arity, including source handlers, one-ready-input dispatch, multi-input ready masks and selectors, typed `Result` handling, and typed outputs.
- Add a built-in bounded local edge queue used only by the one worker, and execute configured custom `QueueOps` on that same worker. Define `Drop` and progress-preserving `Wait` behavior without atomics, channels, or cross-group queues in the data path.
- Add typed output acquisition and commit handles so handlers can create or move batches, then define automatic publish, drop, and source-pool release behavior. Extend edge configuration with the batch capacity needed to construct edge-owned pools. **BREAKING**: `EdgeConfig` validation and queue configuration gain required batch-capacity semantics.
- Add the minimum batch ownership operation needed for a runtime to return an un-moved or dropped batch to its originating pool without type erasure.
- Do not add external ingress `Acquire`/`Commit`/`Submit`: current registration exposes only zero-input source nodes, not a typed ingress endpoint to bind such an API safely.
- Add functional, lifecycle, ownership, failure, selector, queue-policy, zero-allocation, and generated-code coverage for the executable path.

## Capabilities

### New Capabilities
- `single-group-executor`: One-worker definition execution, startup materialization and rollback, generated typed dispatch, local queue policy, output publication, batch release, drain, and close semantics.
- `typed-node-output-ownership`: Typed handler output acquisition and commit API with explicit ownership transfer and automatic cleanup.

### Modified Capabilities
- `immutable-graph-definition`: Permit a valid immutable definition to create one sequential single-group executor while retaining topology immutability and Build dormancy.
- `typed-pipeline-contracts`: Make the batch, output, edge configuration, queue, and node-result contracts executable for the bounded one-group data path; replace the deferred-execution statements for this scope.

## Impact

- Affects `graph` definition bindings, generated registration/normalization code, and new executor internals; `node` output shapes and batch ownership; and the `queue` configuration contract.
- Uses existing typed `Stateful`, `ClosureFactory`, `QueueOps`, `Result`, `Batch`, and immutable-definition contracts without reflection, runtime type assertions, or boxed data-plane values.
- Updates runtime, graph, node, queue, batch, README, and performance documentation to distinguish this one-worker local executor from deferred hybrid execution.
- Excludes cross-group queues, atomics, multi-producer queues, CPU affinity, multiple execution groups, remote transport, metrics/event logging, error routing, and ingress APIs.
