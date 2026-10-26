# Should gpf replace Go's `time` package for speed?

**Conclusion:** Do not replace `time` across gpf on the evidence available. Production code uses `time` narrowly: `runtime.Config.DrainTimeout` is a `time.Duration`, and `graph.monotonicClock` captures `time.Now()` then reads `time.Since(epoch).Nanoseconds()` for an opt-in processing-timing clock. No production timers, tickers, sleeps, parsing, formatting, or deadlines were found outside examples/showcase. The current hot-path clock is a candidate for a targeted benchmark, not a package-wide migration. A custom cycle-counter clock might reduce instruction cost on particular hardware, but changes units/calibration/portability/monotonic semantics and must be measured against this workload.

## Scope and repository evidence

Excluded `examples/showcase/**`, `examples/standalone-showcase/**`, and tests as requested. Search of production `*.go` sources found:

* `runtime/runtime.go`: `DrainTimeout time.Duration` (API/configuration type only).
* `graph/timing.go`: `monotonicClock()` uses `time.Now()` once at configuration and `time.Since(epoch).Nanoseconds()` on each measured call; timing is disabled unless configured. The injected `TimingConfig.Clock` contract requires monotonic modular nanoseconds and no blocking/allocation.
* No production uses of `time.After`, timers, tickers, `Sleep`, wall-clock formatting/parsing, or context deadlines were found.

The working tree already contains unrelated modifications and untracked files; this note is the only change made for this research. The code graph snapshot was ready but dated 2026-09-02; source search was used to verify current working-tree usage, so graph results are not treated as current evidence.

## Semantics that a replacement must preserve

Go's `time.Time` returned by `time.Now` contains both wall-clock and monotonic readings when available. `Sub`, `Since`, and `Until` use monotonic readings when both operands have them; serialization and operations such as `Round(0)` strip monotonic data. Wall time can jump due to clock adjustment, so elapsed measurement should not be built from Unix timestamps. `time.Duration` is an `int64` nanosecond count, not a clock reading.

For broader future use, timers and tickers have channel delivery semantics, cancellation/resource lifecycle, and runtime integration; `Sleep` blocks the goroutine; deadlines usually carry wall-clock instants with monotonic components when locally constructed; parsing/formatting implement Go's reference-layout behavior and timezone rules. A replacement lacking these operations is not a drop-in substitute. In this repository, only duration representation and monotonic elapsed timing are currently material.

## Candidate comparison

| Candidate | Potential speed case | Compatibility / semantic cost | Assessment for gpf |
|---|---|---|---|
| Keep `time` | Standard library implementation, compiler/runtime integration, monotonic elapsed calculations. | None. | Best default; optimize only if timing is measured as a material bottleneck. |
| `github.com/loov/hrtime` (cycle-counter style high-resolution timing) | May offer lower overhead for repeated elapsed-time reads on supported platforms. | Narrower purpose than `time`; counter calibration, units, CPU/platform support, migration/serialization and monotonic guarantees differ. It does not replace durations, timers, parsing, deadlines, etc. | Only a possible isolated implementation of `TimingConfig.Clock`; validate its contract and portability first. No evidence supports a whole-package replacement. |
| `github.com/benbjohnson/clock` / `github.com/jonboulle/clockwork` | Injectable/mock clocks improve deterministic tests, not an inherent faster runtime clock. | Adds abstraction/dependency and may use standard time underneath; fake time semantics differ from real scheduling. | Not a maximum-speed optimization. Consider only if deterministic time-driven tests are needed. |
| Cached/coarse wall clock (e.g. framework-specific cached `Now`) | Avoids repeated syscalls or clock reads when staleness is acceptable. | Returns stale/coarse wall time, not a suitable substitute for precise monotonic elapsed durations; introduces updater lifecycle and concurrency. | Not appropriate for `monotonicClock` without explicitly accepting accuracy/semantic loss. |

This list distinguishes narrow clock sources from replacements for the complete standard package. There is no meaningful universal “fastest time package”: operation, Go version, OS, architecture, timer resolution, contention and precision requirements control results. Do not infer end-to-end pipeline speedup from a microbenchmark of one clock call.

## Validation plan before any production change

1. Benchmark the exact closure used by `monotonicClock`, comparing `time.Since(epoch).Nanoseconds()` with the candidate's read and conversion. Use `testing.B`, `b.ReportAllocs`, repeated runs and `benchstat`; benchmark on supported OS/architectures and Go versions. Include parallel runs if called concurrently.
2. Measure realistic handler throughput/latency with timing enabled (full and sampled) and disabled; report percentiles or distributions as appropriate. The feature is opt-in, so the practical benefit must be evaluated in that mode.
3. Check allocations, monotonicity, wraparound behavior, units, resolution, cross-core consistency and documented platform support. Exercise concurrent callers and race testing where applicable.
4. Keep the `time.Duration` public configuration API and timer/deadline behavior on `time`; change only the clock behind `TimingConfig.Clock` if the measured end-to-end benefit justifies semantic and maintenance costs. Document loss of standard monotonic guarantees if any.

