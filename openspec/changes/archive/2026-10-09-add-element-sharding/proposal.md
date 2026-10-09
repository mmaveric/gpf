## Why
Stage 6 requires distributing individual elements, rather than whole batches, to typed branches. Shared work queues cannot provide stable key-based branch selection.
## What Changes
- Add a typed per-element selector and branch-specific pooled batches whose selector indices map to the supplied destination order.
- Split full branch batches without unbounded buffering and preserve per-branch Drop/Wait accounting.
## Capabilities
### New Capabilities
- `element-sharding`: Typed element distribution with bounded branch staging.
### Modified Capabilities
- `runtime-metrics`: Distinguish element loss caused by branch pool exhaustion from queue Drop and account for unassigned elements on forced shutdown.
## Impact
Affects ordinary output topology, branch materialization, and pending-work cleanup. Depends on `fix-moved-input-ownership` and `add-runtime-metrics`; error distribution, resharding, and global ordering remain deferred.
