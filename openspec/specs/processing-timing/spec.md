# processing-timing Specification

## Purpose
Define opt-in full and deterministic sampled handler duration measurements for runtime inspection.

## Requirements

### Requirement: Explicit timing modes
Timing SHALL support disabled, full, and every-N-call modes configured before Build. Sampled N=0 SHALL be rejected. Sampling SHALL begin at call one independently per physical node and include successful and failed returned handlers. Disabled and unsampled calls SHALL perform no timestamp reads.
#### Scenario: Sample every third call
- **WHEN** one node returns seven handlers with sampling interval three
- **THEN** calls one, four, and seven are measured and all other calls avoid timestamp reads

### Requirement: Report measured handler duration only
Snapshots SHALL report measured call count, total measured nanoseconds, and maximum measured nanoseconds, excluding selector, output publication, Wait retry, and logger work. Unsampled work SHALL not be extrapolated. Framework timing updates SHALL allocate no steady-state memory.
#### Scenario: Retry an output
- **WHEN** a completed handler's output retries a full Wait queue
- **THEN** retries do not increase measured handler count or handler duration totals
