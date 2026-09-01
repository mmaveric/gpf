## Why

`node.Result[E]` already carries a typed failure value and node registration already reserves output port 9, but the executor currently drops every failure. Activating this path now makes failures observable by a deliberately connected typed consumer while retaining the fixed-group, zero-erasure execution model.

## What Changes

- Add an explicit `ErrorPort[E]` registration handle and `ConnectError[E]` API; ordinary `Connect` continues to accept only ordinary `OutPort[T]` values.
- Route a failed `Result[E]` to its connected typed error edge through generated monomorphized bindings; discard its value when that error port is unconnected.
- Materialize typed error-edge pools and built-in local or remote queues using the existing fixed-group classification and `EdgeConfig[E]` policy semantics.
- Extend graph validation, immutable definition copying, startup, shutdown, ownership cleanup, allocation gates, benchmarks, and race coverage for error edges.
- **BREAKING** Change `NodePorts.Errors()` from `OutPort[E]` to `ErrorPort[E]` and remove `ErrErrorRoutingUnavailable`; the previously returned error output could not be connected.

## Capabilities

### New Capabilities
- `typed-error-routing`: Explicit statically typed routing and ownership semantics for failed `Result[E]` values.

### Modified Capabilities
- `typed-pipeline-contracts`: Activate the reserved typed `Result[E]` output contract and execution boundary.
- `typed-graph-builder`: Return the distinct error-port handle from generated node registration.
- `typed-graph-connections`: Add explicit typed error-edge connection and binding semantics.
- `graph-dag-validation`: Include connected error edges in validation where required while preserving ordinary-data DAG rules.
- `immutable-graph-definition`: Copy typed error-edge topology and bindings into the immutable definition.
- `single-group-executor`: Publish failed results and clean up typed error transport on one-group execution.
- `manual-multigroup-runtime`: Classify and execute error edges over the existing local-or-remote transport model.
- `typed-node-output-ownership`: Define framework ownership for runtime-created error transport batches.
- `pipeline-performance-harness`: Measure supported error-routing steady-state paths.

## Impact

Affected packages are `graph`, generated graph bindings, `node`, `batch`, `queue`, and the benchmark/test harness. No dependency, reflection, type assertion, or dynamically typed data-plane registry is introduced; existing group planning, CPU affinity, pool-origin ownership, lifecycle, and remote SPSC semantics are extended rather than replaced.
