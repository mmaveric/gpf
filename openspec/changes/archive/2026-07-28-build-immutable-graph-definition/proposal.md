## Why

The preceding graph changes provide typed construction and deterministic DAG validation, but callers still need a stable boundary between editable topology and reusable runtime input. `Builder.Build` must consume validation transactionally and produce a definition that cannot expose or accumulate mutable runtime state.

## What Changes

- Add `func (b *Builder) Build() (*Definition, error)` with explicit nil, zero-value, open-invalid, open-valid, and already-sealed behavior, reusing the builder prerequisite's `ErrInvalidBuilder` and `ErrBuilderSealed` sentinels.
- Make Build consume the `ValidationReport` produced by `add-graph-dag-validation`, including canonical forward and reverse adjacency and canonical topological order, without recomputing graph analysis.
- Add an opaque `Definition` that owns copied dense metadata and report topology while retaining the builder's persistent non-zero owner identity.
- Cache the successful definition so repeated Build calls return the same pointer without revalidation or reconstruction.
- Expose exactly `NodeCount`, `EdgeCount`, and clone-returning `TopologicalOrder` as public definition inspection; keep adjacency private.
- Preserve dormant `Stateful.Factory`, `ClosureFactory`, `queue.Factory`, and available `Init` and `Close` callbacks without invoking them during Build or retaining factory-produced state or live `QueueOps`.
- Document and test transactional Build, structural immutability, persistent identity, factory dormancy, and sequential definition reuse.
- Keep runtime startup, handler execution, scheduling, queue creation, concurrent starts, and CPU placement out of scope.

## Capabilities

### New Capabilities
- `immutable-graph-definition`: Exact Build lifecycle behavior, persistent non-zero graph identity, copied immutable validation topology, fixed cloned inspection, idempotent pointer caching, and runtime-neutral sequential reuse.

### Modified Capabilities
- `typed-pipeline-contracts`: Refine queue and execution semantics so Build yields reusable immutable topology with dormant factories while runtime startup and live execution resources remain unavailable.

## Impact

- Affects `graph.Builder.Build`, the opaque graph definition representation, the three definition inspection methods, and graph documentation and tests.
- Relies on the five preceding split changes through `add-graph-dag-validation`; it does not redefine their registration, connection, factory, or validation contracts.
- Does not add dependencies or implement runtime starts, execution, queues, pools, scheduling, metrics, or benchmarks.
