## Context
See proposal.md. Physical expansion introduces mandatory boundaries alongside manual logical placements.
## Goals / Non-Goals
Goals: immutable constraints for later planning. Non-goals: merging components or starting automatic workers yet.
## Decisions
- Normalize explicit manual labels, replica pairing, worker-pool cuts, and ingress/external boundaries into copied physical constraint records.
- Constraints distinguish must-link components from must-separate worker boundaries. Reject a component containing conflicting fixed manual labels or both a must-link and must-separate pair.
- Place remains authoritative; unplaced nodes are free only in opt-in automatic mode. Manual mode keeps DefaultGroupID behavior.
- Expose a constraint preview without factories or queues so the minimizer can consume stable numeric data.
## Risks / Trade-offs
- Treating default placement as an explicit label prevents all optimization → Track explicit Place facts separately from effective manual defaults.
- Expansion hides a hard cut → Normalize after replica/pool expansion and test conflicting synthetic plans.
## Migration Plan
Requires both parallelism modes. Add planning metadata first; runtime auto-mode remains disabled until CPU-complete plan integration.
