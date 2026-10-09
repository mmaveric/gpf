## 1. Public Preview
- [x] 1.1 Confirm remote minimization and queue selection are complete; verify pure planner fixtures before public integration.
- [x] 1.2 Add opt-in expansion→constraints→coalescing→queue-selection preview; verify no runtime factory or live-resource creation.
- [x] 1.3 Assign stable noncolliding group IDs preserving explicit labels; verify canonical ordering and repeat-plan identity.
- [x] 1.4 Add owner/revision freshness metadata; verify registration/connection/placement changes invalidate prior previews without sealing the builder.

## 2. Verification
- [x] 2.1 Add copied-row and mixed parallel-topology tests; verify manual defaults and CPU-incomplete preview status remain explicit.
- [x] 2.2 Update planning docs and run affected Go tests and `openspec validate add-automatic-group-planning --strict`; verify preview cannot start unbound workers.
