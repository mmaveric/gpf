## Why

Generated input handlers receive batch handles by value, so moving the handler's copy to an output leaves the runner's cleanup copy pointing at the same pool slot. Unconditional cleanup then resets and frees a batch already forwarded downstream, corrupting its logical length and potentially allowing concurrent reuse; this also blocks the moved-batch verification in `add-runtime-metrics`.

## What Changes

- Add invocation-local transfer bookkeeping so runner cleanup distinguishes an untouched input from an input moved or explicitly released by its handler.
- Preserve existing by-value explicit-state and closure handler signatures, typed output Commit, originating-pool release, and zero-allocation steady-state execution.
- Ensure accepted input commits relinquish automatic input cleanup while rejected commits leave the input owned for normal cleanup.
- Preserve ownership through local/remote forwarding, Wait retention, failed-result finalization, Drop, and forced Close.
- Add standalone ownership regressions that do not depend on the unfinished metrics API.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `typed-node-output-ownership`: Specify invocation-local input cleanup and accepted/rejected transfer behavior across output finalization.
- `typed-pipeline-contracts`: Clarify batch transfer bookkeeping for by-value handler inputs while retaining the absence of general runtime misuse checking.

## Impact

Affected areas are `batch` handle transfer/release bookkeeping, generated graph input invocation and cleanup, the arity generator, and ownership tests. The fix adds no dependencies and does not change handler signatures, queue models, grouping, CPU placement, or lifecycle APIs. It is a prerequisite to completing `add-runtime-metrics`; that change's partial code and task state must be preserved. Correctness tests for this change must independently demonstrate nonzero forwarded lengths, original values, exclusive pool ownership, and eventual reclamation.
