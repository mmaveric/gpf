## ADDED Requirements

### Requirement: Opt-in processing timing configuration
The graph contract SHALL expose TimingConfig with Mode and Every fields and SetTiming(*Builder, TimingConfig) error. The default SHALL disable timing without clock reads. Sampled mode SHALL require a nonzero interval; unsupported modes and nonzero intervals for full/disabled modes SHALL reject without storing configuration. Builder configuration SHALL be copied and preserved by the immutable definition. NodeMetrics SHALL expose MeasuredCalls, MeasuredNanoseconds, and MaxMeasuredNanoseconds. This opt-in capability enables handler timing but SHALL not enable exporters, user clock injection, logger timing, or extrapolated estimates.

#### Scenario: Configure sampled timing
- **WHEN** a caller sets sampled timing with interval three before Build
- **THEN** each node independently measures returned calls one, four, seven, and so forth using the built-in monotonic source
