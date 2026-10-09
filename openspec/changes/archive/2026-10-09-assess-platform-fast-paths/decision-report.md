# Platform fast paths: no-go for this iteration

Date: 2026-10-06. Go 1.26.0, Linux amd64, Intel Core Ultra 7 255U; benchmark GOMAXPROCS suffix 14.

## Evidence

After portable ownership, queue, and metrics conformance tests passed, measurements ran serially on the same working tree:

```sh
go test -run '^$' -bench BenchmarkConcurrentQueues -benchmem -benchtime=200ms -count=5 ./queue
go test -run '^$' -bench 'BenchmarkConcurrentQueues/MPMC' -benchtime=2s \
  -cpuprofile=/tmp/opencode/gpf-queue.cpu -o /tmp/opencode/gpf-queue.test ./queue
go tool pprof -top /tmp/opencode/gpf-queue.test /tmp/opencode/gpf-queue.cpu
```

| Serial push/pop + pool lifecycle fixture | ns/op samples | B/op | allocs/op |
|---|---|---:|---:|
| MPSC, capacity 64 | 61.62, 64.25, 64.16, 81.93, 68.09 | 0 | 0 |
| SPMC, capacity 64 | 77.03, 90.94, 81.55, 82.84, 80.56 | 0 | 0 |
| MPMC, capacity 64 | 65.36, 65.44, 61.41, 61.48, 59.57 | 0 | 0 |

The MPMC profile contains 2430 ms of samples. Inline Uint32 CAS accounts for 26.75% flat; Pool.Release 12.35%; Pool.Acquire 9.05%. Queue reservePush is 26.34% cumulative and tryPop 35.80% cumulative. These percentages overlap through call relationships and must not be added as independent cost categories.

Output is retained at `/tmp/opencode/gpf-platform-baseline.txt`, with CPU profile and binary at the paths above. The table is persisted here so the evidence remains readable when temporary files disappear.

## Decision

**No platform-specific variant is justified by this evidence.** The measured work is mostly the existing atomic ownership protocol and pool lifecycle, not an identified OS call or a missing machine-specific primitive. The compiler already lowers typed atomic operations to platform instructions. Replacing those calls with assembly/cgo alone does not establish a benefit; weakening the reservation/publication protocol would violate queue correctness. The fixture is serial, so it also cannot justify changing contention behavior for real multi-party workloads.

No candidate benchmark is claimed, no speedup is claimed, and no optimized source files, build tags, or dispatch switches are introduced. This is a deliberate evidence-gate no-go, not proof that future optimization is impossible. A future proposal should first profile representative contended workloads, isolate a specific bottleneck, and compare a correctness-preserving candidate with repeated same-environment samples.

## Portable conformance

The following checks passed:

- `CGO_ENABLED=0 go test ./...` — native semantic and allocation gates.
- `go test -race ./queue ./batch` — queue and pool ownership conformance.
- `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...`.
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...`.
- `CGO_ENABLED=0 GOOS=linux GOARCH=386 go build ./...`.

Cross-build checks establish compilation, not execution on those targets. Since no variant is enabled, the portable Go implementation is the sole dispatch path and there is no runtime reversal or configuration migration to perform.
