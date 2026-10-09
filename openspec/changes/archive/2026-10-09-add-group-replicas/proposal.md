## Why
Stage 6 requires fixed replication of complete execution groups. The existing plan starts exactly one worker per logical group and cannot scale a connected subgraph as a unit.
## What Changes
- Expand fixed groups into explicitly CPU-bound physical replicas before runtime creation.
- Create replica-local states, paired internal edges, and distributed boundary work with no ordering guarantee.
## Capabilities
### New Capabilities
- `group-replication`: Static group expansion, worker placement, and replica lifecycle.
### Modified Capabilities
- `typed-pipeline-contracts`: Define fixed physical group expansion, queue ownership cardinality, and dormant replica CPU bindings.
- `immutable-graph-definition`: Retain an immutable replica-expanded execution plan and preserve logical topology accessors.
- `cpu-affinity-runtime`: Include logical group and replica identity in per-worker affinity outcomes.
- `node-factory-contracts`: Require independent explicit/closure state and owner-worker lifecycle for each physical node replica.
## Impact
Affects immutable physical plans, generated materialization, metrics identities, and cleanup. Depends on `fix-moved-input-ownership`, `add-automatic-queue-selection`, and `add-runtime-metrics`. No dynamic scaling or source broadcast is introduced.
