## 1. Clock Injection
- [x] 1.1 Confirm `add-processing-timing` is complete; verify full/sampled baseline timing tests.
- [x] 1.2 Add immutable optional func() uint64 configuration; verify nil fallback and zero callback calls during Build and snapshots.
- [x] 1.3 Use the custom clock only at measured handler boundaries; verify 100/140 produces forty nanoseconds and disabled/unsampled calls produce no readings.
## 2. Verification
- [x] 2.1 Add deterministic modular-wrap and concurrency-safe shared-clock tests; verify race and serial allocation gates under all timing modes.
- [x] 2.2 Document nanosecond/concurrency/return obligations and run affected Go tests and `openspec validate add-custom-timestamp-source --strict`; verify no clock-discovery policy is implied.
