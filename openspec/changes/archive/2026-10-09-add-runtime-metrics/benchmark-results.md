# Runtime metrics benchmark baseline

Toolchain: `go version go1.26.0 linux/amd64`.
Machine: Linux amd64, Intel Core Ultra 7 255U, benchmark GOMAXPROCS suffix 14.

The requested command `go test -run '^$' -bench . -benchmem ./...` exceeded the shell's 120-second limit during remote-error calibration. Partial output is retained at `/tmp/opencode/gpf-metrics-before.txt`.

A complete bounded baseline was captured with:

```sh
go test -run '^$' -bench . -benchmem -benchtime=100ms -count=3 -timeout=90s ./...
```

Full output: `/tmp/opencode/gpf-metrics-before-short.txt`. All measured cases reported `0 B/op` and `0 allocs/op`.

| Case | Baseline ns/op, three runs |
|---|---|
| pool-acquire-release/batch=64 | 12.67, 12.68, 12.73 |
| remote-queue/batch=64/groups=2 | 230.6, 600.3, 14005 |
| local-error/batch=64/groups=1 | 10838, 5848208, 6184884 |
| remote-error/batch=64/groups=2 | 5666764, 6500256, 6368432 |
| end-to-end-dag/batch=64/groups=2/depth=3 | 6782190, 5927040, 6626741 |
| pool-release/clear=false/batch=64 | 12.70, 12.84, 12.79 |
| pool-release/clear=true/batch=64 | 16.05, 15.98, 15.12 |

The baseline has substantial scheduling/affinity variability. These samples do not support a performance improvement or regression claim.

## Instrumented comparison

After implementation was measured with the same bounded command and environment:

```sh
go test -run '^$' -bench . -benchmem -benchtime=100ms -count=3 -timeout=90s ./...
```

Full output: `/tmp/opencode/gpf-metrics-after.txt`. All measured cases still reported `0 B/op` and `0 allocs/op`.

| Case | Baseline ns/op | Instrumented ns/op |
|---|---|---|
| pool-acquire-release/batch=64 | 12.67, 12.68, 12.73 | 12.57, 12.21, 12.23 |
| remote-queue/batch=64/groups=2 | 230.6, 600.3, 14005 | 747.6, 6083213, 6210630 |
| local-error/batch=64/groups=1 | 10838, 5848208, 6184884 | 6326359, 3548964, 6629699 |
| remote-error/batch=64/groups=2 | 5666764, 6500256, 6368432 | 6406139, 6210747, 5295922 |
| end-to-end-dag/batch=64/groups=2/depth=3 | 6782190, 5927040, 6626741 | 6600440, 6277764, 6208964 |

The scheduler/affinity spread is too large for a meaningful throughput conclusion from these short runs. The useful result is that the existing allocation gates and benchmark cases remain at zero allocations. No throughput pass/fail threshold is applied.
