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

### Against previous commit

Runs all Go benchmark on the current commit and the previous one, then compares
with [benchstat](golang.org/x/perf/cmd/benchstat).

```
./bench_against.sh
```

### Against ninja

This runs both ninja's performance tests and nin's equivalent for an
apple-to-apple comparison.

```
./compare_against_ninja.sh
```

## Disassembling

I wrote a small tool to post process `go tool objdump` output:

```
go install github.com/maruel/pat/cmd/...
```

Disassemble a single function:

```
disfunc -f 'nin.CanonicalizePath$' -pkg ./cmd/nin | less -R
```

List all bound checks:

```
boundcheck -pkg ./cmd/nin | less -R
```

List all symbols in the binary and their size:

```
go tool nm -size nin  | grep github.com/maruel/nin
```

See the compiler optimization process in a web page:

```
GOSSAFUNC=nin.CanonicalizePath go build ./cmd/nin && open ssa.html
```

Escape analysis:

```
go build -gcflags="-m" .
```
