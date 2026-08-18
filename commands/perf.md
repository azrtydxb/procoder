---
description: "The performance discipline: measure before touching, benchmark what matters, prove regressions and fixes with numbers."
---

The user invoked /procoder:perf with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

Performance work has one law: **measure first**. Never optimize from
intuition; never claim a win without a number from before and after.

1. **Find the instruments this repository already has:**
   - Go: `go test -bench . -benchmem ./...` where `Benchmark*` functions
     exist (`launcher.sh index search Benchmark` finds them);
     `go test -cpuprofile`/`-memprofile` + `go tool pprof -top` for
     hotspots; `-benchtime` and `-count` for stability.
   - Python: `python -m cProfile -s cumtime`, `py-spy top --pid` for live
     processes, `pytest-benchmark` where it is set up.
   - JS/TS: `node --cpu-prof`, `clinic doctor` where installed.
2. **Baseline, change, re-measure.** Run the relevant benchmark before the
   change, keep the output, run it after, and report the delta with the
   command used. `-count=5` and comparing distributions beats single runs;
   for Go, `benchstat` when available.
3. **Hotspots before micro-optimizations:** a profile naming the top
   functions beats guessing; use `launcher.sh index callers <fn>` to see
   who drives a hot function before changing its contract.
4. **If the repository has no benchmarks** and the user's task is
   performance-shaped, write the smallest benchmark that captures the
   claim first — a fix without a benchmark is a hope, not a fix.
5. Report: the numbers before, the numbers after, the command that
   produced them, and what you did NOT optimize because measurement said
   it didn't matter.
