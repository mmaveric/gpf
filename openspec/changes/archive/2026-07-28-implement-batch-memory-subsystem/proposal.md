## Why

The framework currently exposes only contract declarations: `Batch[T]` has no owned storage and `Pool[T]` cannot acquire or release work. A preallocated typed memory subsystem is the first executable data-plane component and is required before typed graph construction or scheduling can be implemented without allocations.

## What Changes

- Implement fixed-capacity, typed `Pool[T]` instances that allocate all slots and backing storage during construction.
- Make `Batch[T]` an owned handle with a controlled logical length, a writable full-capacity `[]T` view, and a readable populated `[]T` view.
- Add allocation-free acquire, length-setting, release, and compatible move semantics.
- Implement configurable clearing on release and allocation-free pool-exhaustion behavior.
- Enable the pool acquire/release benchmark and zero-allocation gate after setup.
- **BREAKING** Replace the contract-only `Batch[T]` and `Pool[T]` placeholders with their executable API.
- Defer ingress-specific `Commit` and copying `Submit` until typed ingress exists in the graph-definition stage.

## Capabilities

### New Capabilities

None.

### Modified Capabilities
- `typed-pipeline-contracts`: Make batch ownership, pool capacity, controlled slice access, clearing, exhaustion, and release behavior executable.
- `pipeline-performance-harness`: Turn the reserved pool acquire/release benchmark into an executable steady-state zero-allocation measurement.

## Impact

- Replaces the placeholder implementation in `batch` with the framework's first executable data-plane path.
- Adds standard-library unit, fuzz, and allocation tests for pool lifecycle and ownership contracts.
- Does not add dependencies, graph behavior, queues, ingress commits, or scheduler behavior.
