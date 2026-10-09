## Why
Stage 8 allows platform-specific optimization only after benchmark evidence. Adding unsafe, assembly, or cgo code without a measured bottleneck would obscure the portable baseline.
## What Changes
- Measure one identified queue/pool hot path against the same-toolchain pure-Go baseline.
- Add an optional build-tagged variant only if evidence supports it, with portable fallback conformance.
## Capabilities
### New Capabilities
- `platform-fast-path-policy`: Evidence and fallback requirements for optional optimized variants.
### Modified Capabilities
None.
## Impact
Affects performance reports and, conditionally, isolated platform files. Depends on `complete-benchmark-matrix`, `add-bounded-concurrent-queues`, and `add-runtime-metrics`. A documented no-go outcome is a valid completion; no speedup is promised.
