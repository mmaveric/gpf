## Why
Stage 6 requires broadcasting an ordinary batch to multiple consumers, while Connect currently enforces one consumer. Shared slot ownership is incompatible with exclusive pooled batches.
## What Changes
- Add explicit ordinary broadcast connections with one copied pooled batch per branch.
- Limit broadcast endpoints to fixed topology; replica and node-worker-pool expansion of either endpoint is rejected in this version.
- Apply independent branch pool-exhaustion and queue policies with bounded pending state.
## Capabilities
### New Capabilities
- `broadcast-fan-out`: Typed copying broadcast and independent branch ownership.
### Modified Capabilities
- `typed-pipeline-contracts`: Add explicit ordinary Broadcast copying while preserving ordinary Connect and typed error cardinality.
- `runtime-metrics`: Distinguish broadcast capacity-precondition rejection from queue and pool outcomes.
## Impact
Affects typed connection validation, ordinary output binding, metrics, and cleanup. Depends on `fix-moved-input-ownership` and `add-runtime-metrics`; ordinary Connect and typed error-port cardinality retain their current semantics.
