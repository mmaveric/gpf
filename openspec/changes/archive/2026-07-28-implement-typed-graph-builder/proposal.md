## Why

Typed node arities and factory contracts need a mutable graph-construction layer that can assign stable identities and return compiler-checked ports before connections or immutable definitions can be implemented. This change isolates that registration foundation so later topology stages can build on a small, verifiable contract.

## What Changes

- Add exact `NodeID`, `PortID`, `NewBuilder`, and builder-state contracts: zero is invalid for both ID types, valid IDs begin at one, and registration reserves `ErrBuilderSealed` for the later sealing transition.
- Give every builder a private non-zero-sized owner token so separately allocated owner pointers are guaranteed to compare unequal.
- Add opaque, builder-owned `InPort[T]`, `OutPort[T]`, and `OutputRef[O]` handles whose zero values are invalid.
- Add generic `NodePorts` registration results with named accessors and generated graph `InPorts0` through `InPorts8`, `OutPorts0` through `OutPorts8`, and exact `OutputPorts0` through `OutputPorts8` expansion functions.
- Add generated explicit-state and closure registration functions for input arities zero through eight, consuming the node contracts supplied by prerequisite changes.
- Record static required, present, and deferred validation masks plus dormant typed normalization closures without `any`, reflection, or runtime type assertions.
- Extend the shared arity generator with a graph-owned output manifest and graph-specific generated-file stability test.
- Preserve package direction `node <- graph` by keeping graph handles and registration results out of the `node` package.
- Establish ordering dependencies on `generate-typed-node-arities`, followed by `add-node-factory-contracts`, before this change is implemented.
- Leave connection edges, DAG validation, the operation that seals a builder, `Build`, and immutable `Definition` behavior to later changes.

## Capabilities

### New Capabilities

- `typed-graph-builder`: Mutable graph identity, opaque typed handles, stable registration identities, generated graph port bundles and registration APIs, and dormant typed registration metadata.

### Modified Capabilities

None.

## Impact

- Adds public graph-construction APIs and generated declarations in the `graph` package, plus private registration metadata needed by later graph stages.
- Depends in order on `generate-typed-node-arities` and `add-node-factory-contracts`; it does not duplicate their node output-shape, handler, or factory contracts.
- Adds a graph-specific generated-output manifest and focused compile/runtime tests for all supported arities, identity stability, validation masks, invalid handles, package direction, and forbidden dynamic mechanisms.
- Reserves sealed-state rejection for later mutation APIs but does not implement `Connect`, `Build`, sealing, graph validation, or immutable `Definition` behavior.
