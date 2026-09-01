## Context

The preceding staged changes establish a mutable typed `Builder`, generated node arities, static explicit-state and closure factory contracts, and typed `Connect` operations. Those operations can reject invalid individual mutations, but validity also depends on the completed topology: all required ports must be connected, the graph must be acyclic, and every node must participate in a source-to-sink path.

Validation is staged separately from builder sealing and immutable definition construction. The separate `build-immutable-graph-definition` change will consume the validation result later; this change must neither seal a builder nor introduce `Definition`.

Validation must not execute user code. Go map iteration is nondeterministic, so issue order, topological order, and cycle witnesses all require explicit ordering. Validation may allocate scratch state because it is a control-plane operation, not a steady-state data path.

## Goals / Non-Goals

**Goals:**

- Expose non-mutating `Validate() (*ValidationReport, error)` with explicit receiver-state behavior and structured aggregate diagnostics.
- Check statically knowable node factory, handler, selector, ordinary-port connectivity, and edge-configuration invariants.
- Compute deterministic DAG order and a deterministic cycle witness.
- Verify forward source reachability and reverse sink reachability.
- Return sorted issues, canonical order, and deterministic adjacency snapshots that later immutable definition construction can copy directly.
- Cover ordering and graph invariants with focused tests and fuzz/property tests.

**Non-Goals:**

- Seal `Builder`, cache build results, create immutable `Definition`, or add `Build` behavior.
- Invoke factories, handlers, selectors, queue factories, or lifecycle callbacks.
- Validate callbacks that exist only in a future value returned by a closure factory.
- Execute nodes or queues, add runtime state, or implement error routing.
- Change typed connection cardinality, fan-out policy, port identity, or registration APIs established by prior changes.

## Decisions

### Return an explicit reusable validation report

The exact entry point is:

```go
func (b *Builder) Validate() (*ValidationReport, error)
```

`NewBuilder` is required; the zero value is not a usable builder. A nil receiver and an uninitialized zero-value receiver each return `(nil, *ValidationError)` with exactly one `ValidationCodeInvalidBuilder` issue. An initialized builder with no nodes returns a non-nil empty report and a `*ValidationError` with exactly one `ValidationCodeEmptyGraph` issue. A non-empty initialized builder always returns a non-nil report: the error is nil when the issue list is empty and is a `*ValidationError` containing the same sorted issues otherwise.

An open builder remains open after validation. A builder sealed by the later immutable-build stage may also be validated; sealed state alone is not an issue and `Validate` does not return `ErrBuilderSealed`. Validation reads its fixed topology and follows the same report/error rules as an open builder. Neither success nor failure changes the preexisting open or sealed state.

`ValidationReport` has unexported storage and these read-only, defensive-copy accessors:

```go
func (r *ValidationReport) Issues() []ValidationIssue
func (r *ValidationReport) TopologicalOrder() []NodeID
func (r *ValidationReport) ForwardAdjacency() []ValidationAdjacency
func (r *ValidationReport) ReverseAdjacency() []ValidationAdjacency

type ValidationAdjacency struct {
	NodeID NodeID
	Edges  []ValidationAdjacentEdge
}

type ValidationAdjacentEdge struct {
	EdgeID         EdgeID
	AdjacentNodeID NodeID
}
```

Both adjacency snapshots contain one row for every node, including nodes with no edges, with rows sorted by `NodeID`. A forward row lists destinations and a reverse row lists sources; each edge list sorts by `AdjacentNodeID` and then `EdgeID`. Only ordinary edges are present. The report's topological order contains every node exactly once when the topology is acyclic, even if non-cycle issues exist, and is empty rather than a partial Kahn prefix when a cycle exists. The issue list is always sorted.

The report owns copies of its order, issues, cycle witnesses, adjacency rows, and edge slices. Its accessors deep-copy nested slices. The later immutable `Build` can copy the report's package-private snapshots directly rather than rerunning graph analysis, but this change does not add `Build` or cache a report on `Builder`.

Every validation call derives the report and all traversal scratch state from current builder metadata. It does not write validation state, order, marks, snapshots, or cached errors into the builder. Both successful and failed validation leave identity allocation, registration, and connection behavior unchanged.

Alternatives considered:

