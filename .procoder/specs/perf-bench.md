# perf-bench

Status: draft

## Problem

The perf skill demands numbers — measure before touching, prove
regressions with benchmarks — but ships no way to produce, keep, or
compare them. There is no baseline to stand on, so the discipline
depends entirely on the agent improvising the same commands twice and
eyeballing the difference.

## Users

- The agent proving a change did not regress the hot path.
- Pascal reviewing a perf claim: the numbers and the delta, not prose.

## In scope

- `procoder bench` — run the repository's Go benchmarks
  (`go test -bench . -benchmem -run ^$` over packages that contain
  Benchmark functions), print the results, and compare against the
  stored baseline when one exists: per benchmark, ns/op and B/op with
  percentage delta, regressions beyond the threshold marked loudly.
- `procoder bench --save` — additionally write the run to
  `.procoder/bench/baseline.txt` (procoder-owned state, committed) so
  future runs compare against it. Saving is explicit — a baseline is a
  decision.
- `[bench] threshold = 10` (percent, default 10) in config.toml.
- Go only in v1, said in every output where it matters: other
  ecosystems answer "NOT run — bench covers Go in this version" so the
  scope is never silently narrower than it looks.
- Comparison is name-matched: new benchmarks are listed as new,
  vanished ones as gone — both informational.

## Out of scope

- Statistical rigor (multiple runs, benchstat-grade variance) — the
  output says results are single-run and machine-local.
- Enforcement: regressions are reported, marked, and exit 1 so CI can
  choose to care; nothing blocks the gate or closes.
- Non-Go ecosystems, profiling (pprof), flamegraphs.
- Cross-machine baseline normalization; the baseline records GOOS/
  GOARCH and warns when they differ from the current run.

## Constraints

- Pure Go stdlib; package internal/bench.
- P-CONTROL boundary: the only write is the baseline file, and only
  under --save (an explicit ask, procoder-owned state).
- Honesty: no benchmarks found is said plainly; a failed bench run is
  NOT run, never an empty pass; baseline from a different GOOS/GOARCH
  is compared WITH a visible warning.
- Timeout 10 minutes with the hung-tool message.

## Interfaces

- `procoder bench [--save]`; exit 0 clean/informational, 1 when any
  regression beyond threshold (or the run failed), 2 usage.
- config: `[bench] threshold = <percent>`.
- Usage text, docs.Commands, docs site; the perf skill
  (commands/perf.md) is rewritten to drive `procoder bench` instead of
  improvised commands; OpenCode twin follows.

## Data

- `.procoder/bench/baseline.txt`: the raw `go test -bench` output
  prefixed with a header line recording date, commit, GOOS/GOARCH.
  Committed with the repo; the file is the whole story, no database.

## Edge cases

- No Benchmark functions anywhere → "no benchmarks in this repository
  — the perf skill starts by writing one", exit 0.
- Benchmark renamed → old name reported gone, new name reported new;
  no false regression.
- Baseline saved on darwin/arm64, run on linux/amd64 → deltas printed
  with the cross-platform warning line.
- A benchmark that fails to build/run → NOT run with the compiler or
  panic excerpt, exit 1.
- Threshold 0 → every slowdown marks; negative threshold → exit 2.

## Failure modes

- go toolchain missing → NOT run naming it (a Go-less machine cannot
  bench Go).
- Baseline unreadable/corrupt → comparison skipped with the reason,
  current results still printed, and --save offered as the fix.

## Acceptance criteria

- [ ] On a fixture with one benchmark, `bench --save` writes the
      baseline with the header; a second run reports ~0% delta and
      exits 0.
- [ ] Slowing the benchmarked code (fixture with a tunable loop) makes
      `bench` mark the regression and exit 1 at the default threshold.
- [ ] A repo with no benchmarks answers the no-benchmarks line, exit 0.
- [ ] Baseline parsing and delta math have unit tests over recorded
      `go test -bench` output, including the renamed-benchmark case.
- [ ] The perf skill instructs measuring via `procoder bench` and its
      OpenCode twin matches.

## Open questions

<!-- none — decisions recorded above -->
