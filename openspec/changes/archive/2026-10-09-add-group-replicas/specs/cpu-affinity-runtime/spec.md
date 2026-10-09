## MODIFIED Requirements

### Requirement: Executor exposes immutable affinity outcomes
`AffinityStatus` SHALL include logical `GroupID`, zero-based `Replica`, assigned `CPU`, `Outcome`, and optional `Warning`. Start SHALL record exactly one status per physical group instance in deterministic ascending physical group-plan order before returning successfully. Applied status SHALL have nil warning; fallback status SHALL retain `ErrAffinityUnavailable` and its adapter cause. Returned slices SHALL be independent, statuses immutable after Start, and remain available after stop. No affinity callback/log/event surface is added.

#### Scenario: Inspect replicated worker affinity
- **WHEN** two replicas of one logical group start with explicit CPUs
- **THEN** affinity outcomes contain the same logical group twice with distinct replica indices and assigned CPUs

#### Scenario: Inspect mixed affinity results
- **WHEN** one physical group applies affinity and another falls back
- **THEN** the independent status snapshot reports each result and preserves the fallback warning cause

#### Scenario: Protect the returned status slice
- **WHEN** a caller mutates an AffinityStatus slice returned during replicated execution
- **THEN** subsequent reads preserve the original logical group, replica, CPU, outcome, and warning data
