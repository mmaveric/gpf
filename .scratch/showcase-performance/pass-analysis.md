# Analysis of executorWorker.pass

Date: 2026-10-26. Scope: current scheduler pass, its direct control/lifecycle context, existing full TSC-handler showcase profile, and synthetic pass experiments. No production code/configuration changed.

## Main finding

The current value-range loop copies the **48-byte runner** onto the stack on every runner visit. This is confirmed by the actual profiled executable's assembly, not inferred solely from Go source. An experimental index/pointer loop retaining source, timing and metrics checks reduces isolated pass time by approximately **10–17%** across the measured cases. It is the best first production experiment, but has not been applied and has no full-application speedup evidence yet.

## What pass does

`graph/executor.go:707–728`:

1. Optionally start worker timing (disabled in the current showcase; handler timing is separate).
2. Visit every runner in order.
3. For source runners, skip them during Close; after source admission closes, only run sources with pending work.
4. Invoke `runner.run()` through a function value and accumulate whether any runner reports progress.
5. Update local group pass/idle/timing counters and periodically publish them.

`executorWorker.loop` calls pass after a nonblocking commands-channel check. It does not park on each idle pass: only after 2^20 consecutive no-progress passes does it call `runtime.Gosched`. Thus tiny per-pass costs repeat frequently, including on empty queues. The current evidence does not measure per-group idle ratios, so it cannot quantify how much of the aggregate profile is useful vs idle scanning.

The showcase placements put three nodes in the ingress group and one in each of the six other groups. This motivated one- and three-runner benchmark cases, rather than testing only a large artificial scan.

## Profile decomposition

Source: `/tmp/opencode/showcase-tsc-full-cpu.prof`, total sampled CPU 20.85 s, matching `.test` executable.

| Attribution | Sampled CPU | Interpretation |
|---|---:|---|
| pass self time | 2.74 s / 13.14% | Includes scan, checks, dispatch setup and call overhead |
| pass cumulative | 8.91 s / 42.73% | Includes runners/handlers; not pure scheduler overhead |
| runner loop line 714 | 0.91 s flat | Copying plus loop bookkeeping/instruction attribution |
| source/closing condition line 715 | 0.53 s flat | Includes checking the source flag, not necessarily atomic loads |
| source/admission condition line 718 | 0.09 s flat | Only source runners reach admission/pending checks |
| runner.run line 721 | 0.22 s flat / 5.77 s cumulative | Indirect dispatch plus substantial nested work |
| timing-enabled condition line 710 | 0.32 s flat | Disabled fast path still has branch/control overhead |
| recordPassMetrics | 0.55 s flat / 0.57 s cumulative | Separate local accounting cost |

Line samples are approximate; do not assign all 0.91 s to copying or all 0.53 s to atomic operations. In particular, Go short-circuit evaluation means non-source runners do NOT load closing/sources atomics. These percentages cannot be mapped directly to expected wall-time speedup.

Generated `normalizeExplicit1` runner source checks input readiness and returns false before invoking handler timing if no input is ready (`graph/registration_gen.go:350–355`). Therefore full handler timing does not imply clock reads on every empty runner poll. Runner cumulative costs mix readiness polling, batch ownership/finalization, metrics and application handlers.

## Assembly evidence

`go tool objdump -s 'graph\.\(\*executorWorker\)\.pass$' /tmp/opencode/showcase-tsc-full.test` shows:

```text
ADDQ $0x30, SI             // next runner, 48-byte stride
LEAQ 0x40(SP), R10
MOVUPS 0(SI), X14
MOVUPS X14, 0(R10)
MOVUPS 0x10(SI), X14
MOVUPS X14, 0x10(R10)
MOVUPS 0x20(SI), X14
MOVUPS X14, 0x20(R10)
```

