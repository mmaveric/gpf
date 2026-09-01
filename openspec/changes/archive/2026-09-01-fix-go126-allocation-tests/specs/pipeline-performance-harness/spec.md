## MODIFIED Requirements

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