- Return only an error: rejected because the later build stage needs a stable analysis snapshot and should not duplicate or rerun graph algorithms.
- Return a report value: rejected because a pointer permits `(nil, error)` to distinguish an uninitialized receiver from an initialized graph analysis.
- Add `Build` now and return a mutable intermediate: rejected because it blurs the explicit immutable-definition boundary and would create lifecycle behavior outside this change.
- Cache validation results: rejected because the builder remains mutable and cache invalidation would add unnecessary mutation and stale-result risk.

### Use fixed structured codes and entity fields

`ValidationCode` uses an invalid zero value that validation never emits, followed by exported constants `ValidationCodeInvalidBuilder`, `ValidationCodeEmptyGraph`, `ValidationCodeMissingFactory`, `ValidationCodeMissingHandler`, `ValidationCodeMissingSelector`, `ValidationCodeUnconnectedInput`, `ValidationCodeUnconnectedOutput`, `ValidationCodeInvalidEdgeCapacity`, `ValidationCodeInvalidEdgePolicy`, `ValidationCodeCycle`, `ValidationCodeUnreachableFromSource`, and `ValidationCodeCannotReachSink` in sort order.

`ValidationIssue` carries its code and relevant `NodeID`, node-local `PortID`, or `EdgeID`; a cycle issue additionally carries the ordered witness. `ValidationError` owns the sorted aggregate, implements `error`, and exposes issues through a defensive deep copy so callers cannot change later formatting or witness data. For an initialized builder, its issues exactly equal `ValidationReport.Issues()`. Callers use `errors.As` to obtain the aggregate and inspect codes rather than parsing text.

```go
type ValidationIssue struct {
	Code   ValidationCode
	NodeID NodeID
	PortID PortID
	EdgeID EdgeID
	Cycle  []NodeID
}

func (e *ValidationError) Issues() []ValidationIssue
```

The issue code determines which entity fields apply; unused fields retain their zero values. `Issues` deep-copies each `Cycle`.

Issues sort by `ValidationCode`, then `NodeID`, `PortID`, `EdgeID`, and finally a lexicographic cycle witness. The fixed code values are part of the diagnostic contract. Error formatting follows this order and includes applicable entity identities, making repeated validation byte-for-byte stable for unchanged topology.

Alternatives considered:

- Return the first error: rejected because independent graph defects would require repeated correction cycles.
- Sort human-readable messages: rejected because wording changes and numeric lexical order would destabilize machine-visible ordering.
- Export the aggregate's backing slice: rejected because external mutation could make one returned error internally inconsistent.

### Consume registration-produced static validation masks

Generated graph registration records a non-generic `staticValidationDescriptor` in each node's structural metadata. The descriptor contains `required` and `present` `uint16` masks: bit 0 denotes the factory, bit 1 denotes the selector, and bits 2 through 9 denote handler positions zero through seven. For a source, handler position zero denotes its source handler; for nodes with inputs, handler position `i` denotes input handler `i`. Explicit-state registration requires the factory, its source or input handlers, and a selector only when input arity exceeds one. Closure registration requires only its factory because produced callbacks do not exist until runtime startup.

Each generated generic registration function determines presence directly from its statically typed function fields and stores only these masks for validation. Validation computes `required &^ present` and maps missing bits to structured node and port issues. It does not retain callback values in `any`, use reflection or runtime type assertions, or rediscover generic callback shapes.

No node, closure, state, lifecycle, handler, selector, or queue factory is called. Panic behavior and callback validity after a closure factory returns remain a future runtime-start concern.

Alternatives considered:

- Inspect typed callback fields directly from heterogeneous node metadata during validation: rejected because this requires reflection, assertions, or an `any` registry; registration already has the static types needed to produce masks.
- Invoke closure factories during validation: rejected because validation must have no user-code side effects and one factory call per future replica per start is part of the runtime lifecycle contract.
- Treat optional lifecycle callbacks as required: rejected because the prior factory contract defines them as optional.

### Validate completed ordinary connectivity and stored edge configuration

Every declared ordinary input and ordinary output must have exactly one edge under the single-consumer connection model. A node may declare zero inputs or zero ordinary outputs. The prior connection API does not permit connecting typed error outputs; validation therefore excludes them from connectivity, adjacency, degree, topological, cycle, and reachability analysis rather than treating them as optional ordinary edges.

