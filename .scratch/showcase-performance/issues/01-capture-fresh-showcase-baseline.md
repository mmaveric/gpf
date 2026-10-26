# Capture a fresh showcase baseline

Type: task
Mode: AFK
Status: resolved

## Question

What reproducible performance baseline and current-code profile should guide the first optimization decision?

### Work

- Add a permanent `BenchmarkRun` for one million deterministic valid orders. Prepare the input outside the timed loop, report allocations, and send output to `io.Discard`. Do not add a CI performance threshold.
- Run the benchmark repeatedly and retain the raw results plus a CPU/allocations profile for the current implementation.
- Build `examples/showcase` once, then time at least ten sequential launches against the unchanged `examples/data.json`. Record each wall-time sample and peak RSS; compilation must not be timed.
- Record the Go version, OS/architecture, CPU, affinity outcomes, and source revision/worktree state with the results.
- Check the full-run output against expected million-order totals. Record any discrepancy as a blocker to using the timing data for optimization. Keep the existing separate error-route correctness test in the validation set.
- Do not optimize production or example hot paths while establishing this baseline.

### Resolution evidence

Attach the raw benchmark output, profile summaries, and full-run timing samples to this ticket. Summarize the measured cost centers without selecting an optimization; that decision belongs in the blocked child ticket.

## Answer

- Added `BenchmarkRun` in `examples/showcase/internal/showcase/run_benchmark_test.go`, committed as `0add447` (`test(showcase): add million-order benchmark`) on `integration/showcase-performance`.
- `go test ./examples/showcase/...` passed. Five benchmark samples ranged from 0.521 to 1.195 s/op, with 143.8 MB/op and 971–1,202 allocs/op.
- Ten prebuilt-binary launches took 1.03–1.26 s and used 170,316–177,192 KB peak RSS. The process reported affinity applied for groups 1–7 on CPUs 0–6.
- Two launches lost orders: one processed 999,975 and reported 25 `pool_drop_elements`; another processed 999,986 and reported 14 `pool_drop_elements`. The other eight matched the expected totals. This appears in `run-output.txt` and `run-times.txt` in `/tmp/opencode/showcase-baseline-artifacts/`.
- CPU samples were concentrated in `graph.(*groupCounters).recordPass` (27.45% flat) and `graph.(*executorWorker).pass` (17.42% cumulative in the profile table); normalization and atomic operations were also visible. The profiles are directional only until the showcase completes the same workload without drops.
- The implementation follows up in [Stabilize the million-order showcase run](03-stabilize-million-order-showcase-run.md). Do not use these lossy runs as the optimization comparison baseline.
