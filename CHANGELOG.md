# Changelog

User-visible changes are collected here before a release is selected. This file
does not announce a version or establish publication history. See
[release preparation](RELEASE.md) for open decisions and verification evidence.

## [Unreleased]

Target: **v0.1.0 experimental**, with no pre-1.0 API-stability promise. No
release, tag, or publication is announced here. Licensing is explicitly deferred;
license type and copyright holder remain blockers before publication.

### Added

- A finite [quickstart](examples/quickstart/README.md): typed string ingress,
  integer parsing, a separate typed failure sink, explicit CPU selection,
  cooperative drain, and optional logical-topology DOT export.
- A getting-started entry point and release-preparation checklist distinguishing
  declared Go requirements from compatibility evidence.
- A [deterministic fixture generator](examples/testdata/README.md) with explicit
  output paths and overwrite refusal; large JSON inputs are optional for ordinary
  tests.

### Changed

- [Showcase](examples/showcase/README.md) uses standard monotonic handler timing
  by default, with explicit TSC opt-in. Showcase and standalone provide checked-in
  full-workflow benchmarks without source overlays; one-iteration smoke results
  are not a new performance baseline.
- Five topic specifications replace duplicated historical stage requirements,
  while [open conformance findings](RELEASE.md#conformance-review-gates) remain
  explicit publication gates rather than implied runtime fixes.

### Fixed

- Compile-failure assertions now check the expected type mismatches rather than
  compiler-version-specific function names, preserving compile-time rejection
  checks on Go 1.22.12 as well as Go 1.26.8.

### Current capability baseline

- Typed preallocated batches, generated stateful and closure handlers, typed
  connections, deterministic DAG validation, and immutable reusable definitions.
- Fixed manual workers, explicit affinity with locked-thread fallback, ingress,
  group replicas, node worker pools, explicit broadcast/sharding/fan-in, and
  queue selection from physical endpoint concurrency.
- Opt-in automatic planning with explicit CPU-complete finalization, numeric
  metrics and counters, optional timing, and bounded asynchronous error logging.
- Inactive NUMA annotations and allocator descriptors; no active NUMA placement.

### Known limitations

- No API-stability or broad Go/OS support promise is established by this entry.
  License type, copyright holder, and publication approval remain unresolved.
- Shutdown deadlines are cooperative; there is no panic recovery, automatic
  handler retry, dynamic worker resizing, or implicit parallel-path ordering.
- Some topology combinations are unsupported. See the
  [README limits](README.md#limits-and-operational-expectations) and
  [specification guide](spec/README.md) for the detailed contracts.
- Source-verified lifecycle, metadata, and benchmark-dimension findings require
  resolution or explicit owner acceptance; passing checks do not close them.
