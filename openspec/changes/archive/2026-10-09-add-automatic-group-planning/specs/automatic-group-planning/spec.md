## Purpose
Specify independent deterministic automatic-group previews formed before any runtime materialization.
## ADDED Requirements
### Requirement: Deterministic copied plan preview
Opt-in automatic planning SHALL report physical node/group identities, edge locality, queue models, and required CPU slots after expansion and constrained coalescing. Explicit group IDs SHALL be preserved and generated IDs SHALL be stable for unchanged topology. Returned rows SHALL not alias mutable builder metadata.
#### Scenario: Repeat an unchanged preview
- **WHEN** a builder is planned twice without mutation
- **THEN** both previews have identical identities, groups, and queue choices
### Requirement: Preview without runtime side effects
Preview SHALL not seal the builder, invoke factories, allocate live pools/queues, or start workers. The copied AutomaticGroupPreview SHALL report CPUComplete and per-group CPUBound status; it SHALL not be executable or accepted as a Definition until a later CPU-binding finalization capability. A topology revision and builder ownership token SHALL identify freshness; changes to registration, connections, or placement SHALL invalidate previous previews.
#### Scenario: Mutate after preview
- **WHEN** a caller changes topology after obtaining a plan
- **THEN** the prior plan is stale and cannot be used as an executable finalization input

#### Scenario: Preview an automatic plan without CPU bindings
- **WHEN** an opt-in builder has valid graph topology and constraints but no CPU assigned to its proposed free group
- **THEN** PlanGroups returns a deterministic CPU-incomplete preview while factories remain dormant and ordinary Build/Start remains manual
