## Why

The typed builder dependencies provide builder-owned ports, but the current `Connect[T]` remains a definition-targeted placeholder that cannot record topology. Executable, compiler-checked connection mutation is needed as a focused step before graph-wide validation or immutable definition construction.

## What Changes

- Add `type Factory[T any] func(Config) (QueueOps[T], error)` as the exact per-runtime queue constructor contract.
- **BREAKING** Replace non-generic `EdgeConfig` with `type EdgeConfig[T any] struct { Capacity uint32; Policy queue.Policy; Factory queue.Factory[T] }`.
- **BREAKING** Replace the placeholder with `func Connect[T any](builder *Builder, output OutPort[T], input InPort[T], config EdgeConfig[T]) (EdgeID, error)`.
- Validate zero and foreign handles, one producer per input, one consumer per output, and explicitly reject unsupported fan-out.
- Export stable connection error sentinels and a typed edge-configuration error, with deterministic validation precedence.
- Reserve typed error `OutPort` handles but reject their connection with `ErrErrorRoutingUnavailable` until the error-routing stage, so error edges do not enter topology.
- Use one pure edge-configuration validator from `Connect` that later graph-wide validation can reuse.
- Make failed connections atomic so they neither mutate topology nor consume edge IDs.
- Capture a typed future-binding closure without invoking the queue factory or constructing queues during connection or graph build.
- Replace placeholder examples and tests with matching typed connections and compile-fail coverage for incompatible port types.
- Exclude graph-wide DAG validation and the Build operation that performs definition sealing; `Connect` only observes the established sealed state and returns `ErrBuilderSealed`.

## Capabilities

### New Capabilities
- `typed-graph-connections`: Exact typed queue and connection APIs, deterministic errors, typed edge configuration, builder mutation, ordinary-port provenance and cardinality checks, dense edge identity, atomic failure, and deferred per-runtime queue binding.

### Modified Capabilities
- `typed-pipeline-contracts`: Replace the placeholder static connection requirement with executable builder-targeted `Connect[T]` behavior while preserving compile-time type compatibility.

## Impact

- Affects public declarations and focused documentation, examples, and tests in `graph` and `queue`.
- Depends on `generate-typed-node-arities`, `add-node-factory-contracts`, and `implement-typed-graph-builder` for typed registration results, valid builder-owned handles, and open-builder mutation state.
- Does not implement runtime queue execution, graph-wide DAG validation, `Definition` construction, or the later Build transition that seals a builder.
