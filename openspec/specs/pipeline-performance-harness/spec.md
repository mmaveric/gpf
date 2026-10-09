## Purpose
Define the benchmark matrix and allocation-measurement requirements for pipeline data-plane paths.

## Requirements

### Requirement: Stable data-plane benchmark matrix
The repository SHALL provide a Go benchmark harness with named cases for node invocation, pool acquire/release, local-edge transfer, remote-queue transfer, and end-to-end DAG processing. The harness SHALL encode batch size and applicable topology dimensions in benchmark case names. The `pool-acquire-release` case SHALL execute acquire, fixed-capacity fill, logical-length update, and release against a constructed pool; cases without an executable owner SHALL remain explicitly unavailable.

#### Scenario: Discover benchmark coverage
- **WHEN** a developer lists the framework benchmarks
- **THEN** they can identify a benchmark entry point for each planned data-plane path and its batch or topology configuration, including an executable pool acquire/release case

### Requirement: Allocation measurement contract
The harness SHALL report allocations and SHALL expose `testing.AllocsPerRun` assertions for approved steady-state paths. An implementation stage SHALL enable a zero-allocation assertion only after the corresponding path is executable and has completed setup before measurement begins. The batch subsystem SHALL assert zero allocations for repeated pool acquire/fill/release work and for the pool-exhaustion path.

Every `testing.AllocsPerRun` assertion SHALL execute while its test package has no active parallel tests. A test that invokes the allocation harness SHALL remain serial and SHALL preserve the existing warm-up, run count, measured operation, and zero-allocation threshold across supported Go versions.

#### Scenario: Measure a steady-state path
- **WHEN** an implemented data-plane benchmark runs after its setup phase
- **THEN** its allocation assertion measures only repeated steady-state work and fails if allocations exceed zero

#### Scenario: Measure pool lifecycle allocations
- **WHEN** the pool benchmark and allocation tests run against a constructed typed pool
- **THEN** acquire, fixed-capacity fill, logical-length update, release, and exhaustion reporting each perform zero allocations per operation

#### Scenario: Run allocation assertions on Go 1.26
- **WHEN** the repository test suite executes with a Go version that rejects `testing.AllocsPerRun` during parallel-test activity
- **THEN** every allocation assertion runs serially, completes without a parallel-test panic, and retains its zero-allocation gate

### Requirement: Measure built-in remote SPSC execution
After the manual-multigroup-runtime capability is implemented, the benchmark harness SHALL execute rather than mark unavailable the `remote-queue` case using a built-in typed SPSC transfer and the `end-to-end-dag` case using a fixed two-group DAG. Those benchmark names SHALL retain batch size and group-count dimensions, call `b.ReportAllocs`, and exclude definition building and Start setup from measured iterations. The repository SHALL add `testing.AllocsPerRun` assertions for the same built-in remote transfer and end-to-end paths and SHALL require zero framework allocations per operation.

#### Scenario: Run the remote benchmark matrix entries
- **WHEN** a developer runs the benchmark matrix after multi-group runtime implementation
- **THEN** remote-queue and end-to-end-dag execute built-in fixed-group work after setup, report allocations, and retain their batch and group dimensions

#### Scenario: Detect a remote allocation regression
- **WHEN** the built-in remote SPSC transfer or two-group end-to-end path allocates during a measured operation
- **THEN** its allocation assertion fails while handler, selector, factory, startup, shutdown, and custom queue allocation remain outside that assertion

### Requirement: Non-blocking performance reporting
The benchmark harness SHALL allow throughput and latency measurements to be recorded for manual comparison without making them merge-blocking thresholds. The project SHALL document that a zero-allocation gate alone does not protect against instruction-count, cache-miss, throughput, or tail-latency regressions.

#### Scenario: Interpret a passing allocation gate
- **WHEN** a data-plane path passes its zero-allocation assertion
- **THEN** the benchmark documentation states that further throughput and latency review can still be required

### Requirement: Measure typed error-routing data paths
After typed-error-routing is implemented, the benchmark harness SHALL execute built-in local and remote typed error-routing cases rather than mark them unavailable. Case names SHALL encode the batch size and applicable group count, call `b.ReportAllocs`, and exclude definition building and Start setup from measured iterations. The repository SHALL add `testing.AllocsPerRun` assertions for local failed-result-to-error-sink delivery, remote error SPSC transfer, error Drop, and error Wait retry and SHALL require zero framework allocations per operation.

#### Scenario: Run the error-routing benchmark matrix entries
- **WHEN** a developer runs the benchmark matrix after this capability is implemented
- **THEN** local and remote typed error-routing cases execute after setup, report allocations, and retain batch and group dimensions in their names

#### Scenario: Detect an error-routing allocation regression
- **WHEN** a built-in failed-result publication, error queue transfer, Drop release, or Wait retry allocates during a measured operation
- **THEN** its allocation assertion fails while handlers, selectors, factories, startup, shutdown, and custom queue allocation remain outside that assertion

### Requirement: Measure instrumented runtime paths
Allocation gates SHALL run with runtime metrics enabled on built-in local and remote ordinary and error delivery, queue Drop, Wait retry, and source-pool exhaustion after setup, and SHALL require zero framework allocations per operation. Snapshot construction SHALL remain outside steady-state allocation measurement. The benchmark harness SHALL retain ReportAllocs and batch/group dimensions and SHALL record comparable before-and-after throughput and allocation results for the instrumentation change without a throughput pass/fail threshold. Allocation assertions SHALL remain serial under the existing allocation measurement contract.

#### Scenario: Detect instrumentation allocations
- **WHEN** a metric update allocates on an instrumented built-in measured operation
- **THEN** its zero-allocation assertion fails even if the transport itself is allocation-free

#### Scenario: Compare instrumentation overhead
- **WHEN** benchmark results for matching pre-change and instrumented cases are compared
- **THEN** batch and group configuration, allocations, and throughput are reported with setup and snapshots excluded from timed iterations

### Requirement: Execute node-call and local-edge matrix entries
The reserved node-call/batch=64 and local-edge/batch=64/group=1 entries SHALL execute concrete typed handlers and built-in local transport instead of skipping. Each measured operation SHALL process a logical 64-element batch, exclude setup and teardown, retain ReportAllocs, and expose a serial zero-framework-allocation assertion.
#### Scenario: Run the completed matrix
- **WHEN** the benchmark matrix runs after setup
- **THEN** both reserved cases execute, report allocations, and verify the advertised batch size without timing snapshots or lifecycle work
#### Scenario: Detect a local allocation regression
- **WHEN** an executable node-call or local-edge fixture allocates during its measured framework operation
- **THEN** its serial allocation assertion fails against a zero threshold
