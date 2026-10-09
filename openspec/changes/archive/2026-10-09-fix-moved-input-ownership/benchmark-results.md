# Moved-input ownership performance check

Toolchain: Go 1.26.0, linux/amd64.
Machine: Intel Core Ultra 7 255U, benchmark GOMAXPROCS suffix 14.

Command:

```sh
go test -run '^$' -bench 'BenchmarkDataPlane/(local-error|remote-error|end-to-end-dag)' -benchmem -benchtime=100ms -count=3 ./internal/benchtest
```

The pre-instrumentation baseline is recorded in `../add-runtime-metrics/benchmark-results.md`. Both runs report `0 B/op` and `0 allocs/op` for these cases.

| Case | Before ns/op | After ns/op |
|---|---|---|
| local-error/batch=64/groups=1 | 10838, 5848208, 6184884 | 195.8, 6519950, 2846861 |
| remote-error/batch=64/groups=2 | 5666764, 6500256, 6368432 | 6363034, 6546540, 6144932 |
| end-to-end-dag/batch=64/groups=2/depth=3 | 6782190, 5927040, 6626741 | 5891162, 5732929, 6222207 |

The local-error and remote cases show high scheduling/affinity variability; this short check supports the zero-allocation result but not a throughput claim. Full output was captured at `/tmp/opencode/gpf-ownership-after.txt`.
