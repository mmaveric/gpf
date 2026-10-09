# Extended metrics primitive measurements

Go 1.26.0, Linux amd64, Intel Core Ultra 7 255U; GOMAXPROCS suffix 14.

```sh
go test -run '^$' -bench BenchmarkExtendedMetrics -benchmem -benchtime=100ms -count=3 ./graph
```

| Case | ns/op samples | B/op | allocs/op |
|---|---|---:|---:|
| counter-update | 3.537, 4.288, 3.544 | 0 | 0 |
| pool-observation | 0.4363, 0.4514, 0.4381 | 0 | 0 |
| worker-sampled-64 | 12.02, 12.09, 11.91 | 0 | 0 |

These are serial primitive/empty-worker fixtures, not contended application throughput claims. Full output: `/tmp/opencode/gpf-extended-metrics.txt`.
