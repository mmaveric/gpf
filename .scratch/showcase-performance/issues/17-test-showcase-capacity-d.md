# Test Variant D with doubled batches

Type: task
Mode: AFK
Status: resolved
Blocked by: 16

## Question

Does another doubling of batch size improve the primary full-process launch metric relative to Variant C without data loss?

### Work

- Preserve topology, CPU placement, data, outputs, and core implementation.
- Compare C (`512/32` ingress, broadcast `512`, shard `24576/16`, fan-in `8192`) to D (`256/64`, broadcast `256`, shard `12288/32`, fan-in `4096`).
- Use identical dataset, exact output checks, paired serial full-process runs, repeated Go benchmarks, full tests, and race checks.
- Retain D only if launch time is statistically significantly lower; report benchmark and memory behavior.

## Answer

- Retained D in the uncommitted showcase topology: ingress capacity/batch `256/64`, broadcast capacity `256`, shard capacity/batch `12288/32`, fan-in capacity `4096`; logger remains `4`.
- In 24 paired serial full-process launches, D was faster in 21 pairs and slower in 3. Median was `0.6717 s` vs C `0.6918 s` (**2.91% faster**, exact two-sided sign-test `p=0.000277`).
- Output was byte-identical across variants and all runs verified 1,000,000 processed, 5,000,000 dequeued, zero queue/pool drops, zero logged errors, and zero pools in use.
- Ten `-benchtime=1x` samples did not show significant sec/op change (`BenchmarkFullShowcase` 791.2 ms vs 783.6 ms, `p=0.853`; `BenchmarkRun` 166.5 ms vs 151.3 ms, `p=0.315`). B/op fell 0.91% and 1.88%; allocation-count changes were not significant.
- Median RSS was 215,616 KiB for C and 224,466 KiB for D (+4.1%); maximum samples were 244,260 KiB and 243,360 KiB respectively.
- `go test ./...` and `go test -race ./graph ./examples/showcase/internal/showcase` passed. Data SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Artifacts: `/tmp/opencode/showcase-capacity-{C,D}-paired-*.{out,time}` and `/tmp/opencode/showcase-capacity-{C-vs-D-baseline,D}-bench.txt`. Showcase source remains uncommitted per user instruction.
