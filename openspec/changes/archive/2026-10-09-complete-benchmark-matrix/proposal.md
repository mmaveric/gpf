## Why
Stage 0 reserves node-call and local-edge benchmarks, but both still skip despite executable handlers and local transport. Finishing those entries makes the existing allocation contract measurable.
## What Changes
- Execute the reserved node-call and local-edge cases with typed batches after setup.
- Add serial zero-allocation gates and retain reproducible benchmark reports.
## Capabilities
### New Capabilities
None.
### Modified Capabilities
- `pipeline-performance-harness`: Require executable node-call and local-edge measurements.
## Impact
Changes benchmark fixtures and allocation tests, not application APIs. Depends on `fix-moved-input-ownership`; coordinate fixtures with `add-runtime-metrics` without changing its scope.
