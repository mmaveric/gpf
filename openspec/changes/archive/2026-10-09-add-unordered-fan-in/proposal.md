## Why
Stage 6 requires merging parallel branch results without reconstructing original order. Multiple typed producers must be able to feed one compatible logical input through an explicit API.
## What Changes
- Add explicit homogeneous fan-in connections backed by the minimal concurrent queue model.
- Validate all producers atomically and drain without batch duplication or loss outside configured Drop.
## Capabilities
### New Capabilities
- `unordered-fan-in`: Multiple typed producers on one logical input with unordered delivery.
### Modified Capabilities
- `typed-pipeline-contracts`: Add explicit homogeneous FanIn as the only multi-producer input path while retaining ordinary Connect exclusivity.
## Impact
Affects connection cardinality, queue selection, DAG validation, and ownership. Depends on `add-automatic-queue-selection`, `fix-moved-input-ownership`, and `add-runtime-metrics`.
