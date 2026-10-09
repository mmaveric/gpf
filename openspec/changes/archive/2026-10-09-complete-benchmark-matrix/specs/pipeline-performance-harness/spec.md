## ADDED Requirements
### Requirement: Execute node-call and local-edge matrix entries
The reserved node-call/batch=64 and local-edge/batch=64/group=1 entries SHALL execute concrete typed handlers and built-in local transport instead of skipping. Each measured operation SHALL process a logical 64-element batch, exclude setup and teardown, retain ReportAllocs, and expose a serial zero-framework-allocation assertion.
#### Scenario: Run the completed matrix
- **WHEN** the benchmark matrix runs after setup
- **THEN** both reserved cases execute, report allocations, and verify the advertised batch size without timing snapshots or lifecycle work
#### Scenario: Detect a local allocation regression
- **WHEN** an executable node-call or local-edge fixture allocates during its measured framework operation
- **THEN** its serial allocation assertion fails against a zero threshold
