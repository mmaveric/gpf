## MODIFIED Requirements

### Requirement: Batch ownership contract
The batch contract SHALL provide explicit allocation-free ownership transfer and release for batches acquired from a typed pool. A caller that acquires a batch SHALL own it until it moves the batch or releases it; `Move` SHALL clear the source handle, and `Pool.Release` and `Batch.Release` SHALL clear the caller's handle. `Batch.Release` SHALL return an owned non-empty batch to the pool that allocated its slot, including after the batch has moved through compatible typed edges. The contract SHALL document that handlers MAY move an input only to a compatible output and that an un-moved, un-released input is returned to its originating source pool by the executor. Moving or explicitly releasing the handler's by-value copy SHALL relinquish automatic input cleanup, so a retained runner copy SHALL not release or reset a transferred slot. Invocation-local transfer bookkeeping SHALL remain allocation-free and SHALL not travel with independently moved batches. General runtime checks for arbitrary handle misuse SHALL NOT be required.

#### Scenario: Move an acquired batch
- **WHEN** a caller moves an acquired batch to another handle
- **THEN** the destination retains the slot and the source is an empty handle that cannot release the slot again through correct API use

#### Scenario: Release an origin-owned moved batch
- **WHEN** an executor releases an un-moved batch after it has crossed one or more compatible edges
- **THEN** Batch.Release returns it to its original allocating pool without type erasure or a runtime pool lookup

#### Scenario: Preserve a move from a by-value handler input
- **WHEN** a handler moves its copied input handle into a compatible committed output
- **THEN** the runner's retained cleanup handle does not release the transferred slot, and downstream retains exclusive ownership of its original length and items
