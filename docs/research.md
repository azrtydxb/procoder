# Research

**An explanation.** Each procoder domain exists because of a belief about
how software work goes wrong. This page states those beliefs and, where
external evidence exists, cites it. Where it does not, this page says so
rather than implying a backing that is not there.

That rule matters more than the citations. A page of references is easy to
write and worth nothing if any of them is misapplied, so every entry below
is one of two kinds: **cited**, meaning a real source was read and says
what is claimed, or **uncited**, meaning the premise is procoder's own
design judgment and is labelled as such.

## Correction policy

If a citation here turns out to be misapplied, misquoted, or to not say
what was claimed, it gets corrected **in place, with the correction
visible** — struck through or marked, never silently deleted. A page that
quietly drops its mistakes teaches its reader nothing about how carefully
the rest was checked.

No corrections have been issued yet. This section is here so the first one
has an established place to go, not because the page is assumed correct.

---

## Cited

### Agents claim completion they have not reached

**Premise behind:** the quality controllers, evidence-gated closes,
`procoder test`'s "NOT run is never green".

**Evidence:** _SlopCodeBench: Benchmarking How Coding Agents Degrade Over
Long-Horizon Iterative Tasks_ — Orlanski, Roy, Yun, Shin, Gu, Ge, Adila,
Roberts, Sala, Albarghouthi
([arXiv:2603.24755](https://arxiv.org/abs/2603.24755), submitted March
2026, revised May 2026). Across 36 problems, 196 checkpoints and 15 coding
agents, the best checkpoint pass rate was **14.8%**, and **no agent solved
any problem end to end**. Structural erosion increased in 77% of
trajectories and verbosity in 75.5%; agent code measured 2.3× more verbose
and 2.0× more eroded than 473 open-source Python repositories.

**How it is used here:** as evidence that iterative agent work degrades
measurably, which is the premise behind refusing to accept a claim of
completion in place of evidence of it. Note the version caveat: earlier and
later revisions of this paper report different problem, checkpoint and
percentage figures. The numbers above are from the revised abstract.

### Prose guidance shifts the starting point, not the drift

**Premise behind:** procoder shipping a _binary_ rather than only an
`AGENTS.md`. See [Where procoder sits](positioning.md).

**Evidence:** the same paper. Explicit quality guidance reduced initial
verbosity and erosion **by up to one third — without affecting the rate of
degradation**.

**How it is used here:** it supports the split procoder is built on, and it
cuts both ways honestly. Guidance demonstrably helps; it demonstrably does
not stop the drift. That is an argument for keeping both halves, not for
dismissing the advisory one.

### Developers do admit debt in comments, and much of it stays

**Premise behind:** `procoder debt` harvesting `debt:` markers, and
flagging a marker with no revisit condition as rot.

**Evidence:** _An Exploratory Study on Self-Admitted Technical Debt_ —
Potdar and Shihab
([Semantic Scholar](https://www.semanticscholar.org/paper/An-Exploratory-Study-on-Self-Admitted-Technical-Potdar-Shihab/ddf82ba67e252428fde55fcf9b73e848924a3372)),
studying Eclipse, Chromium OS, Apache HTTP Server and ArgoUML. Self-admitted
technical debt appeared in up to **31% of files**, and between **26.3% and
63.5%** of it was removed after introduction.

**How it is used here:** the practice procoder relies on — engineers
writing down the corner they cut — is empirically real and widespread. The
removal range is the more useful half: on the low end, roughly three
quarters of admitted debt was still there. A comment alone does not get
debt paid, which is why procoder harvests markers into a ledger and treats
one without a revisit condition as rot rather than as a record.

---

## Uncited — procoder's own judgment

These premises have no external citation here. Some may have supporting
literature nobody has looked for yet; one has literature that does _not_
settle the question. Either way, no claim of research backing is made.

### Multi-lens review catches more than a single pass

**Premise behind:** `procoder review`'s five lenses.

**Status: contested, deliberately not claimed as support.** The nearest
literature compares checklist-based against ad hoc code reading, and it
does not agree with itself. A controlled study in a distributed groupware
environment found **no significant difference** in defect detection
effectiveness, effort, or false positives between the two
([arXiv:0909.4260](https://arxiv.org/abs/0909.4260)). Other experiments
report checklist-based reading as substantially superior. Reviews of this
literature describe the results as inconsistent and conflicting.

Citing only the favourable half would be exactly the failure this page
exists to avoid. The five lenses are a design judgment, and the honest
statement is that the external evidence is mixed.

### Formatting, hygiene and secret drift is costly enough to gate on

**Premise behind:** the commit gate itself.

**Status: uncited.** No study has been read that measures the cost of this
specific drift in agent-written code. The gate is justified here by
mechanism rather than measurement: an unformatted file, a committed secret
and a junk artifact are each objectively identifiable, and identifying them
at the commit is cheaper than at review.

### Specification before implementation reduces rework

**Premise behind:** the spec → plan → backlog → sprint chain.

**Status: uncited.** There is a substantial requirements-engineering
literature on defect cost escalating with the phase in which a defect is
found, and it has not been read and verified for this page. Until it has,
the chain stands on procoder's own reasoning.

---

## What procoder has not measured about itself

Nothing on this page is evidence that _procoder_ works. No benchmark of the
gate's overhead against defects caught has been run — see
[Honest limits](honest-limits.md), and
[#190](https://github.com/azrtydxb/procoder/issues/190), which exists to
make that measurable rather than assumed.

## Related reading

- [Comparable projects](comparable-projects.md) — others solving adjacent problems
- [Influences](influences.md) — what procoder took, and from whom
- [Honest limits](honest-limits.md) — where the rigor stops paying
