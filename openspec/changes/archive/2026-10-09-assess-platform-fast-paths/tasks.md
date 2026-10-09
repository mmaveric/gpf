## 1. Evidence
- [x] 1.1 Confirm completed matrix, bounded queues, and base metrics are complete; verify portable conformance suites before profiling.
- [x] 1.2 Profile one identified pool/queue hot path and retain repeated same-toolchain/hardware baseline results; verify the candidate addresses a measured bottleneck.
- [x] 1.3 Compare an isolated candidate or record no-go; verify the report includes variability, allocation outcomes, and no unsupported speedup claims.
## 2. Conditional Integration
- [x] 2.1 If evidence supports a candidate, isolate it by explicit platform/build tags; otherwise deliver the no-go artifact and verify no optimized runtime path was enabled.
- [x] 2.2 Run shared semantic/allocation fixtures, no-cgo builds, and unsupported-target cross-build checks; verify a working pure-Go fallback for every supported selection.
- [x] 2.3 Document dispatch/reversal and benchmark evidence, run applicable Go checks and `openspec validate assess-platform-fast-paths --strict`; verify the evidence gate is met or no-go is explicit.