All three 16-byte chunks are copied even though a normal non-source iteration mostly needs `source` and `run`. The struct also carries lifecycle/discard/pending function values not needed on every visit. The function has a 0x78-byte stack frame and an indirect call through the copied run function. Index-based access avoids requiring that full value copy; the experiment measures the resulting combined code-generation effect, not just isolated copy instruction latency.

## Synthetic benchmark experiment

Benchmark source is `pass-analysis-benchmark.go.txt` beside this report, installed as a test-only file through `/tmp/opencode/pass-analysis-overlay.json`.

- Go 1.26.8, linux/amd64, Intel Core Ultra 7 255U, GOMAXPROCS suffix 14.
- Six samples per case, `-benchtime=200ms`, serial execution.
- Startup-initialized flush interval 2048; worker timing disabled; source admission enabled, closing false.
- Runners return constant false (idle) or true (reported progress). No real queues or handlers.
- Every variant is invoked through the same benchmark function-value interface.
- Baseline calls production pass unchanged.
- Indexed variant uses `r := &w.runners[i]` and retains all existing conditions and metrics calls.
- Source-free variant uses indexed dispatch without source/admission checks, and is measured ONLY on fixtures containing no sources. It is not safe as a universal replacement.

Medians, ns/pass:

| Fixture | Baseline | Indexed | Indexed delta | Indexed source-free |
|---|---:|---:|---:|---:|
| 1 runner, idle, no source | 5.174 | 4.643 | −10.26% | 4.436 |
| 1 runner, progress, no source | 5.109 | 4.510 | −11.73% | 4.223 |
| 3 runners, idle, no source | 9.350 | 7.795 | −16.63% | 6.851 |
| 3 runners, progress, no source | 9.377 | 7.934 | −15.39% | 6.756 |
| 3 runners, first is source, idle | 9.638 | 8.462 | −12.21% | not applicable |
| 3 runners, first is source, progress | 9.817 | 8.817 | −10.18% | not applicable |

Indexed vs baseline benchstat p-values range from 0.002 to 0.026 (six samples); all cases report zero allocations. Source-free vs baseline improves 14–28% for its eligible fixtures, combining copy elimination AND removal of source checks. Do not interpret that entire difference as the effect of source-check removal. The empty closures make dispatch a disproportionately large part of measured work; queue/cache behavior and concurrent lifecycle transitions are absent. The six comparisons are exploratory, without multiple-testing correction, and measurements are from one session.

## Semantics to protect

- `stop` closes source admission before pausing workers (`executor.go:494–498`). Close also sets closing; Drain must still allow pending source output to finalize. Dropping these checks globally is incorrect.
- A source's pending callback must remain conditional; checking it unconditionally may change behavior/cost.
- Preserve runner order and invoke all eligible runners even after one reports progress. Do not introduce short-circuit OR that skips later `run()` calls.
- Preserve pass counters, idle classification, flush boundaries, sampled timing phase and Pause/Stop final flush.
- Indexed access assumes the runner slice and relevant runner descriptors are stable during a pass. Startup constructs workers/runners before launching execution, but production changes must preserve the existing lifecycle contract and not introduce mutation races. Value snapshots vs pointers should be reviewed if future APIs permit in-pass descriptor changes.
- Avoid splitting source/non-source runners into separate orderings: that can change the demonstrated scheduling order and progress behavior.

## Recommendation

1. **First experiment:** replace the value-range copy with index-based field access, keeping checks, order and accounting unchanged. Validate with lifecycle/source pending tests, full/race suites and paired prebuilt launches on the same current TimingFull/TSC configuration. Keep only claims supported by those measurements.
2. **Second, only if justified:** a startup-selected source-free pass for groups proven to contain no source runners. It needs additional control flow/code paths; its incremental gain over indexed alone is modest (roughly 0.2–1.2 ns in these fixtures). Prefer not to add this complexity before testing step 1 end to end.
3. **Do not prioritize:** increasing metrics flush interval, removing timing functionality, or reducing command polling frequency based only on this profile. They have different correctness/latency tradeoffs; the copied runner is a simpler first target.

