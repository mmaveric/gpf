# Select the first profile-backed optimization

Type: grilling
Mode: HITL
Status: resolved

## Question

Given the fresh baseline and profiles, which measured cost center should be the first optimization target for the GPF showcase?

The lossless configuration now passes ten full launches, but it raises median peak RSS from 176,348 KB to 254,972 KB and `BenchmarkRun` bytes/op by 64.61%. The speed comparison is not statistically significant (`p=0.859`; five baseline samples versus ten current samples). The fresh CPU profile shows `graph.(*groupCounters).recordPass` at 20.49% flat and `graph.(*executorWorker).pass` at 65.41% cumulative; allocation space is led by `showcase.Run` and pool creation.

Choose one first target based on these measurements. Compare likely end-to-end speed impact, correctness/ownership risks, memory cost, and whether the work belongs in showcase code, a narrowly scoped GPF core path, or graph configuration. Preserve the topology and output requirements recorded on the map. Record the evidence and hypothesis; implementation is a separate step after the route is clear.

## Answer

The user selected **scheduler metrics** as the first optimization target. Investigate the per-pass cost in `graph.(*groupCounters).recordPass` and `graph.(*executorWorker).pass`, preserving the live `Executor.Metrics()` contract and exact stopped totals. If a meaningful speedup requires stale snapshots or another API contract change, stop and open a new HITL decision rather than weakening the contract. Keep the lossless capacity configuration during this optimization; its memory cost is a secondary metric, not part of this first change.
