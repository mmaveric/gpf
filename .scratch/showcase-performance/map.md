# Improve performance in the GPF showcase

## Destination

Deliver a profile-backed speedup for `examples/showcase` on the full, prebuilt application workload while preserving its output and demonstrated topology. A narrowly scoped GPF core change is in scope only when fresh measurements show it is needed.

## Notes

- Domain: GPF showcase performance. Sessions should consult `golang-benchmark`, `golang-performance`, and `golang-concurrency` when measuring or changing concurrent hot paths.
- Primary success metric: statistically significant reduction in median wall time for repeated launches of a prebuilt `examples/showcase` binary using the same `examples/data.json`. Include file reading, JSON decoding, graph construction/start, processing, Drain, and report; exclude compilation.
- The standalone showcase is an unchanged contextual baseline, not a parity target.
- Keep the current million-valid-order dataset as the primary performance workload. Verify error routing separately.
- Preserve node/edge topology, outputs, and demonstrated features. Capacity and CPU-group tuning are allowed.
- Keep allocations and peak RSS as secondary metrics; discuss a material regression if one appears. Keep `BenchmarkRun` and `BenchmarkFullShowcase` for pipeline/full-workflow profiling, not as a CI performance gate.
- Per user choice, the pre-existing showcase source, current capacity settings, and local data file remain uncommitted; ticket-owned benchmark tests are committed to the integration branch.

## Decisions so far

- [Capture a fresh showcase baseline](issues/01-capture-fresh-showcase-baseline.md): timings and profiles were captured, but 2 of 10 runs lost orders to sharder pool exhaustion; a lossless run is now required before choosing an optimization.
- [Stabilize the million-order showcase run](issues/03-stabilize-million-order-showcase-run.md): shard/fan-in capacities now produce exact totals in repeated runs; the fix materially increases allocations and peak RSS, while the speed change is not statistically significant.
- [Select the first profile-backed optimization](issues/02-select-first-profile-backed-optimization.md): start with scheduler-pass metrics while preserving the live snapshot contract; defer any contract change for a separate decision.
- [Reduce scheduler metric overhead without weakening snapshots](issues/04-reduce-scheduler-metric-overhead.md): the worker-owned counter candidate was statistically indistinguishable or slower, so it was reverted without changing the live metrics contract.
- [Profile the full showcase workflow](issues/05-profile-full-showcase-workflow.md): full-path profiling attributes substantial allocation space to slice growth and `Run`, while the ten lossless launches retain exact totals.
- [Select the next profile-backed target](issues/06-select-next-profile-backed-target.md): optimize `showcase.Run` input buffering first, preserving mixed-input error routing and requiring a significant full-run speedup.
- [Stream valid orders without a redundant copy](issues/07-stream-valid-orders-without-copy.md): the candidate reduced allocations and RSS, but did not significantly lower end-to-end time, so it was reverted.
- [Select the next speed target](issues/08-select-next-speed-target.md): explore worker idle polling/scheduling separately from metric counter updates, without weakening live metrics or Drain behavior.
- [Measure worker idle polling](issues/09-measure-worker-idle-polling.md): a higher idle-yield threshold improved the test-benchmark workflow but not the full-process launch metric, so it was reverted.
- [Select a target after polling results](issues/10-select-target-after-polling-results.md): test a direct-pop path for single-input runners while retaining multi-input readiness checks.
- [Benchmark direct pop for single-input runners](issues/11-benchmark-single-input-direct-pop.md): direct pop did not improve the full-process metric significantly, so generator and generated changes were reverted.
- [Measure core-aware CPU placement](issues/12-measure-core-aware-cpu-placement.md): a one-thread-per-core map had a small, statistically insignificant speed estimate, so the existing CPU mapping was retained.
- [Decide whether to prototype wake-driven workers](issues/13-decide-on-wake-driven-workers.md): proceed with an internal wake-driven experiment only if it preserves public behavior and polling fallback.
- [Prototype wake-driven worker idling](issues/14-prototype-wake-driven-workers.md): the prototype improved pipeline benchmarks but regressed full-process time, so it was reverted.
- [Tune showcase batch capacities](issues/15-tune-showcase-batch-capacities.md): a lower-memory batch/queue configuration produced a statistically significant full-process speedup with exact totals and zero drops.
- [Test the next larger-batch capacity variant](issues/16-test-larger-batch-capacities.md): doubling batches again improved paired full-process launch time significantly; benchmark sec/op was not significant, and median RSS was nearly flat with one higher peak sample.
- [Test Variant D with doubled batches](issues/17-test-showcase-capacity-d.md): D beat C in 21/24 paired launches with exact output; retained it despite inconclusive benchmark sec/op.
- [Variant D framework profile report](profile-report-variant-d.md): the N=2048 profile shows worker-pass dispatch as the largest framework cost, logger cursor modulo as a low-risk target, and pool preallocation as the main framework allocation source.
- [Optimize logger cursor wrap](issues/19-optimize-logger-cursor-wrap.md): conditional wrap improved the isolated seven-ring logger benchmark by 70.73%; full-launch change was not significant.
- Disabled worker pass timing now skips the timing helper via a startup snapshot; isolated worker-pass benchmarks improve 4–28%, but full launches show no significant change. See the profile report for details.
- [Batch group scheduler metrics](issues/18-batch-group-scheduler-metrics.md): worker-local counters publish every configurable N passes; focused pass benchmark improves, full-launch change is not significant.

## Not yet specified

<!-- No unresolved decisions remain for the agreed destination. -->

## Out of scope

- Changes to `examples/standalone-showcase`.
- Removing or bypassing showcased topology/features to improve the score.
- A CI performance gate or unrelated GPF core optimization.
