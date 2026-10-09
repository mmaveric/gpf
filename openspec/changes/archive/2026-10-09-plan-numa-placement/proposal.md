## Why
Stage 8 explicitly calls for preparing a separate future NUMA placement stage, not activating it before v1.0. A dedicated design record can specify the unresolved platform and locality questions without silently adding runtime behavior.
## What Changes
- Produce a future-placement design and decision checklist grounded in NUMA-ready metadata.
- Describe topology discovery, allocation locality, fallback, benchmark evidence, and migration gates.
## Capabilities
### New Capabilities
None.
### Modified Capabilities
None. This documentation-only change uses `skip_specs: true`.
## Impact
Adds future design documentation only. Depends on `add-numa-allocation-metadata`; enabling NUMA behavior requires a later explicitly approved change.
