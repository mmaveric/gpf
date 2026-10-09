## Why
Stage 5's full metrics set extends beyond the active counter-only change. Pool utilization, worker activity, and application-defined numeric counters need bounded inspection.
## What Changes
- Add sampled pool in-use/high-water observations, idle/work pass measures, and optional sampled worker polling durations.
- Add fixed registered user counter slots and copied snapshots without data-plane labels or maps.
## Capabilities
### New Capabilities
- `extended-runtime-metrics`: Pool utilization and bounded user/worker metrics.
### Modified Capabilities
- `typed-pipeline-contracts`: Add fixed user-counter binding and optional worker polling time snapshots.
- `typed-node-output-ownership`: Preserve acquisition ownership on explicit release and when a different compatible input is committed, as required for correct utilization accounting.
## Impact
Affects pools, runtime metric records, and configuration. Depends on `add-runtime-metrics` and `add-processing-timing`; reuse timing infrastructure for worker observations while handler duration and error/log counts remain with their owning capabilities.
