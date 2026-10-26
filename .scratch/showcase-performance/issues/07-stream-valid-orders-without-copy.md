# Stream valid orders without a redundant copy

Type: task
Mode: AFK
Status: resolved
Blocked by: 06

## Question

Can the all-valid showcase workload submit bounded slices of the original input without allocating and copying a second million-order slice, while preserving mixed-input behavior?

### Work

- Add a public-seam regression for the all-valid fast path and retain coverage for mixed valid/invalid orders, typed error routing, and exact report totals.
- Change only `showcase.Run` input buffering. Preserve the existing topology, invalid-order behavior, and output.
- Compare `BenchmarkRun`, `BenchmarkFullShowcase`, and ten sequential full binary launches against the current lossless baseline. Record benchstat, allocations, peak RSS, output totals, and dataset hash.
- Keep the change only if the full-run median improves statistically significantly. Otherwise revert the candidate and record the evidence.

### Resolution evidence

Attach before/after raw benchmark and launch samples, correctness tests, and the final diff or no-change result.

## Answer

- Tested an all-valid fast path that reused the caller's input slice and preserved the existing mixed-validity path. Existing exact-output all-valid and mixed/error-route tests passed.
- The candidate reduced `BenchmarkRun` bytes/op by 58.19% and `BenchmarkFullShowcase` bytes/op by 31.72%; median peak RSS fell from 253,830 KB to 177,358 KB.
- It did not meet the speed acceptance criterion: `BenchmarkRun` time was 608.8 ms/op before and 591.6 ms/op after (`p=0.247`); `BenchmarkFullShowcase` was 1.192 s/op vs 1.195 s/op (`p=0.481`); full-binary median was 1.105 s vs 1.070 s (`p=0.099`). The candidate was reverted; no code or test commit was retained.
- Ten before/after launches produced byte-identical output with exact totals and zero drops/errors. Tests passed with the current lossless capacities.
- Raw samples and run outputs: `/tmp/opencode/showcase-stream-valid-orders/`. Experiment branch/worktree: `ticket/stream-valid-orders-without-copy` / `/tmp/opencode/gpf-stream-valid-orders-wt`.
