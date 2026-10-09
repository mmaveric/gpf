## Why
Stage 7 requires forming groups before runtime creation. A deterministic planner must be inspectable without starting workers or executing user factories.
## What Changes
- Expose an opt-in automatic-group preview from expanded topology and constraints.
- Copy stable physical node/group/edge metadata and retain factory dormancy.
## Capabilities
### New Capabilities
- `automatic-group-planning`: Read-only automatic planning and immutable preview identities.
### Modified Capabilities
- `typed-pipeline-contracts`: Expose deterministic CPU-incomplete group-plan previews while preserving manual grouping as the executable default.
## Impact
Affects Builder planning and definition metadata. Depends on `add-remote-edge-minimization` and `add-automatic-queue-selection`; CPU-complete executable Build integration belongs to `add-auto-group-cpu-bindings`.
