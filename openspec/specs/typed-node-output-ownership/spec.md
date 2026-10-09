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
For each generated handler invocation on an explicit-state or closure physical worker, an output acquisition SHALL be released at finalization only if its handler-owned by-value handle was neither moved, explicitly released, nor committed. A successful output Commit SHALL transfer exactly the committed batch; if a different compatible input is committed to the output, the original scratch acquisition SHALL be reclaimed independently. Explicit release/move state SHALL be invocation-local, detached before cross-worker publication, and race-free across node workers. No allocation or reflective type lookup SHALL be required. Drop, Wait, failed result, forced Close, and rollback SHALL release each owned slot exactly once.

#### Scenario: Release explicitly returned output
- **WHEN** a node-pool worker explicitly releases an acquired output before returning
- **THEN** finalization does not release the retained copy or corrupt a subsequent owner of the same slot

#### Scenario: Move input instead of scratch output
- **WHEN** a node-pool handler acquires scratch output storage and commits its compatible input instead
- **THEN** scratch storage is reclaimed separately and the input remains owned by its queue path until delivery or discard

#### Scenario: Abandon an acquired output
- **WHEN** a handler acquires an output batch and returns without committing it
- **THEN** the executor returns that batch to its original pool and later acquisition can reuse it

#### Scenario: Reject a duplicate output commit
- **WHEN** a handler commits one batch to an output and attempts a second Commit in the same invocation
- **THEN** the second call returns false and leaves ownership of its argument with the handler

#### Scenario: Respect an explicitly released acquisition
- **WHEN** an acquired output copy is released and the slot is acquired by a new owner before finalization
- **THEN** cleanup leaves the new owner's slot and utilization count unchanged

#### Scenario: Commit input instead of scratch acquisition
- **WHEN** a handler acquires scratch output storage but commits its compatible input to that output
- **THEN** finalization releases the unused scratch acquisition while preserving the committed input until publication or discard

### Requirement: Runtime-owned typed error transport batch
A connected `ErrorPort[E]` SHALL be backed at runtime by a framework-owned typed source pool. Only the generated executor binding may acquire its `Batch[E]`; user handlers receive it only through the target `InPort[E]`. For each failed result, the binding SHALL acquire at most one batch, populate exactly one element, and either transfer it to the edge queue, retain it as the runner's one Wait pending output, or release it. It SHALL not expose an error `node.Output[E]` to a handler.

#### Scenario: Preserve origin ownership through an error consumer
- **WHEN** a connected error batch crosses a local or remote edge and the consumer leaves it un-moved
- **THEN** executor cleanup returns it to the error edge's original typed pool without a runtime pool lookup or type erasure

#### Scenario: Discard a pending error batch on close
- **WHEN** Close observes a runner holding a Wait-pending typed error batch
- **THEN** the executor releases that batch exactly once and clears the pending handle without invoking another handler

### Requirement: Invocation-local input ownership cleanup
For every explicit-state and closure input handler at arities one through eight, the executor SHALL release an input at invocation completion only if the handler has neither moved it nor explicitly released it. A move from the handler's by-value input copy SHALL relinquish automatic input cleanup even though the runner retains a separate handle copy. Untouched inputs SHALL be returned exactly once to their originating pool. Explicit Batch.Release or Pool.Release of the handler input SHALL relinquish automatic cleanup. This bookkeeping SHALL not introduce reflection, runtime type assertions, boxed batch values, or steady-state heap allocation, and SHALL not change existing handler signatures.

#### Scenario: Preserve a forwarded input
- **WHEN** an input handler commits its three-element input to a compatible output and returns success
- **THEN** subsequent runner cleanup does not release the forwarded slot, and the downstream consumer observes the original length and items

#### Scenario: Release an untouched input
- **WHEN** an input handler returns without moving or explicitly releasing its input
- **THEN** invocation cleanup returns the batch to its original pool exactly once

#### Scenario: Respect explicit release
- **WHEN** an input handler explicitly releases its input through Batch.Release or Pool.Release before returning
- **THEN** invocation cleanup clears its retained handle without releasing or resetting that slot again

### Requirement: Accepted and rejected input commits preserve ownership boundaries
A successful compatible input Commit SHALL clear the handler's argument and transfer ownership to output finalization. A rejected Commit SHALL leave the argument and invocation ownership unchanged. Committed input batches SHALL remain unavailable for source-pool reacquisition while queued, pending Wait publication, or owned downstream. A failed handler result, queue Drop, or forced Close SHALL release a committed input through its output ownership path exactly once, and input cleanup SHALL not release it again.

#### Scenario: Reject a duplicate commit without consuming input
- **WHEN** a handler attempts to commit its input to an output that has already committed another batch
- **THEN** Commit returns false, leaves the input owned and unchanged, and normal cleanup releases it unless the handler subsequently moves or releases it

#### Scenario: Retain an input across Wait
- **WHEN** a committed input batch cannot enter a full Wait queue and the handler returns
- **THEN** the output retains the original batch length and contents, the source pool cannot reacquire its slot, and input cleanup does not release it

#### Scenario: Discard a committed input on failed result
- **WHEN** a handler commits its input to an ordinary output and then returns a failed Result
- **THEN** failed-output cleanup releases the original slot once and automatic input cleanup does not release it again

### Requirement: Ownership transfer is isolated across invocations and workers
Transfer bookkeeping SHALL be scoped to the invoking worker and handler call. A moved batch SHALL not carry a mutable reference to the producing invocation's bookkeeping when it crosses a queue or becomes independently owned. Reusing invocation bookkeeping SHALL not affect batches previously transferred. Local, custom-local, and remote forwarding, including a typed error input moved to an ordinary output, SHALL preserve originating-pool ownership and exact logical contents across multiple hops and sequential executor starts.

#### Scenario: Forward across groups repeatedly
- **WHEN** successive invocations move typed inputs through a multi-hop graph with a remote boundary
- **THEN** each downstream batch retains its own contents and length, no prior transferred slot is released by a later invocation, and execution remains race-free

#### Scenario: Forward a typed error input
- **WHEN** an error consumer moves its one-element error batch to a compatible ordinary output
- **THEN** downstream receives the original failure value and eventually releases the batch to its error-edge source pool
