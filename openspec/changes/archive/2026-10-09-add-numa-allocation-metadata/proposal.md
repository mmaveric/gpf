## Why
Stage 8 requires NUMA IDs and typed allocator hooks as preparation while keeping NUMA behavior inactive. Missing metadata would force a broader API redesign later.
## What Changes
- Add immutable optional NUMA annotations to pool/group/queue configuration.
- Define dormant typed allocation-hook contracts without changing placement or invoking custom NUMA allocation.
## Capabilities
### New Capabilities
- `numa-allocation-metadata`: NUMA-ready configuration with inactive allocator descriptors.
### Modified Capabilities
- `immutable-graph-definition`: Add copied GroupNUMA inspection and retain inactive NUMA/allocator metadata with the immutable plan.
## Impact
Affects configuration validation and copied definition metadata. Depends on `add-auto-group-cpu-bindings`; portable allocation and CPU behavior remain unchanged.
