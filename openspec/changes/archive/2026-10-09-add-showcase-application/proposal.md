## Why

The framework now supports typed ingress, explicit ownership, broadcast, element sharding, fan-in, typed failures, multi-group execution, and runtime metrics, but it has no runnable end-to-end application that shows how those capabilities fit together. A deterministic, dependency-free example will give users a concrete starting point and make the framework easier to evaluate and learn.

## What Changes

- Add a runnable order-processing showcase under `examples/showcase` using a small deterministic workload.
- Demonstrate typed ingress, validated and rejected records, an audit broadcast branch, customer-key element sharding, fan-in aggregation, fixed CPU-bound groups, typed error handling, runtime metrics, and graceful drain.
- Document how to run the showcase and explain the framework features it exercises.
- Add tests that verify the example's end-to-end output and lifecycle behavior without external services.

## Capabilities

### New Capabilities

None. The example exercises existing framework contracts and adds no runtime API or behavior.

### Modified Capabilities

None. This change is an example application and documentation only; `.openspec.yaml` sets `skip_specs: true`.

## Impact

- Adds Go source and tests under `examples/showcase`.
- Updates `README.md` with run instructions and a link to the example.
- Uses only existing public framework APIs and the Go standard library; no new dependencies.
