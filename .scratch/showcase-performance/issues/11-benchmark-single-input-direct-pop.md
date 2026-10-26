# Benchmark direct pop for single-input runners

Type: task
Mode: AFK
Status: resolved
Blocked by: 10

## Question

Does removing the separate queue-readiness check for single-input runners produce a statistically significant full-process speedup without changing observable behavior?

### Work

- Modify the code generator, not only generated `graph/registration_gen.go`, to attempt `input1.pop()` directly for single-input runners. Regenerate the committed source. Keep readiness masks and selectors unchanged for multi-input runners.
- Preserve handler calls, queue ownership, pending outputs, errors, live metrics, Drain, and Close semantics. Use existing public-seam behavior tests; add a regression only if a contract is not covered.
- Compare `BenchmarkRun`, `BenchmarkFullShowcase`, and ten sequential prebuilt launches against the current lossless baseline; use repeated samples and `benchstat`. Keep the change only if full-process median wall time improves significantly.
- Confirm exact million-order totals, zero queue/pool drops and logged errors; record allocations and peak RSS as secondary metrics.
- If the direct-pop path is not significantly faster, revert both generator and generated changes and record the result.

### Resolution evidence

Attach the before/after benchmark and launch samples, generated-code diff, tests, profiles, and final implementation or no-change result.

## Answer

- Tried removing `ready()` and the readiness-mask/switch path for one-input runners, using direct `pop()`. Multi-input generated code was unchanged.
- The candidate did not meet the full-process speed criterion and was reverted. `BenchmarkFullShowcase` improved 5.7% by median but was not significant (`p=0.052`); `BenchmarkRun` was not significant (`p=0.436`); full-process launches were 3.4% slower by median and not significant (`p=0.089`).
- All ten baseline and candidate launch outputs were identical with exact million-order totals and zero drops/errors. Tests and code generation checks passed. No generated or generator change was retained or committed.
- Raw samples and candidate diff: `/tmp/opencode/direct-pop-*`; worktree/branch: `/tmp/opencode/gpf-single-input-direct-pop-wt` / `ticket/benchmark-single-input-direct-pop`.
