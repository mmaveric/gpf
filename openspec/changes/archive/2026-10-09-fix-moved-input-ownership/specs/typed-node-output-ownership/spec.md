## ADDED Requirements

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
