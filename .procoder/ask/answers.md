# What a human decided

Written 2026-08-27 16:28 UTC. procoder reads this
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

## [decision] decisions.md

Key: 612a37231e9b
Question: Label #209 and #211 roadmap alongside #194?

All three are documentation-positioning work: an evidence bibliography, a
comparable-projects doc, and the docs-hardening issue already labelled
roadmap. They are one cluster and none is small.

- label both roadmap: the cluster reads as direction rather than queued
  work, which is what the label was created for.
- leave them unlabelled: they stay in the ordinary queue.

Answer: yes, label both

## (no longer asked)

Key: 6617364d8670
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## [decision] decisions.md

Key: 815c92a8de33
Question: Close #206, whose premise does not hold for procoder?

#206 asks for ReDoS defence — bounded, sandboxed regex evaluation — because
procoder "evaluates regex in several places that ultimately read
repo-controlled or config-controlled text".

Checked rather than taken on trust, the way #201's premise was:

- there is no `regexp.Compile` anywhere in the tree;
- the only two `MustCompile` calls with a non-literal argument build their
  pattern from constants in source (`gitx.aiIdentities`,
  `security.pyDeps`);
- `.procoder/lint/RULES.md`'s `checks` list is passed to clang-tidy as its
  `--checks` value. It never reaches a Go regex engine.

So no repository- or config-controlled text becomes a pattern procoder
compiles. The exposure the issue describes is not there.

- close it, with the evidence recorded, and reopen if a runtime-compiled
  pattern is ever introduced.
- keep it open as a standing constraint on future work — a reminder not to
  add one.

Answer: not yet — do a deep analysis first, and close only if it is 100% certain

## (no longer asked)

Key: a3f73c3bc2de
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## (no longer asked)

Key: aa8ea1f17c0c
Question: Do the four large features stay open as a roadmap?

Answer: keep open, but label them roadmap/large so they stop reading as queued work

## [decision] decisions.md

Key: b2c54f852d82
Question: Remove the cached 3.1.0 plugin too, or keep it as the rollback?

**Decided: keep 3.1.0.** The rollback is worth 45 MB; prune's
active-plus-one-previous policy stands as written.

`procoder prune --apply` removed 3.0.0 and reclaimed 45 MB. It kept 3.1.0
deliberately: the policy is the active version plus one previous, so a
release that misbehaves has somewhere to fall back to. Removing 3.1.0 is
outside what prune will do — it would be a manual `rm -rf` of
`~/.claude/plugins/cache/procoder/procoder/3.1.0`.

The trade is a fixed ~45 MB against the one-step rollback. Note that 3.1.0
is now three releases behind and predates `release.maintainers`, so as a
rollback target it would reintroduce the false positive that blocked the
3.3.0 release commit.

- keep 3.1.0: prune's own policy, and a rollback that costs 45 MB.
- remove 3.1.0: reclaims the space; rollback then means reinstalling from
  the marketplace rather than a local directory.

Answer: Run the per-file scans concurrently. It attacks process startup directly, scans exactly what it scans today, and does not depend on the whole-tree figure I got wrong.

## (no longer asked)

Key: b55269facb93
Question: Rescope #198 and #191, and merge #200 with #201 and #204 with #208?

Answer: yes — rescope and merge now, before anyone starts on a duplicate

## (no longer asked)

Key: bdef8e3588e5
Question: Does `procoder prune` delete, or print what it would delete?

Answer: report by default, delete on --apply — the safe thing stays the default and the reclaim still happens in one command

## (no longer asked)

Key: c9c26deda0a7
Question: Which is the next piece of work?

Answer: #193, merge-conflict discipline

## [decision] decisions.md

Key: e721d4e97e3f
Question: Do #210 now, before #193?

`docs/influences.md` credits superpowers, ponytail and serena. BMad appears
in it zero times, while `internal/planning/bmad.go` has shipped since 2.0.0
and `[planning] method = "bmad"` reads BMad's own artifacts. Verified: the
integration is real and the doc gap is real.

Small, factual, and the trademark constraint is already understood — BMad
is named to describe interoperation, never as a procoder feature name.

- do it now: ten minutes, and it closes a verified gap in a provenance
  record that is currently wrong by omission.
- after #193: #193 was already chosen as the next piece of work.

Answer: no — #193 as planned

## (no longer asked)

Key: f4733ae6d75a
Question: How many cached plugin versions does `procoder prune` keep?

Answer: 2 — active + 1 previous
