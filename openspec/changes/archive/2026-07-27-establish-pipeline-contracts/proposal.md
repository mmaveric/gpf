## Why

The pipeline framework plan requires strict, allocation-free data-plane behavior and statically typed APIs, but the repository has no executable contracts or performance baseline. Establishing them first prevents later batch, graph, and scheduler implementations from drifting from the intended public model or introducing hidden allocations.

## What Changes

- Initialize the Go module and package layout needed for the framework's public contracts and benchmarks.
- Define documented public generic API contracts for batches, pools, typed ports and connections, node results, queue operations, and execution configuration.
- Define ownership, move, drop, wait, error, and lifecycle semantics without implementing the data plane.
- Add compile-time examples that demonstrate valid heterogeneous typed connections and document that incompatible ports must fail compilation.
- Add a benchmark harness and allocation gates for future node-call, pool, local-edge, remote-queue, and end-to-end DAG paths.

## Capabilities

### New Capabilities
- `typed-pipeline-contracts`: Public, generic pipeline API contracts and documented runtime semantics that later implementation stages must preserve.
- `pipeline-performance-harness`: Repeatable benchmarks and zero-allocation assertions for approved steady-state data-plane paths.

### Modified Capabilities

None.

## Impact

- Adds the initial Go module and public package skeletons under the planned framework layout.
- Establishes API shapes consumed by subsequent batch, graph, node, queue, runtime, metrics, and event-log work.
- Adds standard-library Go tests and benchmarks; no external runtime dependencies are required.
