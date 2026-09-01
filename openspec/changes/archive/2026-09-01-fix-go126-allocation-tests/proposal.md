## Why

Go 1.26 panics when `testing.AllocsPerRun` executes while a parallel test is active, causing the repository's allocation assertions to abort `go test ./...` instead of measuring steady-state allocations. The test harness needs to remain valid across the supported Go 1.22-or-newer range without weakening its zero-allocation gates.

## What Changes

- Run every `testing.AllocsPerRun`-based assertion from serial test execution rather than from tests marked with `t.Parallel`.
- Preserve the existing warm-up, run counts, measured operations, and zero-allocation expectations.
- Verify the complete unit suite on Go 1.26 and retain compatibility with the module's Go 1.22 minimum.
- Leave application runtime behavior, public APIs, and benchmark topology unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `pipeline-performance-harness`: Require allocation assertions to execute outside parallel-test activity so `testing.AllocsPerRun` remains valid on Go 1.26 and newer.

## Impact

- Affected tests: batch pool allocation tests and the shared allocation assertion self-test.
- Affected harness: `internal/benchtest.AssertZeroAllocs` callers and their scheduling constraints.
- No production code, public API, dependency, benchmark name, or allocation threshold changes.
