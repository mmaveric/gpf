## 1. Fixtures
- [x] 1.1 Confirm `fix-moved-input-ownership` is complete; verify the moved-input regression and existing allocation suite pass.
- [x] 1.2 Implement the concrete node-call fixture with a logical 64-element batch and observable consumed result; verify the reserved case runs instead of skipping.
- [x] 1.3 Implement the built-in local-edge fixture with setup excluded; verify length, origin ownership, and consumed operation count outside timing.
## 2. Measurement and Completion
- [x] 2.1 Add serial AllocsPerRun gates for both fixtures; verify zero framework allocations with no parallel-test overlap.
- [x] 2.2 Run repeated same-toolchain benchmarks and retain hardware/flags/variability in a report; verify ReportAllocs and stable case dimensions.
- [x] 2.3 Run `go test ./...`, `git diff --check`, and `openspec validate complete-benchmark-matrix --strict`; verify documentation agrees with executable measurements.