`Connect` and validation use the same unexported, non-generic, pure edge-configuration validator for capacity and queue-full policy. `Connect` maps its result to mutation-time errors; validation maps the same result to edge-specific validation codes. The helper neither mutates topology nor invokes the optional typed queue factory. Duplicating capacity or policy predicates in validation is prohibited.

Alternatives considered:

- Trust `Connect` and omit edge checks: rejected because the later immutable builder should depend on one complete validation boundary, and tests inside the package may construct malformed metadata.
- Duplicate `Connect` predicates in validation: rejected because mutation-time and whole-graph acceptance could drift.
- Require an error edge: rejected because executable error routing belongs to a later stage.

### Use deterministic Kahn order and residual DFS witness

Kahn's algorithm computes indegrees from the report's forward adjacency snapshot in scratch storage and always removes the smallest available `NodeID`. Outgoing adjacency already sorts by destination `NodeID` and then `EdgeID`. If all nodes are consumed, the resulting order is canonical and every edge runs from an earlier to a later position.

If nodes remain, validation discards the partial order as a build candidate and runs color-marked DFS over residual nodes. DFS starts nodes by ascending `NodeID` and traverses residual outgoing edges by destination `NodeID`, then `EdgeID`. The first back edge produces one structured closed witness from the ancestor through the DFS stack and back to that ancestor. A self-loop therefore produces `[N, N]`. This witnesses the DAG failure without attempting to enumerate every cycle.

Alternatives considered:

- Use map iteration or a FIFO seeded from maps: rejected because valid order and cycle diagnostics would vary between runs.
- Report all simple cycles: rejected because their count can be exponential and one deterministic witness is sufficient to establish failure.
- Run DFS alone for both ordering and cycles: rejected because smallest-ready Kahn order is an explicit canonical order suitable for later build metadata.

### Define sources and sinks from declared arity

A structural source has zero declared ordinary inputs; a structural sink has zero declared ordinary outputs. Validation marks nodes through the report's forward snapshot from all structural sources and, independently, through its reverse snapshot from all structural sinks. It emits an issue for each node missing either property.

An isolated node with both zero ordinary inputs and zero ordinary outputs is seeded in both traversals and is valid. Non-connectable error outputs are absent from both traversals and do not affect source or sink status. Traversals and issue emission use ascending node and edge identity order.

Alternatives considered:

- Infer sources and sinks from current indegree and outdegree: rejected because missing required connections could incorrectly promote an incomplete node to a source or sink.
- Skip reachability when a cycle exists: rejected because reachability failures are independently discoverable and aggregate diagnostics should remain complete.

## Risks / Trade-offs

- [Connectivity and reachability can report related defects for one node] -> Preserve distinct codes so callers can group diagnostics while still receiving every independently discoverable invariant failure.
- [A fixed code order constrains future diagnostics] -> Append new codes deliberately or define an explicit sort rank without renumbering existing values.
- [Public snapshots add API surface] -> Keep representation ID-only, defensive, and deterministic so final build can copy it without exposing mutable builder metadata.
- [A report becomes stale after an open builder mutation] -> Define it as an owned point-in-time snapshot; callers validate again after mutation.
- [Validation allocates proportional scratch memory] -> Accept control-plane `O(V + E)` storage and use linear graph algorithms; do not optimize at the cost of builder mutation.
- [One witness does not identify every cyclic component] -> Document that it proves acyclicity failure; rerunning after correction may expose another cycle.

## Migration Plan

1. Add diagnostic, adjacency, and report types with deterministic defensive accessors without changing builder mutation APIs.
2. Make generated registration populate static validation masks and share the pure edge validator with `Connect`.
3. Add static and topology checks that populate `ValidationReport`.
4. Expose `Builder.Validate` with the specified receiver-state matrix and add focused and fuzz/property tests.
5. Let the later immutable-definition change copy a valid report before sealing.

There is no deployed graph runtime or persisted graph format. Rollback removes the validation API and implementation without transforming data.

## Open Questions

None for this change. Public definition inspection and builder sealing remain decisions for `build-immutable-graph-definition`.
