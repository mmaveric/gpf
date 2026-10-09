## 1. Typed Distribution
- [x] 1.1 Confirm ownership and base metrics are complete; verify existing moved-batch and loss-accounting tests.
- [x] 1.2 Add typed distribution configuration and atomic branch validation; verify foreign/duplicate/incompatible destinations cause no topology mutation.
- [x] 1.3 Implement bounded source cursor and per-branch pooled staging; verify full splitting, end-of-input partial flushing, and original element values.
- [x] 1.4 Implement cached branch selection across Wait; verify the selector runs once per element and invalid indices follow the documented user contract.
## 2. Loss and Lifecycle
- [x] 2.1 Add independent queue/pool element-loss reasons and pending cleanup; verify pool exhaustion does not fabricate batch losses and Close reclaims all staging.
- [x] 2.2 Add loss/duplication property tests, remote race fixtures, and serial allocation gates; verify accepted elements occur once without staging growth.
- [x] 2.3 Reconcile current distribution deferral deltas/docs and run `openspec validate add-element-sharding --strict` plus affected Go tests.
