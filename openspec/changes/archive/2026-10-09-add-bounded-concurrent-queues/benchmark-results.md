# Bounded concurrent queue model baseline

Toolchain: Go 1.26.0, linux/amd64.
Machine: Intel Core Ultra 7 255U, benchmark GOMAXPROCS suffix 14.

Command:

```sh
go test -run '^$' -bench BenchmarkConcurrentQueues -benchmem -benchtime=100ms -count=3 ./queue
```

Each case is a serial push/pop round trip after preallocated setup; these are primitive baselines, not contended multi-worker throughput comparisons.

| Model | ns/op | B/op | allocs/op |
|---|---|---:|---:|
| MPSC/capacity=64 | 53.66, 50.91, 50.86 | 0 | 0 |
| SPMC/capacity=64 | 52.46, 50.51, 50.70 | 0 | 0 |
| MPMC/capacity=64 | 51.72, 51.21, 50.59 | 0 | 0 |

Full output: `/tmp/opencode/gpf-concurrent-queues-after.txt`.
