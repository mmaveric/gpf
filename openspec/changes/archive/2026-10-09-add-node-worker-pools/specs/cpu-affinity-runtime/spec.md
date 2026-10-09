## MODIFIED Requirements

### Requirement: Executor exposes immutable affinity outcomes
AffinityStatus SHALL identify logical GroupID, group Replica, optional logical NodeID and NodeReplica worker index, CPU, Outcome, and Warning. A fixed node worker pool SHALL populate logical NodeID and zero-based NodeReplica and produce one immutable ordered outcome per physical worker, preserving which logical node was expanded. Existing group-only workers SHALL keep zero-value node identity. Applied/fallback warning semantics, defensive copies, and post-stop availability remain unchanged.

#### Scenario: Inspect node-pool affinity outcomes
- **WHEN** a logical node has three CPU-bound worker instances
- **THEN** affinity outcomes identify the same logical node with worker indices zero through two and their assigned CPUs

#### Scenario: Inspect replicated worker affinity
- **WHEN** two replicas of one logical group start with explicit CPUs
- **THEN** affinity outcomes contain the same logical group twice with distinct replica indices and assigned CPUs

#### Scenario: Inspect mixed affinity results
- **WHEN** one physical worker applies affinity and another falls back
- **THEN** the copied status rows retain both outcomes and the fallback warning

#### Scenario: Protect the returned status slice
- **WHEN** a caller mutates a returned affinity status row
- **THEN** a later snapshot preserves the original worker identity and result
