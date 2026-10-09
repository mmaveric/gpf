## Why
Stage 7 makes manual placement a hard constraint on automatic grouping. A planner needs immutable constraints before it can safely merge adjacent nodes.
## What Changes
- Represent explicit placements and mandatory worker boundaries in a copied planning constraint model.
- Reject conflicting constraints without invoking runtime factories or mutating topology.
## Capabilities
### New Capabilities
- `auto-grouping-constraints`: Manual placement and boundary constraints for opt-in planning.
### Modified Capabilities
- `typed-pipeline-contracts`: Expose immutable hard-constraint previews while leaving automatic merging and execution disabled.
- `immutable-graph-definition`: Add a copied constraint-preview accessor over the retained logical/physical plan.
## Impact
Affects builder planning metadata and validation. Depends on `add-group-replicas` and `add-node-worker-pools`; fixed manual grouping remains the default.