No full application was rerun for these synthetic variants, no lifecycle equivalence of the helpers is claimed, and no production optimization has been installed by this analysis task.

## Reproduction and artifacts

```sh
go test -overlay=/tmp/opencode/pass-analysis-overlay.json -run '^$' \
  -bench '^BenchmarkPassAnalysis$' -benchmem -count=6 -benchtime=200ms ./graph
benchstat /tmp/opencode/pass-analysis-baseline.txt \
  /tmp/opencode/pass-analysis-indexed.txt /tmp/opencode/pass-analysis-source-free.txt
```

- Raw samples: `/tmp/opencode/pass-analysis-bench.txt`.
- Normalized variant files: `/tmp/opencode/pass-analysis-{baseline,indexed,source-free}.txt`.
- Assembly: `/tmp/opencode/pass-analysis-assembly.txt`.
- Profile listing: `/tmp/opencode/showcase-tsc-full-workers-list.txt`.

Graph symbol search returned no current pass candidate; coverage flags executor/generated registration as changed and topology as untracked in the stale 2026-09-02 index. Current source reads, fresh benchmark compilation and the matching profile executable are the evidence for this analysis. This is bounded source/measurement verification, not an exhaustive call-graph audit.

## Follow-up: indexed loop implemented and measured

The production loop in `graph/executor.go` now uses index-based access and a pointer to the current runner. The source/closing/admission conditions, runner order, every eligible `run()` call, progress accumulation and metrics call are otherwise unchanged. This avoids copying the 48-byte runner value. No source-free scheduler specialization was added.

Validation on the current worktree/configuration:

- `go test ./...` passed.
- `go test -race ./graph ./examples/showcase/...` passed.
- `git diff --check` passed.
- Before executable: `/tmp/opencode/showcase-tsc-full-rerun` (pre-loop-change; same current TimingFull/TSC config).
- After executable: `/tmp/opencode/showcase-pass-indexed-after`.

Paired prebuilt launch comparison, 24 alternating pairs:

| Measurement | Before | After |
|---|---:|---:|
| Median wall time | 0.688264 s | 0.685494 s |
| Estimated change | — | **−0.40%** |
| Peak RSS median | 200,936 KiB | 201,020 KiB |
| Wins | 11 | 13 |

Exact two-sided sign test `p=0.839`: **no statistically significant full-launch speedup**. Max RSS was 242,896 KiB before vs 240,872 KiB after. These shifts are noise-level and not treated as a benefit.

All 24 runs had identical deterministic result, topology summary, no-drop/error/pool-in-use values, and affinity statuses. Entire stdout is not byte-identical because the showcase prints nondeterministic TSC timing measurements: measured call count was stable but aggregate/max duration varied. Those timing numbers are instrumentation observations, not correctness totals. This explains the initial byte-level comparison failure; semantic lines were compared after excluding only `handler_timing_clock=...`.

One after-change 10-iteration full profile measured 836.778 ms/op, 381,455,040 B/op, 1,010 allocs/op. Earlier same-config one-capture profile was 814.929 ms/op, 381,455,884 B/op, 1,010 allocs/op. These isolated profile runs disagree in wall/op and are too few/noisy for a performance claim. In their CPU profiles, `executorWorker.pass` flat CPU was 13.14% before and 11.95% after; cumulative was 42.73% before vs 45.08% after. These shares vary with sampled workload; do not interpret them as proof of faster execution.

Fresh disassembly/profile line listing confirms indexed access: the loop now samples `runner := &w.runners[i]` and does not contain the former three 16-byte struct-copy stores. That validates the intended local codegen change, not a full-app speedup.

Artifacts: `/tmp/opencode/showcase-pass-paired.json`, `/tmp/opencode/showcase-pass-{before,after}-paired-*`, and `/tmp/opencode/showcase-pass-indexed-after-{cpu,mem}.prof`.
