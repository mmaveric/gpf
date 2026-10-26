# Test the next larger-batch capacity variant

Type: task
Mode: AFK
Status: resolved
Blocked by: 15

## Question

Does doubling Variant B's batch sizes again reduce full-process wall time while preserving losslessness and approximately constant configured item buffering?

### Work

- Keep topology, CPU placement, input data, output, and core implementation unchanged.
- Compare Variant B (`1024/16` ingress, broadcast `1024`, shard `49152/8`, fan-in `16384`) with Variant C (`512/32`, broadcast `512`, shard `24576/16`, fan-in `8192`).
- Validate exact output, zero queue/pool drops, zero logged errors, zero pools in use, all tests, and full-process timings on identical data.
- Retain C only if paired prebuilt-launch times improve significantly; report Go benchmark and RSS trade-offs.

## Answer

- Variant C was initially retained in the uncommitted showcase source, then superseded by the statistically faster Variant D in ticket 17.
- The configured capacity×batch products are unchanged from Variant B (ingress 16,384; each broadcast branch 16,384; each shard branch 393,216; each fan-in branch 131,072). Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Across 24 paired serial full-binary launches, C was faster in all 24 pairs: median `0.6961 s` vs B `0.7542 s` (**7.71% faster**, exact two-sided sign-test `p=1.19e-7`). Every run matched exact totals: 1,000,000 processed, 5,000,000 dequeued, zero queue/pool drops, zero logged errors, and zero pools in use; B/C stdout matched byte-for-byte.
- `benchstat` with ten `-benchtime=1x` samples did not show significant sec/op changes: `BenchmarkFullShowcase` 798.0 ms vs 804.2 ms (`p=0.853`); `BenchmarkRun` 265.1 ms vs 235.6 ms (`p=0.123`). B/op fell by 1.80% and 3.62% respectively; allocs/op changes were not significant.
- Paired median RSS was 212,972 KiB for B and 215,632 KiB for C (+1.25%). Max sample RSS was 215,148 KiB vs 244,316 KiB (+13.6%); investigate the C outlier if memory ceiling is important. No material median RSS change observed.
- `go test ./...` and `go test -race ./graph ./examples/showcase/internal/showcase` passed in the isolated candidate worktree. User showcase source remains uncommitted.
- Raw samples: `/tmp/opencode/showcase-capacity-{B,C}-paired-*.{out,time}` and `/tmp/opencode/showcase-capacity-{B,C}-bench*.txt`.
