# Tune showcase batch capacities

Type: task
Mode: AFK
Status: resolved
Blocked by: 14

## Question

Can larger batches reduce scheduler and queue overhead enough to produce a statistically significant full-process speedup while keeping the million-order run lossless?

### Work

- Keep node/edge topology, data, output semantics, and live metrics contracts unchanged. Tune only showcase batch/queue capacities, which are in scope.
- Compare a small, documented matrix of ingress/broadcast/shard/fan-in batch sizes and bounded queue capacities. Track the resulting total buffered elements so capacity changes do not obscure the memory trade-off.
- For every candidate, verify exact totals, zero queue/pool drops, and zero logged errors. Run benchmark and full prebuilt-launch samples serially against the same lossless baseline.
- Keep a configuration only if end-to-end median wall time improves statistically significantly; report allocations and peak RSS as secondary metrics and discuss material increases.
- Do not retain wakeups, worker-scheduler changes, or other core implementation experiments in this ticket.

### Resolution evidence

Attach the capacity matrix, raw benchmark/launch samples, statistical comparison, output checks, and selected configuration or no-change result.

## Answer

- Retained Variant B: ingress capacity/batch `1024/16`, broadcast capacity `1024`, shard capacity/batch `49152/8`, fan-in capacity `16384`; logger capacity remains `4`. Topology and CPU mapping are unchanged.
- Variant A also passed, but B had the lower measured peak RSS. Multiplying capacity by batch size shows that each configured queue/pool branch retains the same item capacity as baseline (ingress 16,384; each broadcast branch 16,384; each shard branch 393,216; each fan-in branch 131,072). Variant B uses one-quarter as many queue slots with four times the batch size.
- Ten paired full-process launches per mapping were all lossless. Baseline median was 1.025 s; Variant B median was 0.715 s (about 30.2% faster). Candidate was faster in all ten pairs; exact sign-test `p=0.00195`. Every output retained the expected totals and reported zero queue/pool drops, logged errors, and pools in use.
- Benchmark medians also improved: `BenchmarkRun` 540.8 ms to 200.5 ms; `BenchmarkFullShowcase` 1,190.7 ms to 772.3 ms. Peak RSS maximum fell from 272,568 KB to 248,524 KB.
- `go test ./examples/showcase/...` passed. The selected config remains in the pre-existing showcase source, uncommitted as the user requested; no user source or dataset was staged/committed.
- Raw results and per-run outputs: `/tmp/opencode/showcase-capacity-*`.
