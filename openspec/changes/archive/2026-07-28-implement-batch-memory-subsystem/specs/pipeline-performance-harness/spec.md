## MODIFIED Requirements

### Requirement: Stable data-plane benchmark matrix
The repository SHALL provide a Go benchmark harness with named cases for node invocation, pool acquire/release, local-edge transfer, remote-queue transfer, and end-to-end DAG processing. The harness SHALL encode batch size and applicable topology dimensions in benchmark case names. The `pool-acquire-release` case SHALL execute acquire, fixed-capacity fill, logical-length update, and release against a constructed pool; cases without an executable owner SHALL remain explicitly unavailable.

#### Scenario: Discover benchmark coverage
- **WHEN** a developer lists the framework benchmarks
- **THEN** they can identify a benchmark entry point for each planned data-plane path and its batch or topology configuration, including an executable pool acquire/release case

### Requirement: Allocation measurement contract
The harness SHALL report allocations and SHALL expose `testing.AllocsPerRun` assertions for approved steady-state paths. An implementation stage SHALL enable a zero-allocation assertion only after the corresponding path is executable and has completed setup before measurement begins. The batch subsystem SHALL assert zero allocations for repeated pool acquire/fill/release work and for the pool-exhaustion path.

#### Scenario: Measure a steady-state path
- **WHEN** an implemented data-plane benchmark runs after its setup phase
- **THEN** its allocation assertion measures only repeated steady-state work and fails if allocations exceed zero

#### Scenario: Measure pool lifecycle allocations
- **WHEN** the pool benchmark and allocation tests run against a constructed typed pool
- **THEN** acquire, fixed-capacity fill, logical-length update, release, and exhaustion reporting each perform zero allocations per operation
