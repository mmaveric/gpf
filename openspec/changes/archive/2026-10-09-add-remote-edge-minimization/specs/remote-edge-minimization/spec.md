## Purpose
Define deterministic constraint-preserving component coalescing that reduces unnecessary remote transport.
## ADDED Requirements
### Requirement: Locally minimize legal remote boundaries
The coalescing algorithm SHALL preserve fixed labels and mandatory cuts, never increase remote-edge count, and terminate with no eligible adjacent improving merge. Results SHALL be deterministic for equivalent canonical inputs. It SHALL not claim globally optimal partitioning.
#### Scenario: Coalesce an unconstrained DAG
- **WHEN** no manual or parallelism boundary constrains an input DAG
- **THEN** the resulting plan places compatible nodes in one group with zero remote edges
### Requirement: Preserve quotient acyclicity
Every accepted merge SHALL preserve an acyclic group quotient graph as well as original node topology. Reports SHALL include before/after remote-edge counts and constraint rejection reasons without invoking factories.
#### Scenario: Reject a cyclic contraction
- **WHEN** merging an adjacent shortcut's endpoints would create a cycle through an intermediate component
- **THEN** the merge is rejected even though the original node graph is a DAG
