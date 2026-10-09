# Handler timing modes

Go 1.26.0, Linux amd64, Intel Core Ultra 7 255U; GOMAXPROCS benchmark suffix 14.

```sh
go test -run '^$' -bench BenchmarkTimingModes -benchmem -benchtime=100ms -count=3 ./graph
```

| Mode | ns/op samples | B/op | allocs/op |
|---|---|---:|---:|
| off | 11.94, 11.84, 12.00 | 0 | 0 |
| full | 47.35, 46.39, 46.07 | 0 | 0 |
| every 64 calls | 15.15, 14.68, 14.41 | 0 | 0 |

These cases invoke a normalized generated source handler without worker scheduling. They illustrate clock/counter overhead on this fixture, not application throughput guarantees. Full output: `/tmp/opencode/gpf-timing-modes.txt`.
