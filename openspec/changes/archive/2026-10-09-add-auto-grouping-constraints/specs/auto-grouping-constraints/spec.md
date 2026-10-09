## Purpose
Define immutable placement and physical boundary constraints consumed by opt-in automatic grouping.
## ADDED Requirements
### Requirement: Honor explicit placement as a hard constraint
Automatic constraints SHALL preserve explicit manual Place labels and mandatory physical worker boundaries. Effective default placement SHALL not falsely become an explicit hard label. Conflicting must-link/manual-label or must-link/must-separate requirements SHALL reject without topology mutation or callback invocation.
#### Scenario: Preserve distinct manual groups
- **WHEN** adjacent nodes have different explicit manual placements
- **THEN** constraint normalization prohibits merging them into one physical worker
### Requirement: Copy dormant planning constraints
Constraint previews SHALL be independent immutable numeric copies formed after group-replica/node-pool physical expansion. `EnableAutomaticPlanning` SHALL mark unplaced nodes as free in the preview without changing effective manual Build/Start placement. Manual grouping SHALL remain the default and its existing placements SHALL be unchanged. Preview SHALL not merge nodes, construct queues, invoke factories, or enable automatic runtime scheduling.
#### Scenario: Inspect constraints
- **WHEN** a caller requests and mutates a returned constraint preview
- **THEN** the builder's topology and later previews remain unchanged and no factory was invoked

#### Scenario: Leave unplaced nodes free in automatic preview
- **WHEN** automatic planning preview is enabled but several nodes have no explicit Place labels
- **THEN** their preview constraint rows use an invalid/no-hard-label group while normal Build/Start keeps the manual default group
