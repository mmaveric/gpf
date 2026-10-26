# Stabilize the million-order showcase run

Type: task
Mode: AFK
Status: resolved
Blocked by: 01

## Question

What minimal, in-scope change makes the million-valid-order showcase run preserve every order under repeated execution?

### Work

- Reproduce the missing orders and confirm which bounded resource is exhausted; baseline artifacts are in `/tmp/opencode/showcase-baseline-artifacts/`.
- Preserve the existing node/edge topology and report semantics. Capacity tuning is allowed; change framework behavior only if configuration cannot meet the lossless workload contract.
- Add a regression check for the million-order expected totals and zero queue/pool drops. Keep the existing separate error-route test.
- Repeat full prebuilt-binary launches against the unchanged `examples/data.json`; require exact totals and zero drops on every run. Record any memory or timing effect.
- Do not pursue a separate performance optimization in this ticket. The lossless behavior and a fresh profile-backed baseline unblock the optimization decision.

### Resolution evidence

Record the root cause, minimal fix, regression check, and repeated-run outputs. The next decision ticket remains blocked until this evidence is available.

## Answer

- **Root cause:** `ConnectShard` advances past an order when a branch's bounded batch pool is exhausted. The million-order run could saturate shard branch pools; fan-in result pools also exhausted independently at low capacities.
- **Retained configuration:** broadcast capacity 4,096; shard capacity 196,608; fan-in capacity 65,536. The shard setting was the lowest tested that passed the repeated deterministic run test (163,840 still dropped one order in 20 runs). Fan-in 8,192 still logged errors; 65,536 passed. Topology and core exhaustion semantics are unchanged.
- Added `TestRunPreservesMillionDeterministicOrdersAcrossRepeatedRuns`; it checks exact totals, zero queue/pool drops, and zero logged errors over ten calls. It passed three repeated invocations (30 complete runs). The separate error-route test also passes.
- Ten sequential launches against the unchanged dataset passed with exact totals and zero reported losses. Dataset SHA-256: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`.
- Median full-launch time was 1.13 s versus the previous 1.165 s; this comparison does not establish a significant speedup. Median peak RSS rose from 176,348 KB to 254,972 KB. `BenchmarkRun` allocated 225.7 MiB/op versus 137.1 MiB/op (+64.61%, p=0.001); time was not statistically different (`p=0.859`, baseline `n=5`, current `n=10`).
- The current CPU profile attributes 20.49% flat samples to `graph.(*groupCounters).recordPass` and 65.41% cumulative time to `graph.(*executorWorker).pass`. Allocation space is led by `showcase.Run` and pool creation.
- Regression test commit: `9add6ce` (`test(showcase): guard million-order runs`). The capacity edits in `examples/showcase/internal/showcase/topology.go` remain uncommitted as requested. Detailed raw results and profiles are in `/tmp/opencode/showcase-stabilize-refinement/`.
