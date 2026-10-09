## MODIFIED Requirements

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
