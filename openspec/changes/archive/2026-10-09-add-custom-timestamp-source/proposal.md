## Why
Stage 5 explicitly requires a user-supplied timestamp function. Deterministic tests and specialized clocks should not require replacing generated handler code.
## What Changes
- Allow a configured func() uint64 nanosecond clock for timing.
- Document worker concurrency and clock correctness obligations with a default fallback.
## Capabilities
### New Capabilities
- `custom-timestamp-source`: Immutable timestamp callback configuration and invocation rules.
### Modified Capabilities
- `typed-pipeline-contracts`: Extend TimingConfig with optional custom clock configuration and its concrete callback obligations.
## Impact
Affects timing configuration and clock tests. Depends on `add-processing-timing`; it does not select OS-specific clocks or enable platform fast paths.
