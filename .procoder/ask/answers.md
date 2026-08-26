# What a human decided

Written 2026-08-26 13:00 UTC. procoder reads this
file to avoid asking a question twice; edit an answer here to change what
it believes. Reword the question and it will be asked again.

## (no longer asked)

Key: 0489b93052fa
Question: Is v3.1.1 tagged once the PRs are merged?

Answer: merge only — do NOT tag. The maintainer will say when.

## (no longer asked)

Key: 32f242effa43
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## (no longer asked)

Key: 3e59374380b3
Question: Scope of v3.1.1: does it wait for #186 and #188?

Answer: yes — do #186 and #188 too. The maintainer tags, not me.

## (no longer asked)

Key: 5517b4921f0f
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## (no longer asked)

Key: 6617364d8670
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## (no longer asked)

Key: a3f73c3bc2de
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## [decision] decisions.md

Key: aa8ea1f17c0c
Question: Do the four large features stay open as a roadmap?

#189 SKILL.md redesign, #190 `procoder learn`, #192 `procoder wizard`, #194
docs hardening. Each is a release of its own; two add new commands.

- keep open: a roadmap that says what procoder might become.
- close until wanted: an open issue nobody is going to start reads as a
  commitment, and twenty-three of them read as a plan.

Answer: keep open, but label them roadmap/large so they stop reading as queued work

## [decision] decisions.md

Key: b55269facb93
Question: Rescope #198 and #191, and merge #200 with #201 and #204 with #208?

Today's release overtook two of them, and two pairs are the same idea filed
twice. Left alone, somebody rebuilds what already exists.

- rescope and merge now: #198 loses its unfalsifiable-criteria half (shipped
  as `CriteriaWithoutFalsifiers`) and keeps fixed-output, hedgy vocabulary
  and unmeasured thresholds; #191 keeps board visibility and shared
  blockers and drops "there is no decisions file", because there is one now.
- leave them: the overlap is discoverable by whoever picks one up, at the
  cost of them finding out after starting.

Answer: yes — rescope and merge now, before anyone starts on a duplicate

## (no longer asked)

Key: bdef8e3588e5
Question: Does `procoder prune` delete, or print what it would delete?

Answer: report by default, delete on --apply — the safe thing stays the default and the reclaim still happens in one command

## [decision] decisions.md

Key: c9c26deda0a7
Question: Which is the next piece of work?

- #193, merge-conflict discipline: the failure happened here today — git
  split a conflict through a function, "keep both sides" truncated a test,
  and only the compiler caught it. Concrete evidence, and the fix is prose.
- #201 + #200, the execute path: verified, not assumed —
  `internal/runcmd/runcmd.go:172` execs argv the repository declares, so an
  agent writing a launch command under injection is a live path. The only
  security-shaped items in the set.
- #195, the context.md glossary: small, self-contained, pays offevery session.

Answer: #193, merge-conflict discipline

## (no longer asked)

Key: f4733ae6d75a
Question: How many cached plugin versions does `procoder prune` keep?

Answer: 2 — active + 1 previous
