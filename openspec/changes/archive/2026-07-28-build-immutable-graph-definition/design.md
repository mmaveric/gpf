## Context

This is the sixth and final split of the monolithic `implement-typed-graph-definition` change. In order, the preceding five changes provide generated arities, node factory contracts, builder-owned typed topology, executable typed connections, and deterministic DAG validation. This change assumes those contracts are present and adds only the commit boundary from an editable builder to an immutable graph definition.

The validator from `add-graph-dag-validation` already owns deterministic issue aggregation, canonical adjacency, and canonical topological ordering. Its `ValidationReport` is the single graph-analysis result consumed by both `Builder.Validate` and this change's Build operation. No runtime exists at this stage, so a definition is immutable control-plane input, not an executing graph.

Go function values can close over caller-owned mutable objects. The framework can copy and hide its own structural metadata, but it cannot deep-copy arbitrary captures in `Stateful.Factory`, `ClosureFactory`, `queue.Factory`, handlers, or lifecycle callbacks. Their contracts therefore keep all user functions dormant during Build and require captured configuration to be treated as immutable.

## Goals / Non-Goals

**Goals:**

- Define nil, zero-value, open-invalid, open-valid, and already-sealed behavior for `func (b *Builder) Build() (*Definition, error)`.
- Leave an open builder fully editable after validation failure and seal all topology mutation only after successful Build.
- Cache one successful definition so subsequent Build calls are idempotent and return the same definition instance.
- Preserve the builder's non-zero owner identity used by existing typed handles in the resulting definition.
- Own dense copied node and edge metadata, forward and reverse adjacency, and the validator's canonical topological order.
- Prevent callers from mutating definition structure through inspection APIs.
- Retain dormant state, closure, and queue factory function values needed by sequential future runtime starts without storing live state or queue operations.

**Non-Goals:**

- Change registration, typed connection, cardinality, validation rules, issue ordering, or cycle detection established by preceding changes.
- Invoke an explicit state factory, closure factory, typed queue factory, or lifecycle callback.
- Start a runtime, execute handlers, create state or queue operations, allocate pools, schedule work, collect metrics, or implement CPU placement.
- Promise concurrent Build calls, concurrent builder mutation, or concurrent runtime starts from one definition.
- Deep-freeze caller-owned values captured by function closures.

## Decisions

### Define the complete Build state contract

The public method has exactly this shape:

```go
func (b *Builder) Build() (*Definition, error)
```

Its receiver-state behavior is:

- A nil `*Builder` returns `(nil, ErrInvalidBuilder)` and does not panic.
- A zero-value or otherwise uninitialized `Builder` returns `(nil, ErrInvalidBuilder)` and does not allocate an owner identity or mutate the receiver.
- An initialized open builder runs the prerequisite validator once. A report with issues returns `(nil, *ValidationError)` and leaves the builder open and unchanged.
- An initialized open builder with an issue-free report constructs and publishes one definition, seals mutation, and returns `(definition, nil)`.
- A builder already sealed by a successful Build returns its cached definition pointer and `nil` immediately. Successful Build is the only sealing transition, so a sealed builder without a cached definition is an unreachable internal invariant violation rather than a public state.

`ErrInvalidBuilder` and `ErrBuilderSealed` are the shared sentinels supplied by `implement-typed-graph-builder`; this change does not define replacements. Build itself does not return `ErrBuilderSealed`: after successful sealing it returns the cache. Registration and `Connect` detect the sealed state and return `ErrBuilderSealed` without mutation.

For an open valid builder, Build creates all copied definition data in local variables. Only once the snapshot is complete does it publish the cached pointer and set the sealed state. The builder APIs do not promise concurrent Build or mutation safety.

This ordering avoids half-built observable state and lets a caller correct the same builder after failure. Sealing on the first attempt was rejected because it would turn ordinary validation feedback into a requirement to reconstruct all typed handles and topology.

### Consume one exact ValidationReport

The prerequisite's `ValidationReport` contains sorted validation issues, canonical forward adjacency, canonical reverse adjacency, and canonical topological order. Both adjacency outer slices are dense by `NodeID-1`. Forward inner slices contain `EdgeID` values ordered by destination `NodeID` and then `EdgeID`; reverse inner slices contain `EdgeID` values ordered by source `NodeID` and then `EdgeID`. An issue-free report contains every node exactly once in topological order.

Build consumes that report directly. It does not rerun a second adjacency builder, topological sort, or map-derived traversal. Build deep-copies both nested adjacency structures and copies the topological order into the definition before publication. A report containing any issue yields the same `ValidationError` contract exposed by `Builder.Validate`; no report slices are retained in a partial definition.

