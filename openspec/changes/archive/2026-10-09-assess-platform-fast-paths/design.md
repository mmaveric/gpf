## Context
See proposal.md. Portable rings and metrics expose measurable hot paths; no optimized variant is justified merely by the roadmap.
## Goals / Non-Goals
Goals: evidence-backed optional optimization or explicit no-go. Non-goals: mandatory cgo, changing ownership, or promising a speedup.
## Decisions
- Profile one queue/pool path after prerequisites complete, record toolchain/hardware, and compare repeated serial same-topology benchmarks against pure Go.
- Select the smallest justified optimization; isolate any unsafe/assembly/cgo code behind explicit build tags plus OS/architecture guards. Portable default and CGO_ENABLED=0 remain supported.
- Verify semantic, race where supported, and zero-allocation conformance against identical fixtures. Cross-build unsupported targets to confirm fallback selection.
- If no reproducible benefit exists, deliver a benchmark decision report and no optimized code; do not manufacture a candidate simply to check a task.
## Risks / Trade-offs
- Toolchain or pinning noise mimics a gain → Same-toolchain repeated measurements and recorded variability.
- Optimized code loses portability → Pure-Go forced path, no-cgo lane, and fallback cross-build checks.
## Migration Plan
Requires completed benchmark matrix, concurrent queues, and base metrics. Default dispatch remains portable; any enabled candidate must be explicitly documented and reversible.
