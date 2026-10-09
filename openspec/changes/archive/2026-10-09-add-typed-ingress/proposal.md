## Why
Stage 1's external producer Acquire/Commit/Submit path remains deferred; only polled zero-input sources can currently introduce data. A typed ingress makes a running definition usable by external producers.
## What Changes
- Register a typed ingress definition and obtain a fresh executor-bound producer endpoint per Start.
- Support allocation-free Acquire/fill/Commit and copying Submit with bounded ownership and shutdown admission.
## Capabilities
### New Capabilities
- `typed-ingress`: Single-producer typed ingress, bounded submission, and stop ownership.
### Modified Capabilities
- `typed-pipeline-contracts`: Add a dormant typed ingress source, executor-local producer binding, and status-based ownership semantics.
## Impact
Affects graph registration, runtime materialization, pools, and drain admission. Depends on `fix-moved-input-ownership` and `add-runtime-metrics`. Multi-producer ingress remains outside this change.
