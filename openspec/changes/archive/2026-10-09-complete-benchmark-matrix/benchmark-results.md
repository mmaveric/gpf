# Completed Stage 0 benchmark entries

Toolchain: Go 1.26.0, linux/amd64.
Machine: Intel Core Ultra 7 255U, benchmark GOMAXPROCS suffix 14.

Command:

```sh
go test -run '^$' -bench 'BenchmarkDataPlane/(node-call|local-edge)' -benchmem -benchtime=100ms -count=3 ./internal/benchtest
```

| Case | ns/op, three runs | B/op | allocs/op |
|---|---|---:|---:|
| node-call/batch=64 | 21.60, 22.13, 21.60 | 0 | 0 |
| local-edge/batch=64/group=1 | 2049, 6488411, 6788475 | 0 | 0 |

The local-edge fixture measures request-to-delivery through the single-group executor, including scheduler handoff. Its short samples have extreme scheduling variance and are not evidence for a throughput claim. Both formerly skipped cases now execute the advertised 64-element work and retain zero-allocation gates. Full output was captured at `/tmp/opencode/gpf-benchmark-matrix-after.txt`.
