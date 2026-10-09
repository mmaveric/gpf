## 1. Copying Broadcast
- [x] 1.1 Confirm ownership and base metrics are complete; verify existing single-output contracts before extending explicit connections.
- [x] 1.2 Add typed broadcast registration and uniform-capacity validation; verify all-or-nothing endpoint mutation and unchanged ordinary/error cardinality.
- [x] 1.3 Materialize branch pools and shallow typed copies; verify distinct slot ownership with original logical elements.
- [x] 1.4 Track branch completion and independent Wait/Drop outcomes; verify retries neither recopy nor redeliver accepted branches.
## 2. Lifecycle and Verification
- [x] 2.1 Add pool-exhaustion/forced-close/drain tests; verify only affected branches lose work and every pending copy releases once.
- [x] 2.2 Add local/remote property/race tests and serial allocation gates; verify fixed staging bounds and zero framework copying allocations.
- [x] 2.3 Document shallow-copy/capacity preconditions, record fixed-topology replica/worker-pool incompatibility, reconcile current broadcast deferral deltas, and run `openspec validate add-broadcast-fan-out --strict` plus affected Go tests.
