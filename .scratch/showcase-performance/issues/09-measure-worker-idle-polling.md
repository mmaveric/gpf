# Measure worker idle polling

Type: task
Mode: AFK
Status: resolved
Blocked by: 08

## Question

Can a narrowly scoped change to `executorWorker` idle polling improve full-workflow wall time without changing live metrics, Drain behavior, or output?

### Work

- Establish a same-environment before measurement using `BenchmarkRun`, `BenchmarkFullShowcase`, and ten sequential prebuilt launches with the current lossless capacities and unchanged dataset.
- Test a narrowly scoped idle-poll/yield adjustment around `executorWorker.pass`; do not change topology, counters' snapshot semantics, queue behavior, or public APIs.
- Run the same measurements serially after the change. Confirm exact output, zero drops/errors, and existing lifecycle/drain tests.
- Keep a code change only if repeated measurements show a statistically significant improvement in the full-workflow wall-time metric. Record CPU use as diagnostic; if only CPU or memory changes, revert the candidate.
- If a viable design requires queue notifications, blocking waits, API changes, or a new latency contract, stop and open a separate HITL decision ticket rather than expanding this task.

### Resolution evidence

Attach before/after benchmark and launch samples, profiles, test results, and the final diff or no-change finding.

## Answer

- Tested changing the idle-yield threshold from `1<<20` to `1<<24`; exact outputs and all tests passed, but the candidate was reverted because the primary prebuilt-launch metric did not improve significantly.
- `BenchmarkFullShowcase` improved from 1.465 to 1.235 s/op (`-15.69%`, `p=0.019`); `BenchmarkRun` was not significant (`p=0.579`). The ten-launch benchmark reported 1.143 s median before and 1.107 s after (`p=0.971`), so the required full-process speedup was not established.
- All launches had exact totals and zero drops/errors. Median peak RSS was 264,382 KB before and 254,368 KB after.
- `go test ./graph ./examples/showcase/...` and `go test -race ./graph` passed. No code change or commit was retained.
- Raw samples and profiles are in `/tmp/opencode/worker-idle-*`; the experiment branch/worktree was `ticket/measure-worker-idle-polling` / `/tmp/opencode/gpf-worker-idle-polling-wt`.
