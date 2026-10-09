## Context
See proposal.md. Existing matrix entries for node-call and local-edge skip; pool, remote, and error cases already run.
## Goals / Non-Goals
Goals: reproducible executable baselines and serial allocation gates. Non-goals: new queue implementations or throughput thresholds.
## Decisions
- Invoke a concrete generated handler directly for node-call. Measure local-edge via a one-group graph with a built-in local edge and a request/delivery gate, because that queue is private to the executor; initialize pools/runners before measurement.
- Keep reserved names and batch dimensions. Record consumed batches, not uncontrolled background source calls; verify batches contain the advertised logical size.
- Reuse existing ReportAllocs and warm-up conventions. Separate snapshots and lifecycle from measured work. Node-call excludes workers; local-edge reports end-to-end one-group request/delivery cost, including its scheduler gate, and is not claimed as an isolated ring operation. Both fixtures process a logical 64-element batch.
## Risks / Trade-offs
- Compiler elimination or invalid batch size → Consume observable results and assert logical lengths outside timed iterations.
- Allocation tests overlap parallel tests → Keep AllocsPerRun tests serial.
## Migration Plan
Requires the ownership fix. Replace only the skipped entries, run allocation gates, and retain same-toolchain benchmark reports.
