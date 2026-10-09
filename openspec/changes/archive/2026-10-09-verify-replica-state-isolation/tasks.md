## 1. Isolation Coverage
- [x] 1.1 Confirm group replicas and node worker pools are complete; verify their state/lifecycle suites before extending coverage.
- [x] 1.2 Add nested mutable explicit-state tests for both modes; verify every physical instance has independently mutated maps/slices/pointers.
- [x] 1.3 Add closure-environment and sequential-restart tests; verify once-per-instance factories and no state reuse across Starts.
## 2. Rollback and Documentation
- [x] 2.1 Inject factory/Init failures at each replica position; verify owner-worker callbacks and exactly-once reverse cleanup.
- [x] 2.2 Run `go test -race ./...`, update capture-ownership examples, and run `openspec validate verify-replica-state-isolation --strict`; verify skip_specs remains appropriate and no runtime behavior/API changes are added.
