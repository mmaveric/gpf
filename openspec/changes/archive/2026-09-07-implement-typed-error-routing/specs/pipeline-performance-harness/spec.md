## ADDED Requirements

### Requirement: Measure typed error-routing data paths
After typed-error-routing is implemented, the benchmark harness SHALL execute built-in local and remote typed error-routing cases rather than mark them unavailable. Case names SHALL encode the batch size and applicable group count, call `b.ReportAllocs`, and exclude definition building and Start setup from measured iterations. The repository SHALL add `testing.AllocsPerRun` assertions for local failed-result-to-error-sink delivery, remote error SPSC transfer, error Drop, and error Wait retry and SHALL require zero framework allocations per operation.

#### Scenario: Run the error-routing benchmark matrix entries
- **WHEN** a developer runs the benchmark matrix after this capability is implemented
- **THEN** local and remote typed error-routing cases execute after setup, report allocations, and retain batch and group dimensions in their names

#### Scenario: Detect an error-routing allocation regression
- **WHEN** a built-in failed-result publication, error queue transfer, Drop release, or Wait retry allocates during a measured operation
- **THEN** its allocation assertion fails while handlers, selectors, factories, startup, shutdown, and custom queue allocation remain outside that assertion
