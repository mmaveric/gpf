## Purpose
Define typed handler output acquisition, commit, and automatic ownership cleanup.

## Requirements

### Requirement: Typed output acquisition and commit
The node package SHALL expose `type Output[T any]` with exactly `func (o *Output[T]) Acquire() (batch.Batch[T], bool)` and `func (o *Output[T]) Commit(*batch.Batch[T]) bool`. Runtime-created output values SHALL be the only usable values. Acquire SHALL return at most one owned batch for an output during one handler invocation and SHALL return an empty batch and false when its preallocated source pool is exhausted or the output already has an acquired or committed batch. Commit SHALL accept exactly one valid `Batch[T]`, move and clear the caller handle, and stage it for executor publication; it SHALL return false without taking ownership for a nil receiver, empty batch, or second commit. `Outputs1` through `Outputs8` SHALL contain exported `Out1` through `Out8` fields of the corresponding `Output` types, while `Outputs0` SHALL remain empty.

#### Scenario: Acquire and publish a transformed output
- **WHEN** a handler acquires a batch from `outputs.Out1`, fills it, and commits it
- **THEN** Commit clears the local handle and the executor can publish that typed batch on output one after the handler returns

#### Scenario: Move a compatible input to an output
- **WHEN** a handler passes its `batch.Batch[int]` input to `outputs.Out1.Commit`
- **THEN** the input handle is cleared and the staged output retains ownership without a conversion or runtime type assertion

### Requirement: Automatic output cleanup boundary
The executor SHALL release an output batch acquired through Output.Acquire but not committed by handler return. It SHALL also release committed output batches when the handler returns a failed Result, the connected queue drops the batch, startup rolls back, or close discards pending work. A handler SHALL not need access to an edge pool to release an uncommitted acquired output. A successful committed output transfers ownership only when its edge queue accepts it.

#### Scenario: Abandon an acquired output
- **WHEN** a handler acquires an output batch and returns without committing it
- **THEN** the executor returns that batch to its original pool and later acquisition can reuse it

#### Scenario: Reject a duplicate output commit
- **WHEN** a handler commits one batch to an output and attempts a second Commit in the same invocation
- **THEN** the second call returns false and leaves ownership of its argument with the handler
