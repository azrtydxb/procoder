---
description: "The performance discipline: measure before touching, benchmark what matters, prove regressions and fixes with numbers."
---

The user invoked /procoder:perf with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

Performance work without numbers is storytelling. The discipline:

1. **Baseline first.** Run `procoder bench` — it runs the Go
   benchmarks and compares against `.procoder/bench/baseline.txt`. No
   baseline yet? `procoder bench --save` records one BEFORE you
   touch anything. No benchmarks yet? Write one for the hot path the
   change touches — that is the first task, not an afterthought.
2. **Change, then measure again.** `procoder bench` marks any
   regression beyond `[bench] threshold` (default 10%) and exits 1.
   Improvements are numbers too — quote the delta, not adjectives.
3. **Save deliberately.** A new baseline (`--save`) is a decision that
   the current numbers are the new normal — take it after a reviewed
   improvement, never to silence a regression.
4. Results are single-run and machine-local; treat small deltas as
   noise and cross-platform comparisons with the warning the tool
   prints. For deeper hunts, profile (pprof) before optimizing.
