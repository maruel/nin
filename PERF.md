# Performance

Right now nin is not fast enough to be usable as a ninja replacement. Thankfully
we can directly compare performance and find the obvious areas to optimize.

## Profiling

The memory dump is kinda useful but the sampling profiler is currently mostly
useless.

Run:

```
nin -cpuprofile cpu.prof -memprofile mem.prof
go tool pprof -http :8010 cpu.prof
go tool pprof -http :8010 mem.prof
```

Visit http://localhost:8010/ui/flamegraph?si=cpu for CPU profile or
http://localhost:8010/ui/flamegraph?si=alloc_objects for memory.

Use `-http 0.0.0.0:8010` to be remotely accessible.


## Comparing perf

```
./compare.sh
```
