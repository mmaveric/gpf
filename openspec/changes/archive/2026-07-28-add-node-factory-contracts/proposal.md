## Why

Generated typed handler families need a lifecycle-safe way to create replica-local explicit state and closure-bound handlers without sharing mutable runtime state across replicas or restarts. Defining these contracts separately establishes ownership and rollback behavior before graph registration or runtime startup is implemented.

## What Changes

- Add exact public `node.StateFactory[S]`, `node.InitFunc[S]`, and `node.CloseFunc[S]` function types and compose them in `node.Stateful[S, H]` with typed generated handlers.
- Add exact public `node.ClosureReplica[H]` and `node.ClosureFactory[H]` types for freshly constructed closure-bound handlers with optional lifecycle callbacks.
- Require one successful factory result per replica per start, with no mutable state shared by the framework across replicas or starts.
- Require a provided `Close` hook exactly once after every successful factory return, including rollback when `Init` fails after partially initializing a replica.
- Require configuration captured by factories, handlers, and lifecycle callbacks to remain immutable for the lifetime of the contract.
- Establish a deferred integration constraint that future graph registration records only static validation facts and future graph Build never invokes state or closure factories; the graph bridge belongs to the next change.
- Explicitly depend on `generate-typed-node-arities` for the generated handler-set types used as `H`.
- Exclude graph builders, ports, registration, connections, DAG validation, and immutable graph definitions.

## Capabilities

### New Capabilities

- `node-factory-contracts`: Typed explicit-state and closure factory contracts, fresh replica lifecycle semantics, rollback cleanup, captured-configuration immutability, and deferred factory invocation.

### Modified Capabilities

None.

## Impact

- Adds public generic data contracts and documentation in the `node` package, plus focused node-level compile-time and lifecycle contract tests.
- Requires the generated handler families delivered by the `generate-typed-node-arities` change before implementation begins.
- Constrains future graph registration, Build, and runtime implementations: registration records static validation facts without invoking factories, Build cannot execute user factories, and runtime startup must create and clean up fresh replicas according to these contracts.
- Does not add or change executable graph construction, registration, connection, topology, or definition APIs.
