## Why
Stage 6 requires scaling a single expensive node without replicating its surrounding graph. Whole-group replication cannot express this isolation.
## What Changes
- Expand one logical node into a fixed set of CPU-bound workers with independent factory state.
- Use bounded shared input/output queues and preserve per-port typed selection.
## Capabilities
### New Capabilities
- `node-worker-pools`: Fixed node-level parallelism and lifecycle integration.
### Modified Capabilities
- `typed-pipeline-contracts`: Define typed shared queues and per-port selectors for fixed physical node workers.
- `immutable-graph-definition`: Retain a copied node-worker expansion plan and materialize fresh per-worker state at Start.
- `cpu-affinity-runtime`: Report affinity by logical node worker index in addition to its physical group.
- `node-factory-contracts`: Clarify fresh state and lifecycle per physical worker.
- `typed-node-output-ownership`: Preserve output-acquisition cleanup when handlers move or explicitly release output handles.
## Impact
Affects physical node planning, typed queue cardinality, metric identities, and lifecycle. Depends on `add-automatic-queue-selection`, `add-runtime-metrics`, and `fix-moved-input-ownership`; node pools cannot overlap a replicated group in this version.
