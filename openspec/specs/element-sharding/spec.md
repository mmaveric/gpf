# element-sharding Specification

## Purpose
Define fixed typed per-element distribution into bounded branch-specific pooled batches.

## Requirements

### Requirement: Exactly one branch choice per element
An explicit sharding connection SHALL validate a fixed compatible destination list and typed selector before Build. Each processed element SHALL be assigned once to one valid branch, preserving element values without boxing. Invalid selector indices SHALL be documented as contract violations. Destination registration failure SHALL not partially connect topology.
#### Scenario: Map selector indices to supplied input order
- **WHEN** destinations are supplied in a non-canonical order and the selector returns index `i`
- **THEN** the element is sent to the destination at position `i` in the supplied list
#### Scenario: Reject an invalid selector result
- **WHEN** the selector returns an index outside the fixed destination list
- **THEN** execution panics with `ErrInvalidShardIndex` instead of silently remapping the element
#### Scenario: Resume a waiting shard
- **WHEN** a selected branch waits for queue space
- **THEN** resumption uses the retained branch choice without invoking the selector again for that element

### Requirement: Bounded branch staging and cleanup
Sharding SHALL hold bounded per-branch staging and at most one active source batch, splitting full batches and flushing partial batches at input completion. Branch Wait/Drop and pool exhaustion SHALL have distinct element-loss accounting. Close SHALL release all source, partial, and pending batches exactly once. Global cross-branch order SHALL not be guaranteed.
#### Scenario: Distribute beyond branch batch capacity
- **WHEN** an input contains more elements for one branch than its staging batch can hold
- **THEN** the branch receives successive bounded batches or configured losses without allocating additional staging storage
#### Scenario: Count branch pool exhaustion separately
- **WHEN** a selected branch has no staging slot available for an element
- **THEN** only that element is dropped, its `PoolDroppedElements` increases, and queue-drop element counts remain unchanged
#### Scenario: Reclaim pending sharding state on Close
- **WHEN** Close reaches a pending sharding operation with queued, staged, and unassigned elements
- **THEN** every owned batch is released once and unassigned source elements are counted in `ShardShutdownElements`

### Requirement: Fixed-topology endpoint compatibility
Sharding sources and destinations SHALL NOT participate in whole-group replica or node-worker-pool expansion in this version. Build SHALL reject such a topology with `ErrShardParallelExpansion` before runtime resources are created.
#### Scenario: Reject an expanded sharding endpoint
- **WHEN** a sharding source or destination is included in a replica or node-worker-pool expansion
- **THEN** Build rejects the topology with `ErrShardParallelExpansion`
