## Context
See proposal.md. Constraint records supply fixed labels and mandatory cuts on expanded DAG endpoints.
## Goals / Non-Goals
Goals: deterministic legal coalescing. Non-goals: global optimality claims for arbitrary constrained partitions.
## Decisions
- Process canonical physical edges in stable order; union adjacent components only when label/cut constraints permit and the quotient group graph remains acyclic.
- Repeat legal improving merges until no adjacent eligible merge remains. Report remote-edge counts before/after and rejected-merge reasons.
- This is deterministic local minimization, not an NP-hard optimal partition solver. With no constraints an entire DAG can coalesce into one group.
- Quotient-DAG checks are required: contracting a shortcut edge in A→B→C plus A→C can create a cycle even though the node graph is acyclic.
## Risks / Trade-offs
- Union-find alone permits group-level cycles → Validate the quotient graph for each proposed merge.
- Greedy choices differ from global optimum → State the guarantee as no remaining legal adjacent merge and test reproducibility.
## Migration Plan
Requires auto-grouping constraints. Land a pure control-plane algorithm before exposing automatic preview/finalization.
