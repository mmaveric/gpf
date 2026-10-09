## 1. Timing Modes
- [x] 1.1 Confirm `add-runtime-metrics` is complete; verify base snapshots before adding duration fields.
- [x] 1.2 Add disabled/full/every-N configuration and validation; verify zero N rejects and default disabled behavior remains unchanged.
- [x] 1.3 Instrument generated handler entry/return with a monotonic default source; verify call-one sampling and no selector/Wait/publication timing.
- [x] 1.4 Add measured count/total/max snapshot fields; verify failed and successful calls count, while unsampled work is not extrapolated.
## 2. Measurement and Completion
- [x] 2.1 Regenerate bindings and add deterministic clock fixtures plus serial allocation gates; verify exactly two readings per measured handler and none otherwise.
- [x] 2.2 Benchmark off/full/sampled modes, reconcile current timing deferral deltas/docs, and run `go test -race ./...` and `openspec validate add-processing-timing --strict`.
