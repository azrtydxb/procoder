# bench misreports an empty run as NOT run

Status: done 2026-08-19
Created: 2026-08-19
Epic: -
Sprint: -
Type: bug
Severity: s3

## Description

**Reproduction**: run `procoder bench` in this repository before any
benchmark existed.

**Observed**: `NOT run — go test -bench produced no benchmark rows:
PASS`, exit 1.

**Expected**: `no benchmarks in this repository — the perf skill starts
by writing one`, exit 0.

**Cause**: `hasBenchmarks` is a `git grep` for `func Benchmark`, and
internal/bench/bench_test.go carries a fixture module in a string
literal that contains exactly that text. The grep says benchmarks
exist, the run finds none, and the zero-rows branch reports a
successful run as a failed one. Found by using the tool during a
`/procoder:perf` pass.

<!-- Reproduction steps: the shortest path from a clean state to the
     failure — commands, inputs, environment. -->

<!-- Observed vs expected: what actually happens, and what should
     happen instead. -->

## Acceptance criteria

<!-- Each criterion is testable. The first is non-negotiable — a bug is
     only fixed when a test would catch its return. -->

- [x] a regression test pins the fix: red before the change, green after

## Evidence

- TestFixtureStringDoesNotFakeABenchmark builds a fixture whose only
  test file MENTIONS `func BenchmarkAdd` in a string, exactly like the
  file that caused this, and asserts exit 0 with the no-benchmarks
  line. It fails against the old branch and passes against the new one.
- The run itself is now the authority: a successful `go test -bench`
  with zero rows means there are no benchmarks, because a grep cannot
  tell a benchmark from the words for one.
- Live: `procoder bench` on this repository now answers
  "no benchmarks in this repository — the perf skill starts by writing
  one", exit 0 — and after the benchmarks were written, it reports both
  and compares against the saved baseline.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
