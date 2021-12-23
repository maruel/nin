# Ginja

An experimental fork of ninja translated in Go. Currently a toy.

## Are you serious?

Yeah.

## Marc-Antoine, isn't it a stupid idea?

Yeah.

When Google was created, Altavista was king. When Facebook was created, Myspace
was hot. When ginja was created, there were other options.

The reason it's possible at all is because ninja is well written and has
a reasonable amount of unit tests.

## Why?

- The parser can be used as a library
- Refactoring Go >> refactoring C++
- Making the code concurrent will be trivial
- Plans to have it be stateful and/or do RPCs and change fundamental parts
- Go has native benchmarking
- Go has native CPU and memory profiling
- Go has native code coverage
- Go has native documentation service
- Since it's GC, and the program runs as a one shot, we can just disable GC and
  save a significant amount of memory management overhead.

## The name sucks

Yeah. Please send suggestions my way!

## ninja

Ninja is a small build system with a focus on speed.
https://ninja-build.org/

See [the manual](https://ninja-build.org/manual.html) or
`doc/manual.asciidoc` included in the distribution for background
and more details.

## Getting ginja

```
go install github.com/maruel/ginja@latest
```
