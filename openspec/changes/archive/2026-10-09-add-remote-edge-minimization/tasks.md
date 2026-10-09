## 1. Deterministic Coalescing
- [x] 1.1 Confirm auto-grouping constraints are complete; verify fixed-label/cut fixtures before algorithm work.
- [x] 1.2 Implement canonical adjacent-component union and legal-merge checks; verify every merge respects labels/cuts and never increases remote-edge count.
- [x] 1.3 Check quotient acyclicity per merge; verify the A→B→C plus A→C contraction regression cannot create a group cycle.
## 2. Properties and Reporting
- [x] 2.1 Iterate to local minimality and report before/after costs/rejections; verify deterministic repeated results and zero remote edges for unconstrained DAGs.
- [x] 2.2 Add generated-DAG property tests and document local-not-global guarantees; verify all constraints and quotient acyclicity across fixtures.
- [x] 2.3 Run affected Go tests and `openspec validate add-remote-edge-minimization --strict`; verify no worker/factory materialization is introduced.
