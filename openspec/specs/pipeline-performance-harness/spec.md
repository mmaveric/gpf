## ADDED Requirements

### Requirement: Stable data-plane benchmark matrix
The repository SHALL provide a Go benchmark harness with named cases for node invocation, pool acquire/release, local-edge transfer, remote-queue transfer, and end-to-end DAG processing. The harness SHALL encode batch size and applicable topology dimensions in benchmark case names.

#### Scenario: Discover benchmark coverage
- **WHEN** a developer lists the framework benchmarks
- **THEN** they can identify a benchmark entry point for each planned data-plane path and its batch or topology configuration

### Requirement: Allocation measurement contract
The harness SHALL report allocations and SHALL expose `testing.AllocsPerRun` assertions for approved steady-state paths. An implementation stage SHALL enable a zero-allocation assertion only after the corresponding path is executable and has completed setup before measurement begins.

#### Scenario: Measure a steady-state path
- **WHEN** an implemented data-plane benchmark runs after its setup phase
- **THEN** its allocation assertion measures only repeated steady-state work and fails if allocations exceed zero

### Requirement: Non-blocking performance reporting
The benchmark harness SHALL allow throughput and latency measurements to be recorded for manual comparison without making them merge-blocking thresholds. The project SHALL document that a zero-allocation gate alone does not protect against instruction-count, cache-miss, throughput, or tail-latency regressions.

#### Scenario: Interpret a passing allocation gate
- **WHEN** a data-plane path passes its zero-allocation assertion
- **THEN** the benchmark documentation states that further throughput and latency review can still be required
