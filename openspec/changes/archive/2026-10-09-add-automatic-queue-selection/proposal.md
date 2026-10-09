## Why
Stage 4 requires choosing the least concurrent queue model from expanded producer/consumer counts. The current immutable plan assumes one producer and one consumer everywhere.
## What Changes
- Derive queue models from physical endpoint cardinality and expose copied selection metadata.
- Materialize appropriate built-in rings without invoking factories during Build.
## Capabilities
### New Capabilities
- `automatic-queue-selection`: Deterministic queue selection for expanded endpoints.
### Modified Capabilities
- `typed-pipeline-contracts`: Expose immutable physical edge model/cardinality plans and typed built-in queue materialization while preserving the fixed manual topology.
- `immutable-graph-definition`: Add an ordered copied edge queue-plan inspection accessor without exposing runtime resources.
## Impact
Affects build planning and dormant typed bindings. Depends on `add-bounded-concurrent-queues`. Supplies cardinality planning used by replicas, pools, and fan-in; does not itself create replicas.
