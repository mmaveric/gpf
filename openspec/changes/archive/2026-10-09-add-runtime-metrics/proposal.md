## Why

The fixed-group executor now transports ordinary batches and typed failures, but queue drops, pool exhaustion, and unconnected failures remain invisible to applications. Runtime counters make these outcomes inspectable while preserving typed ownership, bounded transport, and zero-allocation processing.

## What Changes

- Add always-on executor-owned node, edge, and group counters with a race-safe `Executor.Metrics()` snapshot API.
- Define exact counting boundaries for handler invocation, processed input, failure, error publication, queue Drop, pool exhaustion, Wait retry, and shutdown discard.
- Expose sampled queue depth and sampled high-water marks without invoking queue callbacks from snapshot readers.
- Keep metrics fresh per Start and readable after stop, with deterministic identity ordering and independent snapshot storage.
- Instrument generated explicit-state and closure bindings plus local and remote transport without boxing values or changing scheduling and ownership.
- Extend allocation gates, race coverage, benchmarks, and documentation for instrumented execution.

## Capabilities

### New Capabilities
- `runtime-metrics`: Numeric executor snapshots, counter semantics, concurrency, lifecycle, and bounded instrumentation.

### Modified Capabilities
- `typed-pipeline-contracts`: Make built-in runtime counters available while retaining the remaining deferred observability features.
- `pipeline-performance-harness`: Verify zero-allocation instrumented processing and report instrumentation overhead.

## Impact

Affected code includes `graph` executor bindings and workers, `node` output-finalization hooks where necessary, the arity generator and committed generated bindings, and performance tests. Public numeric snapshot types live in `graph`; no new dependency or export backend is required. Build remains runtime-neutral, Start signatures remain unchanged, and metrics snapshots may allocate in the control plane.
