## Context
See proposal.md. NUMA annotations and allocator descriptors exist but are inactive by design; the roadmap restricts activation to a separate future stage.
## Goals / Non-Goals
Goals: write a grounded future design and decision checklist. Non-goals: implement discovery, change allocation, or enable NUMA placement.
## Decisions
- Produce docs/numa-placement-design.md describing topology snapshot inputs, CPU-to-NUMA mapping, allocator ownership, batch move locality, fallback, and OS/architecture support.
- Separate known metadata contracts from unresolved activation decisions. Include local/remote memory benchmarks, failure policy, and versioned migration questions as explicit future acceptance gates.
- Require any future implementation proposal to choose allocator behavior and platform policy before code. This preparation change is complete when documents are reviewable, not when those later decisions are guessed.
## Risks / Trade-offs
- Documentation reads like a shipped feature → Label inactive status and distinguish proposed behavior from existing APIs.
- Platform assumptions age → Cite primary platform/runtime sources and record the research date.
## Migration Plan
Requires NUMA-ready metadata. Documentation only with skip_specs=true; no runtime migration or application code edits.
