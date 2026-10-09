## Why
Stage 6's separate-state point already has normative factory contracts, but expanded group and node execution needs executable isolation coverage. Testing the existing requirement avoids inventing a second state-ownership API.
## What Changes
- Verify independent explicit and closure state for every replica and restart.
- Exercise partial startup failure, owner-worker lifecycle, and exactly-once cleanup.
## Capabilities
### New Capabilities
None.
### Modified Capabilities
None. This verification-only change uses `skip_specs: true` and tests `node-factory-contracts` without changing behavior.
## Impact
Adds focused tests and state-ownership documentation. Depends on `add-group-replicas` and `add-node-worker-pools`.
