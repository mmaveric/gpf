## ADDED Requirements

### Requirement: Explicit asynchronous logger configuration
The graph contract SHALL expose `LoggerConfig` with per-worker `Capacity uint32` and `Handle func(node.ErrorEvent)`, and `SetLogger(*Builder, LoggerConfig) error`. An open initialized builder SHALL accept at most one valid logger configuration. Zero/unrepresentable capacity and a nil handler SHALL reject without storing configuration; invalid/sealed builders and duplicate valid configuration SHALL report their corresponding categories. Configuration SHALL be copied, with user callback captures remaining immutable caller-owned configuration. This explicit capability enables bounded asynchronous logging for configured definitions; definitions without configuration SHALL retain the deferred/dormant logging behavior. Worker publication SHALL require no boxed E or formatted event payload.

#### Scenario: Reject an invalid logger
- **WHEN** SetLogger receives a zero-capacity configuration or nil callback
- **THEN** it reports an invalid logger and the builder remains available for a corrected configuration

### Requirement: Ordered numeric logger outcomes
Executor Metrics SHALL include independently copied per-worker logger rows with group identity and attempted, accepted, full-ring dropped, delivered, and shutdown-discarded event counts. Disabled logging SHALL return no logger rows. After completed stop, accepted events SHALL equal delivered plus shutdown-discarded events; attempted events SHALL equal accepted plus full-ring dropped events. Nil MapError configuration SHALL produce a zero numeric payload with runtime-owned identity when logging is installed.

#### Scenario: Inspect a stopped saturated logger
- **WHEN** a logger stops after ring-full losses and forced discard
- **THEN** final counters distinguish producer drops from accepted-event delivery/discard, and modifying returned rows does not alter future snapshots