## Primary sources

* Go `time` package documentation, including monotonic clocks, `Time`, `Duration`, timers and tickers: <https://pkg.go.dev/time>
* Go source: `Time` monotonic representation and `Now`/`Since` implementation: <https://go.dev/src/time/time.go>
* Go runtime timer implementation and semantics: <https://go.dev/src/time/sleep.go>
* Go benchmark tooling: <https://pkg.go.dev/testing#B> and `benchstat` project documentation/source: <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>
* `loov/hrtime` project source and documented API/platform behavior: <https://github.com/loov/hrtime>
* `benbjohnson/clock` source/docs (mock clock): <https://github.com/benbjohnson/clock>
* `jonboulle/clockwork` source/docs (fake clock): <https://github.com/jonboulle/clockwork>

Sources establish APIs and semantics, not a performance result for gpf. No candidate benchmark directly comparable to gpf's `time.Since` closure was found; use the controlled validation above rather than importing vendor claims as a conclusion.

## Local comparison for `graph/timing.go` (2026-10-26)

Ran a temporary benchmark harness outside the repository so no runtime dependency or `go.mod` change was needed. Environment: Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U. Each benchmark called a clock closure returning `uint64` nanoseconds, matching the shape of `TimingConfig.Clock`; six 1-second samples were summarized with `benchstat`.

| Clock read | ns/op (benchstat) | allocs/op | Semantics / result |
|---|---:|---:|---|
| `uint64(time.Since(epoch).Nanoseconds())` | 13.08 ± 1% | 0 | Current production implementation; monotonic elapsed time. |
| `uint64(hrtime.Since(epoch).Nanoseconds())` | 23.76 ± 1% | 0 | Slower here. On non-Windows, hrtime v1.0.4 implements `Now()` from `time.Now().UnixNano()`, so it loses the monotonic-clock property as well. Not recommended. |
| `uint64(hrtime.TSCSince(epoch).ApproxDuration().Nanoseconds())` | 6.478 ± 1% | 0 | About 2.02× faster per read in this microbenchmark; TSC reported supported on this machine (reported read overhead: 26 counts). Returns an *approximate* calibrated duration, not the same guarantee as Go's monotonic clock. |

The real `BenchmarkTimingModes` in `graph/timing_test.go` was also run before comparing alternatives: `off` 11.12 ns/op, `full` 46.00 ns/op, `sampled-64` 14.10 ns/op (median-like representative of five 2-second runs; all 0 allocs/op). In full mode the clock is read twice per measured handler, so the isolated TSC delta suggests a possible reduction of roughly 13 ns per handler, not a demonstrated end-to-end improvement. The handler benchmark was not rerun with TSC wired in, because doing so would mean changing production behavior before validating TSC's semantics.

### Implementation and recommendation

The configuration now exposes `TimingClockStandard` (zero/default) and `TimingClockTSC` via `TimingConfig.ClockSource`; worker timing has the same option on `WorkerTimingConfig`. A custom `Clock` still takes precedence. Unsupported TSC systems reject the setting with `ErrUnsupportedTimingClock`; TSC calibration is warmed before the clock closure is returned.

The real graph benchmark with the selected source shows `full/time` at about 46.0 ns/op versus `full/tsc` at about 32.8 ns/op (roughly 29% lower total benchmark time). `sampled-64` is about 14.5 ns/op with `time` and 14.3 ns/op with TSC, so sampling already amortizes the clock cost. All variants remain allocation-free. These are local microbenchmark results, not a promise of production workload improvement.

Use the standard source unless full timing is enabled and the measured improvement is worth the TSC's approximate conversion and hardware-specific behavior. The benchmark includes monotonicity and execution tests on supported TSC hardware; broader supported-platform and cross-core validation is still needed before relying on TSC in portable deployments. Do not use `hrtime.Since` as an optimization on Linux. The initial temporary harness and raw isolated-clock output are in `/tmp/opencode/timebench/` and are not repository changes.

Benchmark output was generated with `benchstat /tmp/opencode/timebench/results.txt`; the source of the hrtime implementation details is its v1.0.4 Go module (`now_other.go`, `tsc.go`, and `stopwatchtsc.go`): <https://github.com/loov/hrtime/tree/v1.0.4>.
