## MODIFIED Requirements

### Requirement: Go module and public contract packages
The repository SHALL define a Go module requiring Go 1.22 or newer and SHALL provide compileable public contract packages for executable typed batch handles and pools, typed graph ports and connections, node result types, typed queue operations, and runtime configuration. The declarations and batch steady-state operations SHALL not use reflection, runtime type assertions, or `interface{}` to represent data-plane values.

#### Scenario: Compile public contracts
- **WHEN** the module is tested with Go 1.22 or newer
- **THEN** all public packages compile and the batch package works without requiring an executable graph runtime

### Requirement: Batch ownership contract
The batch contract SHALL provide explicit allocation-free ownership transfer and release for batches acquired from a typed pool. A caller that acquires a batch SHALL own it until it moves the batch or releases it; `Move` SHALL clear the source handle, and release SHALL clear the caller's handle. The contract SHALL document that future handlers MAY move an input only to a compatible output and that an un-moved input is returned to its source pool by the future runtime. Runtime ownership checks SHALL NOT be required.

#### Scenario: Move an acquired batch
- **WHEN** a caller moves an acquired batch to another handle
- **THEN** the destination retains the slot and the source is an empty handle that cannot release the slot again through correct API use

## ADDED Requirements

### Requirement: Preallocated typed pool
`NewPool[T]` SHALL validate a nonzero pool capacity and batch capacity, reject invalid size arithmetic, and allocate all slot metadata and typed batch storage before returning a pool. A pool SHALL NOT resize or allocate additional batch storage after construction.

#### Scenario: Construct a valid pool
- **WHEN** a caller constructs a pool with positive slot and batch capacities
- **THEN** the pool contains exactly the configured number of fixed-capacity typed batch slots

#### Scenario: Reject invalid pool configuration
- **WHEN** a caller constructs a pool with zero capacity, zero batch capacity, or overflowing storage size
- **THEN** construction returns an error without creating a usable pool

### Requirement: Controlled batch slice access
An acquired `Batch[T]` SHALL expose `Mutable()` as its full fixed-capacity writable storage and SHALL expose `Items()` limited to its explicit logical length. `SetLen(n)` SHALL accept only a length from zero through `Cap()` and SHALL reject an out-of-range length without allocating. Callers SHALL fill `Mutable()` by index and SHALL NOT rely on capacity-growing append behavior.

#### Scenario: Fill and publish logical content
- **WHEN** a caller writes elements into `Mutable()` and sets a valid logical length
- **THEN** `Items()` exposes exactly those logical elements and `Cap()` remains the configured batch capacity

#### Scenario: Reject oversized logical content
- **WHEN** a caller sets a logical length greater than the batch capacity
- **THEN** the operation reports failure and the batch remains within its fixed capacity

### Requirement: Allocation-free pool lifecycle
`Acquire` SHALL return an owned typed batch and availability indicator without allocating after pool construction. If every slot is acquired, `Acquire` SHALL report exhaustion with no batch and SHALL NOT allocate a replacement. `Release` of an owned batch SHALL return its slot for reuse without allocating. Stage 1 pool operations SHALL be used by one owner goroutine and SHALL NOT promise concurrent safety.

#### Scenario: Reuse a released batch
- **WHEN** a caller acquires, fills, and releases a batch from a non-exhausted pool
- **THEN** a later acquire can reuse a pool slot without allocation

#### Scenario: Exhaust a pool
- **WHEN** a caller acquires every configured slot and requests one more batch
- **THEN** acquire reports exhaustion without allocation or capacity growth

### Requirement: Configurable release clearing
When `ClearOnRelease` is enabled, release SHALL zero the complete fixed-capacity storage of the released batch before making its slot reusable. When disabled, release SHALL retain element values until overwritten and SHALL document the pointer-retention risk.

#### Scenario: Clear a previously populated slot
- **WHEN** a caller releases a batch with clearing enabled and later reacquires its slot
- **THEN** every element of the slot's full batch capacity is the zero value

#### Scenario: Retain storage when clearing is disabled
- **WHEN** a caller releases a batch with clearing disabled
- **THEN** the pool does not perform clearing work and its documentation identifies possible retained references
