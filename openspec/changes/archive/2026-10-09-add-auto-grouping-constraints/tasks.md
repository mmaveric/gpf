## 1. Hard Constraints
- [x] 1.1 Confirm group replicas and node worker pools are complete; verify their physical identity and boundary metadata tests.
- [x] 1.2 Normalize explicit manual labels separately from default effective placement; verify unplaced nodes remain free only in automatic mode.
- [x] 1.3 Add must-link/must-separate physical constraints; verify conflicting labels and cuts reject without factory calls or topology mutation.
## 2. Preview and Verification
- [x] 2.1 Expose copied constraint rows after physical expansion; verify defensive copies and preserved manual mode.
- [x] 2.2 Add mixed replica/pool/manual boundary tests, update planning docs, and run affected Go tests and `openspec validate add-auto-grouping-constraints --strict`.
