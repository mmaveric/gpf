# platform-fast-path-policy Specification

## Purpose
Define evidence, compatibility, and portable fallback conditions for optional platform-specific optimization.

## Requirements

### Requirement: Evidence gates optional optimized paths
An optimized variant SHALL be introduced only with recorded same-toolchain/hardware benchmark evidence for its affected path. A measured no-go result SHALL be an acceptable outcome without optimized code. Reports SHALL include variability and allocation results and SHALL not claim a gain from unsupported evidence.
#### Scenario: Reject an unproven variant
- **WHEN** measured samples do not support a reproducible improvement
- **THEN** the report records no-go and the portable implementation remains the only enabled path

### Requirement: Preserve pure-Go fallback conformance
Any optimized variant SHALL be isolated by platform/build tags and preserve the same ownership, capacity, and zero-allocation contracts as pure Go. Unsupported platforms, explicit portable builds, and no-cgo builds SHALL retain a working fallback.
#### Scenario: Build without optimized support
- **WHEN** the target lacks the optimized implementation or cgo is disabled
- **THEN** the package compiles using the portable path and passes applicable semantic conformance checks
