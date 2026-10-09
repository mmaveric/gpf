## ADDED Requirements

### Requirement: Bound fixed numeric counter updates
The graph contract SHALL expose RegisterCounter(*Builder, NodeID, CounterID) error before Build. Counter IDs SHALL be nonzero and unique within their node; invalid/duplicate/unknown-node registration SHALL not add slots. Generated explicit and closure handlers SHALL expose optional typed BindCounter callbacks invoked during normalization with each registered ID and its bound func(uint64) update function. Executor.Counter SHALL resolve registered handles in the control plane. Updates SHALL use fixed atomic slots without runtime registry growth or labels, wrap modulo uint64, and become no-ops after completed stop. Metrics SHALL return ordered copied Counters rows.

#### Scenario: Reuse IDs in distinct nodes
- **WHEN** two nodes register the same numeric ID
- **THEN** each receives an independent update function and snapshot value

### Requirement: Independent optional worker polling timing
SetWorkerTiming SHALL configure an optional WorkerTimingConfig with Every and Clock fields before Build. Zero Every SHALL disable collection. A nil clock SHALL retain the default monotonic source; custom clock obligations SHALL match handler timing without enabling handler measurements. Group snapshots SHALL report MeasuredPasses, IdleNanoseconds, and WorkNanoseconds based on the scheduler's reported progress. These SHALL be sampled polling wall durations, not CPU usage or extrapolated totals.

#### Scenario: Leave worker timing disabled
- **WHEN** no nonzero worker timing interval is configured
- **THEN** worker passes and snapshots perform no worker timestamp calls
