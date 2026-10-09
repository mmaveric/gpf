## Why
Stage 7 requires avoiding unnecessary remote edges while respecting placements and parallelism boundaries. The manual plan does not compute compatible node coalescing.
## What Changes
- Add deterministic constrained adjacent-component merging and remote-edge reporting.
- Document a greedy/local-minimal objective rather than claiming a globally optimal graph partition.
## Capabilities
### New Capabilities
- `remote-edge-minimization`: Constraint-preserving coalescing and deterministic remote-edge cost.
### Modified Capabilities
None.
## Impact
Adds a pure control-plane planning algorithm and property tests. Depends on `add-auto-grouping-constraints`; public immutable plan integration follows in `add-automatic-group-planning`.
