## ADDED Requirements

### Requirement: Measure instrumented runtime paths
Allocation gates SHALL run with runtime metrics enabled on built-in local and remote ordinary and error delivery, queue Drop, Wait retry, and source-pool exhaustion after setup, and SHALL require zero framework allocations per operation. Snapshot construction SHALL remain outside steady-state allocation measurement. The benchmark harness SHALL retain ReportAllocs and batch/group dimensions and SHALL record comparable before-and-after throughput and allocation results for the instrumentation change without a throughput pass/fail threshold. Allocation assertions SHALL remain serial under the existing allocation measurement contract.

#### Scenario: Detect instrumentation allocations
- **WHEN** a metric update allocates on an instrumented built-in measured operation
- **THEN** its zero-allocation assertion fails even if the transport itself is allocation-free

#### Scenario: Compare instrumentation overhead
- **WHEN** benchmark results for matching pre-change and instrumented cases are compared
- **THEN** batch and group configuration, allocations, and throughput are reported with setup and snapshots excluded from timed iterations
