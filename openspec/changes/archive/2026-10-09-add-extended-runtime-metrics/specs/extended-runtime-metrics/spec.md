## Purpose
Extend numeric runtime snapshots with bounded pool utilization and fixed application counters.
## ADDED Requirements
### Requirement: Pool utilization observations
Snapshots SHALL expose pool identity, capacity, sampled in-use count, and sampled high-water utilization bounded by capacity. Updates SHALL allocate no framework memory after setup and remote release SHALL remain race-safe. Gauges SHALL not be presented as a globally transactional sample.
#### Scenario: Observe exhaustion and recovery
- **WHEN** a pool exhausts and owned slots are subsequently returned
- **THEN** snapshots retain its observed high-water and show bounded current utilization without changing existing exhaustion counters
### Requirement: Fixed registered numeric user counters
Applications SHALL register numeric counter slots before Build and update replica-local counters through bound runtime functions. Registration SHALL reject invalid or duplicate IDs atomically. Snapshot reads SHALL return independent ordered numeric rows; updates SHALL not create labels or grow storage.
#### Scenario: Increment registered counters
- **WHEN** a handler updates its registered counter after setup
- **THEN** the appropriate replica snapshot reflects the increment with zero framework allocation
### Requirement: Sampled worker polling durations
Optional worker timing SHALL record measured idle and progressing scheduler-pass nanoseconds and measured pass counts using the established monotonic timing infrastructure. Collection SHALL be disabled by default and unsampled passes SHALL not read the clock. These measurements SHALL be documented as polling wall durations, not OS CPU usage, and SHALL not extrapolate unsampled time.
#### Scenario: Measure an idle polling pass
- **WHEN** a sampled scheduler pass reports no progress
- **THEN** its measured elapsed duration contributes only to idle polling time and its measured pass count increases once
