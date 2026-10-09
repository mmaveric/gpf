## Why
Stage 7 requires explicit CPUs for each automatically formed group. Previewed topology is not executable until those assignments are validated and frozen.
## What Changes
- Bind an automatic plan's physical group IDs to explicit unique CPU IDs.
- Validate plan freshness and CPU completeness before enabling Build/Start for automatic grouping.
## Capabilities
### New Capabilities
- `automatic-group-cpu-bindings`: CPU-complete automatic definitions and affinity integration.
### Modified Capabilities
- `typed-pipeline-contracts`: Add explicit CPU-complete automatic finalization without changing manual Build/Start defaults.
- `immutable-graph-definition`: Retain a CPU-bound automatic physical definition while preserving logical topology inspection.
- `cpu-affinity-runtime`: Report affinity outcomes for every finalized automatic physical group.
## Impact
Affects auto-plan finalization, affinity snapshots, and Build validation. Depends on `add-automatic-group-planning`; CPU discovery and NUMA placement remain deferred.
