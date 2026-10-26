# Prototype wake-driven worker idling

Type: task
Mode: AFK
Status: resolved
Blocked by: 13

## Question

Can built-in graph edges wake idle workers on work arrival and queue-space changes, reducing repeated empty polling without changing public APIs or behavior?

### Work

- Prototype per-worker wake notifications for built-in queues and ingress. Include wakeups for both enqueue (consumer work) and dequeue (producer space under `queue.Wait`).
- Preserve worker control commands, live `Executor.Metrics()` semantics, Drain/Close barriers, ownership, exact outputs, and the current graph topology.
- Custom queue factories and user `Node0` source handlers must retain a polling fallback; do not add a public queue API or require custom factories/callbacks to implement notifications.
- Add public-seam tests for remote/local work, Wait backpressure, ingress, custom-queue fallback, and shutdown. Run the race detector.
- Compare current and prototype `BenchmarkRun`, `BenchmarkFullShowcase`, and ten sequential prebuilt launches against the same lossless workload. Keep the prototype only if full-process launch time improves statistically significantly.
- If this requires a public API, latency-contract, or broad queue redesign, stop and open a new HITL decision ticket instead of expanding the prototype.

### Resolution evidence

Attach the prototype implementation or no-change result, test/race output, before/after profiles and launch samples, and any new design decisions.

## Answer

- Prototyped coalesced internal worker wakeups while preserving public APIs, live metrics, Drain, custom-queue polling, and unsignaled `Node0` source polling. The candidate was reverted; no graph changes or commit were retained.
- All ten candidate launches had exact totals and zero drops/errors, but median prebuilt-launch time regressed from 1.105 s to 1.345 s (21.72%, `p=0.000`, `n=10`).
- `BenchmarkFullShowcase` improved 7.69% (`p=0.019`) and `BenchmarkRun` improved 27.66% (`p=0.003`), but the full-process result failed the acceptance criterion. Candidate allocations also increased (FullShowcase +157.35%, Run +61.04%).
- Tests passed: `go test ./graph ./examples/showcase/...` and `go test -race ./graph ./examples/showcase/internal/showcase`.
- Artifacts: `/tmp/opencode/wake-candidate-*` and `/tmp/opencode/showcase-profile-full/`. The experiment branch was `ticket/prototype-wake-driven-workers`; no code was merged.
