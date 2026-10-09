## 1. Future Design
- [x] 1.1 Confirm NUMA metadata is complete and read its final contracts; verify the existing inactive behavior baseline is documented accurately.
- [x] 1.2 Research primary platform/runtime sources and record dates/links; verify CPU-domain discovery and allocation-locality claims against those sources.
- [x] 1.3 Write `docs/numa-placement-design.md` covering locality, batch ownership, fallback, and migration; verify current versus proposed behavior is explicitly separated.
## 2. Decision Gates
- [x] 2.1 Add an activation decision checklist and local/remote memory benchmark acceptance plan; verify allocator, platform support, and failure policy decisions are required before a future implementation proposal.
- [x] 2.2 Review document links and run `openspec validate plan-numa-placement --strict` plus `git diff --check`; verify skip_specs is appropriate and no application code changes activate NUMA behavior.
