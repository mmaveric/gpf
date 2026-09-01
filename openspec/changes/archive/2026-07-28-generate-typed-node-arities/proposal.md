## Why

The node package currently handwrites fixed-arity declarations, leaves output shapes unconstrained, and provides only explicit-state handler sets. A deterministic generator and complete typed handler contracts are needed before downstream node factories or graph construction can depend on a stable arity surface.

## What Changes

- **BREAKING** Replace handwritten output and node arity declarations with committed, deterministic generated declarations for arities zero through eight.
- Seal `OutputSet` to the generated `Outputs0` through `Outputs8` shapes and expose exact `OutputCount() uint8` ordinary output counts.
- Replace generic handler naming with typed `SourceHandler` and `InputHandler` contracts, preserving a distinct input element type for every node input.
- Add `ReadyMask`, `PortIndex`, and `Port1` through `Port8` readiness-selection contracts.
- Generate explicit `Node0` through `Node8` and closure `ClosureNode0` through `ClosureNode8` handler sets, including selectors only for multi-input arities and requiring selectors called for multiple ready inputs to choose an in-range ready input.
- Remove only the node-package placeholder error handle while reserving `E` for later graph registration to expose as a separate typed error output beyond eight ordinary outputs.
- Use a standard-library-only generator, commit formatted node-generated files, and add tests that prove complete arity coverage and byte-for-byte stability for this phase's node-output manifest.
- Keep this phase's generated output node-only while making `internal/genarity` intentionally extensible with additional output targets in the later graph-builder change.
- Establish this change as dependency 1 of 6; downstream `add-node-factory-contracts` depends on these generated contracts.
- Do not add state or closure factories, graph builders, graph ports, node registration, `Connect`, DAG validation, or `Definition`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `typed-pipeline-contracts`: Constrain ordinary node outputs to sealed generated shapes and define complete generated explicit and closure handler sets with typed per-input dispatch and readiness selection.

## Impact

- Changes the public API in `node`, replacing handwritten arity declarations and legacy handler names before a compatibility release.
- Adds a standard-library generator under `internal/`, an ordered node-output manifest, committed generated node source, and node-manifest generation-stability tests; later changes may extend the generator with graph outputs.
- Does not modify the graph construction surface or implement executable runtime behavior.
- Provides the required contract foundation for `add-node-factory-contracts` and the remaining staged graph-definition changes.
