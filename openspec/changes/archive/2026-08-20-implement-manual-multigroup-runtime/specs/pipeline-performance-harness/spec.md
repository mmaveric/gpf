## ADDED Requirements

### Requirement: Measure built-in remote SPSC execution
After the manual-multigroup-runtime capability is implemented, the benchmark harness SHALL execute rather than mark unavailable the `remote-queue` case using a built-in typed SPSC transfer and the `end-to-end-dag` case using a fixed two-group DAG. Those benchmark names SHALL retain batch size and group-count dimensions, call `b.ReportAllocs`, and exclude definition building and Start setup from measured iterations. The repository SHALL add `testing.AllocsPerRun` assertions for the same built-in remote transfer and end-to-end paths and SHALL require zero framework allocations per operation.

#### Scenario: Run the remote benchmark matrix entries
- **WHEN** a developer runs the benchmark matrix after multi-group runtime implementation
- **THEN** remote-queue and end-to-end-dag execute built-in fixed-group work after setup, report allocations, and retain their batch and group dimensions

#### Scenario: Detect a remote allocation regression
- **WHEN** the built-in remote SPSC transfer or two-group end-to-end path allocates during a measured operation
- **THEN** its allocation assertion fails while handler, selector, factory, startup, shutdown, and custom queue allocation remain outside that assertion
