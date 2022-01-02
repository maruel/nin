# Performance

Right now nin is not fast enough to be usable as a ninja replacement. Thankfully
we can directly compare performance and find the obvious areas to optimize.


## Profiling

### Memory profile

Generate a memory dump of allocated objects.

```
go test -memprofile mem.prof
# or
nin -memprofile mem.prof
# then
go tool pprof -http :8010 mem.prof
```

Visit http://localhost:8010/ui/flamegraph?si=alloc_objects


### CPU sampling

Saves a sample at ~100Hz. For short runs it may not be useful enough.

```
go test -cpuprofile mem.prof
# or
nin -cpuprofile cpu.prof
# then
go tool pprof -http :8010 cpu.prof
```

Visit http://localhost:8010/ui/flamegraph?si=cpu

**Hint:** Use `-http 0.0.0.0:8010` to be remotely accessible.


## Tracing

Tracing keeps much more information by tracing everything that happens. It is
more useful for short runs.

```
go test -trace=trace.out -bench=.
# or
nin -trace trace.out
go tool trace -http=:33071 trace.out
```


## Comparing perf

This runs both ninja's performance tests and nin's equivalent for an
apple-to-apple comparison.

```
./compare.sh
```

## Tips

Use benchstat to compare Go benchmarks.

```
go install golang.org/x/perf/cmd/benchstat@latest
go test -count=10 -bench=. -run '^$' -cpu 1 > new.txt
git checkout HEAD~1
go test -count=10 -bench=. -run '^$' -cpu 1 > old.txt
benchstat old.txt new.txt
```
