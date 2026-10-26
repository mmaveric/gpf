# Profile the full showcase workflow

Type: task
Mode: AFK
Status: resolved
Blocked by: 04

## Question

Which costs appear when the current million-order file is read and decoded as well as processed by the showcase?

### Work

- Add a diagnostic `BenchmarkFullShowcase` that performs `loadOrders` and `showcase.Run(io.Discard, orders)` in each measured iteration, so the existing JSON file read/decode path is included. Keep `BenchmarkRun` as the pipeline-only comparison.
- Prepare no part of the file outside the timed iteration; report allocations. Do not add a CI performance gate or alter production behavior.
- Capture repeated benchmark samples and CPU/alloc profiles for this full workflow, plus a fresh ten-launch end-to-end timing/RSS set against the unchanged dataset.
- Record Go version, OS/architecture, CPU, affinity outcomes, dataset hash, raw output, and profile summaries. Confirm all launch totals and drop counters remain exact.

### Resolution evidence

Attach raw benchmark/profile/timing artifacts outside the repository and summarize the main costs. Do not select or implement the next optimization in this task.

## Answer

- Added `BenchmarkFullShowcase` in `examples/showcase/main_bench_test.go`; it includes `loadOrders()` and `showcase.Run(io.Discard, orders)` per iteration. Commit: `3ba60f8` (`test(showcase): benchmark full workflow`).
- `go test ./examples/showcase/...` passed. Ten benchmark samples had a 1.223 s/op median (1.110–1.698 s/op), 434,191,036 B/op, and 1,051.5 allocs/op.
- Full-workflow CPU profile: `graph.(*groupCounters).recordPass` 21.74% flat; `graph.(*executorWorker).pass` 62.06% cumulative. Allocation profile: `reflect.growslice` 31.07% flat, `showcase.Run` 30.98%, `batch.NewPool` 14.33%, and `os.readFileContents` 13.48%.
- Ten prebuilt launches against the unchanged dataset all produced exact million-order totals and zero drops/errors. Median wall time was 1.105 s, median peak RSS 254,888 KB. Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Raw benchmark, CPU/alloc profiles, affinity, environment, and full-run samples: `/tmp/opencode/showcase-profile-full/`.
