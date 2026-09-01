## Context

See `proposal.md` for motivation. The allocation harness delegates to `testing.AllocsPerRun`, which temporarily sets `GOMAXPROCS(1)`. Go 1.26 now detects any active parallel test in the package test process and panics before measurement. Two batch allocation tests and the shared harness self-test currently mark themselves parallel; the graph allocation test is already serial.

The module continues to support Go 1.22 and newer. Allocation run counts, setup boundaries, measured operations, and zero-allocation thresholds are existing performance contracts and must not change.

## Goals / Non-Goals

**Goals:**

- Make all current `testing.AllocsPerRun` assertions valid under Go 1.26 test scheduling.
- Preserve parallel execution for tests that do not perform allocation measurement.
- Preserve allocation coverage and measurement semantics across supported Go versions.

**Non-Goals:**

- Changing production batch, graph, queue, node, or runtime behavior.
- Replacing `testing.AllocsPerRun` or changing allocation thresholds.
- Serializing entire packages or disabling Go's package-level test parallelism.
- Introducing a Go-version-specific code path.

## Decisions

### Keep allocation assertions in serial top-level tests

Remove `t.Parallel()` only from tests that call `benchtest.AssertZeroAllocs`. Go's test runner completes serial top-level tests while parallel tests are paused, satisfying the Go 1.26 guard without changing unrelated tests.

This is preferred over removing parallel execution package-wide because the incompatibility is local to allocation measurement. It is also preferred over a mutex inside `AssertZeroAllocs`: a mutex could serialize competing allocation assertions but cannot make Go's independently active parallel-test counter reach zero.

### Preserve the shared helper contract

Leave `AssertZeroAllocs`, its warm-up behavior, caller-supplied run count, and failure reporting unchanged. Test scheduling is a caller responsibility because only the `testing` package controls whether a test is marked parallel.

This is preferred over spawning a subprocess or adding runtime-version detection, both of which would add complexity and could distort allocation measurement without improving coverage.

### Verify all allocation-harness call sites

Audit every `AssertZeroAllocs` call to confirm its containing top-level test is serial. The existing graph allocation assertion requires no change if that audit confirms it remains outside parallel execution.

## Risks / Trade-offs

- [The affected allocation tests no longer run concurrently with sibling tests] -> Limit serialization to the three current allocation-test entry points; their measurement already requires isolated process scheduling.
- [A future caller may add `t.Parallel()` and reintroduce the panic] -> Document the serial requirement in the performance-harness spec and keep full-suite Go 1.26 verification as an implementation task.
- [Removing parallel scheduling could expose order dependence] -> Run the complete unit and race suites rather than only the affected packages.

## Migration Plan

1. Remove parallel markers from the affected allocation tests.
2. Audit all allocation-harness callers for serial scheduling.
3. Run unit, race, vet, and strict OpenSpec validation gates.

Rollback consists of restoring the test-only scheduling markers; no runtime data, public API, or generated artifact migration is involved.
