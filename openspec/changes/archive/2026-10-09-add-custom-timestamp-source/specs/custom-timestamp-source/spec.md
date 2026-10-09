## Purpose
Specify immutable user timestamp callbacks for deterministic and specialized processing timing.
## ADDED Requirements
### Requirement: Injectable nanosecond timestamp function
Timing configuration SHALL accept an optional func() uint64 source; nil SHALL preserve the default monotonic clock. Build and snapshot reads SHALL not invoke it. Every sampled returned handler SHALL invoke it once at entry and once at return; disabled/unsampled calls SHALL not invoke it.
#### Scenario: Use a deterministic clock
- **WHEN** a custom clock returns 100 and 140 around a measured handler
- **THEN** its measured duration increases by forty nanoseconds and exactly two callback calls occur
### Requirement: Explicit custom clock obligations
Documentation SHALL require concurrent safety when shared across workers, nonblocking/nonallocating execution, nanosecond units, and modular monotonic intervals below 2^63. Framework SHALL preserve concrete callback dispatch without reflection or boxed timestamps.
#### Scenario: Leave timing disabled
- **WHEN** an executor has a configured custom clock but timing is disabled
- **THEN** processing and snapshots make no custom-clock calls
