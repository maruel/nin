# nin

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/nin.svg)](https://pkg.go.dev/github.com/maruel/nin)
[![codecov](https://codecov.io/gh/maruel/nin/branch/main/graph/badge.svg?token=KAO6K039PJ)](https://codecov.io/gh/maruel/nin)

An experimental fork of ninja translated in Go. Currently a toy.

## Are you serious?

Yeah.

## Marc-Antoine, isn't it a stupid idea?

Yeah.

When Google was created, Altavista was king. When Facebook was created, Myspace
was hot. When nin was created, there were other options.

The reason it's possible at all is because ninja is well written and has
a reasonable amount of unit tests.

## Why?

- The parser can be used as a library
  - This opens the door to a lot of opportunity and a real ecosystem
- Refactoring Go >> refactoring C++
  - As I made progress, I saw opportunities for simplification
- Making the code concurrent (e.g. making the parser parallel) will be trivial
- Plans to have it be stateful and/or do RPCs and change fundamental parts
  - E.g. call directly the RBE backend instead of shelling out reclient?
- It's easier to keep the performance promise in check, and keep it maintainable
  - Go has native benchmarking
  - Go has native CPU and memory profiling
  - Go has native code coverage
  - Go has native documentation service
- Since it's GC, and the program runs as a one shot, we can just disable GC and
  save a significant amount of memory management (read: CPU) overhead.

I'll write a better roadmap if the project doesn't crash and burn.

Some people did advent of code 2021, I did a brain teaser instead.

### Concerns

- Go's slice and string bound checking may slow things down, I'll have to micro
  optimize a bit.
- Go's function calls are more expensive and the C++ compiler inlines less often
  than the C++ compiler. I'll reduce the number of function calls.
- Go's Windows layer is likely slower than raw C++, so I'll probably call raw
  syscall functions on Windows.
- Staying up to date changes done upstream, especially to the file format and
  correctness checks.

## ninja

Ninja is a small build system with a focus on speed.
https://ninja-build.org/

See [the manual](https://ninja-build.org/manual.html) or
`doc/manual.asciidoc` included in the distribution for background
and more details.

## Getting nin

```
go install github.com/maruel/nin/cmd/nin@latest
```

## Profiling

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

First build ninja with:

```
python3 ./configure.py --bootstrap
```

If you have re2c installed and it's not exactly 1.3, src/depfile_parser.cc
and src/lexer.cc will be modified. Revert back with:

```
git checkout HEAD -- src/depfile_parser.cc src/lexer.cc
```

Then see the difference between these two commands:

```
rm -rf build ninja_test; time nin ninja_test
rm -rf build ninja_test; time ninja ninja_test
```
