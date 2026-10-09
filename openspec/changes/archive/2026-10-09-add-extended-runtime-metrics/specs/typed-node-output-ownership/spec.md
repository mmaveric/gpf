## MODIFIED Requirements

### Requirement: Automatic output cleanup boundary
The executor SHALL release an output batch acquired through Output.Acquire but not committed or independently moved by handler return. If the handler explicitly releases its acquired copy, finalization SHALL not release the retained cleanup copy again. If a handler acquires scratch output storage but commits a different compatible input batch, finalization SHALL reclaim the unused acquisition separately from the committed input. It SHALL also release committed output batches when the handler returns a failed Result, the connected queue drops the batch, startup rolls back, or close discards pending work. A handler SHALL not need access to an edge pool to release an uncommitted acquired output. A successful committed output transfers ownership only when its edge queue accepts it.

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