Duplicating validation during snapshot construction was rejected because the two implementations could disagree on diagnostics or canonical order. Revalidating after a cached success was rejected because the sealed topology cannot change and callers require the same definition instance.

### Store dense private topology with a fixed public surface

`Definition` has unexported fields. It stores private dense node and edge metadata in stable identity order, copied forward and reverse adjacency, and a copied canonical topological order. Slice-backed data owned by the builder or `ValidationReport` is never reused directly. Internal references are remapped to definition-owned data where necessary.

The complete public inspection surface introduced by this change is:

```go
func (d *Definition) NodeCount() int
func (d *Definition) EdgeCount() int
func (d *Definition) TopologicalOrder() []NodeID
```

`NodeCount` and `EdgeCount` return zero for a nil receiver or zero-value definition and otherwise return the dense metadata lengths. `TopologicalOrder` returns `nil` for a nil receiver or zero-value definition. For every valid definition it allocates and returns a new slice of length `NodeCount`, copies the internal order into it, and therefore returns a non-nil empty slice for a valid zero-node definition. There is no public adjacency accessor in this stage.

After publication, no framework-owned field or nested slice in `Definition` is mutated. No public method exposes metadata, adjacency, identity, or order backing storage. Returning builder or report slices directly was rejected because element mutation could alter a supposedly immutable definition.

### Preserve one graph identity across construction and future use

Every definition returned by successful Build retains the exact non-zero private owner token established by `NewBuilder`; Build neither derives a replacement identity nor accepts the zero owner. Existing handles therefore remain attributable to the graph after builder sealing, allowing a future runtime created from the definition to reject foreign handles.

The owner token is immutable identity only, not builder storage or lifecycle state. Identity is not based on pointer equality of copied metadata or user-visible numeric node IDs, because either could collide across builders. Definition inspection does not expose the token or a way to construct, replace, or mutate it. A nil receiver or zero-value `Definition` has no valid owner identity and is never a successful Build result.

### Retain recipes, never runtime instances

Definition metadata may retain the dormant typed registration and edge-binding closures established by preceding changes. Through those closures it preserves `Stateful.Factory`, `ClosureFactory`, optional explicit `Init` and `Close` callbacks, and `queue.Factory` function values. Build never invokes a state factory, closure factory, queue factory, or lifecycle callback. Closure-replica lifecycle callbacks remain uncreated because Build does not call `ClosureFactory`.

Definition contains no factory-produced explicit state, `ClosureReplica`, `QueueOps`, queue implementation, batch pool, scheduler, worker, metric, counter, cancellation object, or lifecycle status. In particular, retaining a factory function is configuration; retaining the state or queue returned by calling it would violate structural immutability.

The same definition is suitable for sequential future starts because each start can invoke its stored factories to create fresh mutable resources and later discard them without changing definition topology. This change specifies that reuse contract but does not provide a start API. Concurrent starts remain unspecified.

Storing pre-created queues or states was rejected because restart would reuse dirty mutable state and make the definition itself lifecycle-dependent. Attempting to deep-copy closure captures was rejected because Go provides no sound general mechanism for doing so without changing the typed contracts.

## Risks / Trade-offs

- [A factory or callback closes over mutable caller state] -> Document that captured configuration must remain immutable; framework-owned metadata is still copied and private.
- [Dense snapshots duplicate temporary builder and validator memory] -> Accept one-time Build allocation in exchange for structural ownership and release builder-only references where the existing API permits.
- [The topological-order accessor leaks backing storage] -> Require repeated clone-mutation tests and keep adjacency private.
- [Snapshot logic drifts from validation semantics] -> Consume validator output directly and test that stored order is exactly the canonical validated order.
- [Future runtime code stores lifecycle state on Definition] -> Keep runtime resources in separately constructed runtime values and enforce absence through focused package tests and review.

## Migration Plan

1. Add the private definition snapshot representation and exact `NodeCount`, `EdgeCount`, and `TopologicalOrder` methods on top of the preceding builder and validator contracts.
2. Implement the complete Build state contract, `ValidationReport` copying, successful sealing, exact non-zero identity retention, and cached repeat behavior.
3. Add immutability, correction-after-failure, factory-dormancy, and sequential-reuse contract tests.
4. Update graph, queue, and project documentation to distinguish a buildable definition from unavailable runtime execution.

There is no deployed graph runtime or persisted definition format. Rollback removes the Build snapshot and inspection APIs while leaving the preceding editable builder and validator intact.

## Open Questions

None. Public adjacency inspection and runtime binding are intentionally deferred.
