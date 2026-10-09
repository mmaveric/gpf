## Why
Stage 5 requires logging outside pinned data-plane workers. Without a bounded event transport, a slow logging callback can block or allocate in processing.
## What Changes
- Add per-worker bounded event rings and one executor-owned asynchronous logging worker.
- Count full-ring drops and integrate logging completion with drain and forced close.
## Capabilities
### New Capabilities
- `async-error-logging`: Bounded event transport, logger lifecycle, and loss reporting.
### Modified Capabilities
- `typed-pipeline-contracts`: Add explicit per-definition logger configuration and numeric logging outcomes while retaining disabled-by-default behavior.
- `immutable-graph-definition`: Retain a copied dormant logger callback/configuration without creating runtime logger resources at Build.
## Impact
Affects generated mapping publication, runtime workers, metrics, and shutdown. Depends on `add-error-mapping`, `add-runtime-metrics`, and `add-drain-timeout`.
