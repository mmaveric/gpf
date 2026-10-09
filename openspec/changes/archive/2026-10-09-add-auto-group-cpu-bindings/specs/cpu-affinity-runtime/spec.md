## MODIFIED Requirements

### Requirement: Executor exposes immutable affinity outcomes
Every successfully finalized automatic physical group SHALL create one locked worker and one immutable ordered AffinityStatus row with logical/physical group identity, replica identity, explicit CPU, outcome, and warning. An applied affinity result SHALL carry no warning; fallback SHALL preserve ErrAffinityUnavailable and any adapter cause. The returned slice SHALL be independently mutable without changing executor state and remain available after stop. Unsupported-platform locked-thread fallback remains portable and does not fail Start.

#### Scenario: Inspect automatic affinity fallback
- **WHEN** a finalized automatic plan starts with an adapter that rejects each explicit CPU
- **THEN** every physical worker remains locked and has an ordered fallback status for its assigned CPU

#### Scenario: Inspect node-pool affinity outcomes
- **WHEN** a logical node has three CPU-bound worker instances
- **THEN** affinity outcomes identify the same logical node with worker indices zero through two and their assigned CPUs

#### Scenario: Inspect replicated worker affinity
- **WHEN** two replicas of one logical group start with explicit CPUs
- **THEN** affinity outcomes contain the same logical group twice with distinct replica indices and assigned CPUs

#### Scenario: Inspect mixed affinity results
- **WHEN** physical workers receive mixed applied and fallback outcomes
- **THEN** the ordered snapshot preserves each outcome and fallback warning

#### Scenario: Protect the returned status slice
- **WHEN** a caller mutates a returned automatic affinity status
- **THEN** subsequent snapshots retain the original result

#### Scenario: Inspect mixed affinity results
- **WHEN** applied and fallback outcomes occur across physical workers
- **THEN** the snapshot preserves each outcome and fallback cause

#### Scenario: Protect the returned status slice
- **WHEN** a caller mutates a returned affinity result slice
- **THEN** subsequent reads preserve the immutable recorded results
