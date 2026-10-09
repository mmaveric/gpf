# broadcast-fan-out Specification

## Purpose
Specify typed copying broadcast with exclusive branch storage and independent bounded delivery outcomes.

## Requirements

### Requirement: Explicit independent broadcast copies
Broadcast SHALL atomically register a fixed list of distinct compatible ordinary input destinations with matching batch capacities. Each delivered branch SHALL receive a distinct pooled storage slot containing the source's logical elements. Copying pointer-containing elements SHALL be documented as shallow. Ordinary Connect and error-route cardinality SHALL remain unchanged.
#### Scenario: Deliver to two branches
- **WHEN** a valid three-element batch broadcasts to two available branches
- **THEN** both observe the original elements in independent owned storage slots

### Requirement: Independent bounded branch outcomes
Pool exhaustion SHALL drop only the affected branch copy. Queue Drop SHALL release that branch; Wait SHALL retry its retained copy without recopying or redelivering completed branches. Drain and Close SHALL account for all pending branch ownership exactly once. Broadcast batches SHALL respect the configured capacity precondition; a moved oversized batch SHALL be rejected on every branch with separate capacity-rejection counters.
#### Scenario: Retry one waiting branch
- **WHEN** one branch accepts its copy while another branch's Wait queue is full
- **THEN** only the waiting branch retries, and the accepted branch is neither recopied nor redelivered

### Requirement: Fixed-topology endpoint compatibility
Broadcast sources and destinations SHALL NOT participate in whole-group replica or node-worker-pool expansion in this version. Build SHALL reject such a topology with `ErrBroadcastParallelExpansion`, leaving broadcast execution unavailable rather than assigning ambiguous delivery semantics.
#### Scenario: Reject an expanded broadcast endpoint
- **WHEN** either endpoint of a registered broadcast is included in a replica or node-worker-pool expansion
- **THEN** Build returns `ErrBroadcastParallelExpansion` before runtime resources are created
#### Scenario: One branch waits
- **WHEN** one branch accepts and another branch waits on a full queue
- **THEN** retries affect only the waiting branch and cannot duplicate the already accepted delivery
