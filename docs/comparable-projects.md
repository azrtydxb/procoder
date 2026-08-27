# Comparable projects

**An explanation.** Other people are solving adjacent problems. This page
names them, states what each targets, and says plainly how procoder's
approach differs. It is descriptive, not competitive.

It is deliberately separate from [Influences](influences.md), which records
what procoder _took_ from other tools. Some projects appear on both pages,
and where they do, this page is not the place the debt is acknowledged —
that page is.

## What this page can and cannot argue

There is a tempting argument that several unrelated teams building the same
defences is itself evidence the problem is real. It is a good argument, and
it does not apply to most of what is listed here.

Procoder read [unlazy](https://github.com/Leonxlnx/unlazy),
[addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) and
[mattpocock/skills](https://github.com/mattpocock/skills) in a research
sweep on 2026-08-26, and shipped mechanisms from all three within days.
Those are borrowings. Calling them convergence would be a claim this
repository's own issue bodies contradict, on a page whose whole purpose is
to be checkable.

The narrower claim does hold, and it is worth stating. Procoder's commit
gate landed on 2026-08-18 and its evidence-gated closes on 2026-08-19 —
both before that sweep, in a project whose first commit was 2026-08-16.
unlazy independently ships acceptance ledgers with runnable gates and
required evidence. Two projects reaching "a claim of completion is not
evidence of completion, so make the evidence runnable" without contact is a
real signal about the failure mode, and it is the one convergence claim on
this page that survives its own check.

## The projects

### unlazy

**Targets:** model laziness, underthinking, and premature completion
claims, at the level of a single task's acceptance ledger.

**Overlaps:** runnable verification gates with recorded evidence; a Stop
hook that blocks a turn ending on incomplete work; parallel-dispatch
honesty; ownership leases for concurrent agents.

**Diverges:** unlazy is task-scoped and ledger-centric — the unit is one
piece of work and its gates. Procoder is repository-scoped: the same
honesty applied to a commit, a sprint, a release, and ten domain checks
that run whether or not any task is open. unlazy also executes approved
checks on re-verification; procoder's binary executes nothing it was not
explicitly asked to, and modifies no file outside its own cache.

**Also:** its `research/validation-protocol.md` retracts an earlier
internal benchmark rather than quietly dropping it, which is the discipline
[Research](research.md) is held to.

### addyosmani/agent-skills

**Targets:** the full engineering lifecycle as 24 skills, plus specialist
personas and reference checklists, installable across 70+ agents.

**Overlaps:** per-skill rationalization tables against agents talking
themselves out of steps; progressive disclosure to keep entry-point files
small; multi-angle review as separate personas.

**Diverges:** it advises and procoder computes. A skill telling an agent to
run the tests and a binary refusing to close the task when they did not run
are different kinds of object. It also ships an honest `docs/comparison.md`
naming its own alternatives, which is the model this page follows.

### mattpocock/skills

**Targets:** disciplined workflows — TDD, code review, debugging, domain
modelling — as model- and user-invoked skills.

**Overlaps:** its `wizard` skill scripts a walkthrough for steps only a
human can perform; its `handoff` compacts work for the next agent, which
procoder keeps as `.procoder/state/handoff.md`.

**Diverges:** same advice-versus-computation line as above.

### anthropics/skills and the Agent Skills spec

**Targets:** the shared format itself, rather than a competing opinion.

**Relationship:** procoder conforms to it. `skills/procoder/SKILL.md` is a
spec-compliant skill and `plugin.json` a spec-compliant plugin. This is the
foundation both this page and [Influences](influences.md) are written on
top of, and naming it is more useful than leaving it implicit.

## Related reading

- [Influences](influences.md) — what procoder took, and from whom
- [Where procoder sits](positioning.md) — the layer, and what it is not
- [Research](research.md) — external evidence for the premises
- [Honest limits](honest-limits.md) — where the rigor stops paying
