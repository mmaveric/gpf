## Why
Stage 5 calls for both full and sampled handler timing. Existing counter snapshots cannot distinguish expensive processing from queue pressure.
## What Changes
- Add disabled/full/every-N-call timing modes and per-node measured duration totals.
- Use an executor-local monotonic default clock and keep retries and selectors outside handler timing.
## Capabilities
### New Capabilities
- `processing-timing`: Full and deterministic sampled handler duration measurements.
### Modified Capabilities
- `typed-pipeline-contracts`: Add opt-in immutable timing configuration and measured node snapshot fields while retaining disabled-by-default behavior.
## Impact
Affects generated dispatch and metrics snapshots. Depends on `add-runtime-metrics`. Custom clock injection belongs to `add-custom-timestamp-source`.
