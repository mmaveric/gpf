## Why

Typed registration and connection can reject local mistakes, but they cannot establish that the complete graph is a valid executable DAG. A deterministic, structured validation stage is needed before a later change can safely produce an immutable definition.

## What Changes

- Add exact `Validate() (*ValidationReport, error)` behavior for invalid nil and zero-value builders plus initialized empty, open, and sealed builders.
- Add a reusable validation report containing sorted issues, canonical topological order, and deterministic forward and reverse adjacency snapshots for later immutable build.
- Add structured `ValidationCode`, `ValidationIssue`, and `ValidationError` diagnostics with deterministic issue ordering and stable error formatting.
- Validate registration-produced static factory, handler, and multi-input selector masks without invoking user code, reflection, type assertions, or `any` storage.
- Require every ordinary input and ordinary output to be connected while excluding typed error outputs, which are not connectable in this stage.
- Recheck stored edge configuration through the same pure validator used by `Connect` and compute deterministic Kahn topological order, with a deterministic residual DFS cycle witness when sorting cannot consume every node.
- Check forward reachability from structural sources and reverse reachability from structural sinks, treating an isolated zero-input/zero-output node as valid.
- Add example-based and fuzz/property tests for deterministic diagnostics, cycle witnesses, reachability, non-mutation, and valid topological ordering.
- Do not seal `Builder`, add immutable `Definition`, or change topology after either successful or failed validation; permit validation of a builder sealed by a later build stage.

## Capabilities

### New Capabilities

- `graph-dag-validation`: Structured, deterministic, non-mutating validation of complete typed graph topology before immutable build.

### Modified Capabilities

None.

## Impact

- Adds validation APIs and reusable analysis snapshots in `graph`, using builder metadata and pure edge checks established by prior typed builder, node-factory, arity, and connection changes.
- Adds graph validation unit, fuzz, and property tests; no runtime execution, queue construction, or data-plane behavior is introduced.
- Preserves the mutable builder lifecycle for the later immutable-definition change and adds no external dependencies.
