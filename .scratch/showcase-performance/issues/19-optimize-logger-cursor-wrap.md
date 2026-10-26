# Optimize logger cursor wrap

Type: task
Mode: AFK
Status: resolved
Blocked by: 18

## Goal

Reduce the profiled cost in `runtimeLogger.pass` without changing logger event order or lifecycle behavior.

## Resolution

- Replaced variable modulo cursor update with increment and conditional wrap in `graph/logger.go`.
- Existing round-robin public-seam test `TestLoggerPollsRingsRoundRobin` passed; full unit and race suites passed.

## Evidence

- Seven-empty-ring `BenchmarkRuntimeLoggerPass`, 10 samples: `24.810 ns/op` before, `7.262 ns/op` after (−70.73%, benchstat `p=0.000`, 0 allocations both).
- 24 paired prebuilt full launches: before median 0.6671 s, after 0.6722 s (+0.75% estimate); the after build won 15/24 pairs, exact sign-test `p=0.307`. No statistically significant end-to-end change.
- All output bytes matched; exact totals and zero drops/errors/pools-in-use were verified.
- `go test ./...`, `go test -race ./graph ./examples/showcase/...`, and `git diff --check` passed.
- Raw measurements and detailed interpretation are in `../profile-report-variant-d.md` and `/tmp/opencode/logger-pass-*`, `/tmp/opencode/showcase-logger-*-paired-*`.
