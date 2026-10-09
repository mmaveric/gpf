## MODIFIED Requirements

### Requirement: Opt-in processing timing configuration
The graph contract SHALL expose TimingConfig with Mode, Every, and optional Clock func() uint64 fields and SetTiming(*Builder, TimingConfig) error. The default SHALL disable timing without clock reads. Sampled mode SHALL require a nonzero interval; unsupported modes and nonzero intervals for full/disabled modes SHALL reject without storing configuration. Builder configuration SHALL be copied and preserved by the immutable definition. Nil Clock SHALL select the built-in monotonic source for enabled timing. A configured Clock SHALL remain dormant during Build and snapshot reads, and disabled/unsampled dispatch SHALL not call it. NodeMetrics SHALL expose MeasuredCalls, MeasuredNanoseconds, and MaxMeasuredNanoseconds. Custom callbacks SHALL satisfy the nanosecond, modular monotonic, concurrent-safe, nonblocking/nonallocating contract. Timing SHALL not enable exporters, logger timing, or extrapolated estimates.

#### Scenario: Configure sampled timing
- **WHEN** a caller sets sampled timing with interval three before Build
- **THEN** each node independently measures returned calls one, four, seven, and so forth using the configured source or built-in fallback

#### Scenario: Configure a dormant clock
- **WHEN** a clock is configured with timing disabled
- **THEN** registration, Build, processing, and snapshots do not invoke it
