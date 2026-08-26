# What a human decided

Written 2026-08-26 08:06 UTC. procoder reads this
file to avoid asking a question twice; edit an answer here to change what
it believes. Reword the question and it will be asked again.

## (no longer asked)

Key: 32f242effa43
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## (no longer asked)

Key: 5517b4921f0f
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## [decision] decisions.md

Key: 6617364d8670
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

- #181 (`procoder prune`): already specified in the issue with guardrails
  agreed, so it can start immediately.
- #177: not yet read this session. Reading it first may change the answer.

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## [decision] decisions.md

Key: a3f73c3bc2de
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Context: v3.1.1 was scoped to #172, #175, #177 and #181. This work is none
of them — it came out of a correction during the sprint.

- in v3.1.1: it is small, tested, and the principles half is worth having
  in agents' hands sooner rather than later. ADR 0003 governs what makes a
  change major, and 2.0.1 already shipped new enforcement in a patch.
- hold for v3.2.0: keeps v3.1.1 exactly the four issues that were scoped,
  which is easier to describe in a changelog and to review.

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## [decision] decisions.md

Key: bdef8e3588e5
Question: Does `procoder prune` delete, or print what it would delete?

P-CONTROL says the binary prints and never modifies files. That rule is
about repository content, and the plugin cache is not that — but a command
that removes 1.1 GB irreversibly is the largest exception the tool would
have, and it is worth being deliberate rather than assuming the rule does
not reach here.

- deletes, behind an explicit flag: `procoder prune` reports what it would
  remove and exits; `procoder prune --apply` does it. One command, the
  reclaim actually happens, and the default is still a report.
- prints only, strictly: procoder emits the exact `rm -rf` lines and the
  human or agent runs them. P-CONTROL holds without exception, nothing is
  ever deleted by procoder, and the user sees precisely what will go.
- deletes by default with `--dry-run` to preview: shortest path to the
  reclaim, and the one most likely to surprise somebody.

Answer: report by default, delete on --apply — the safe thing stays the default and the reclaim still happens in one command

## [decision] decisions.md

Key: f4733ae6d75a
Question: How many cached plugin versions does `procoder prune` keep?

- 3 (active + 2 previous): reclaims 1.01 GB here, two rollback targets.
- 2 (active + 1 previous): reclaims 1.05 GB, one rollback target — the
  minimum that still counts as a rollback.
- 5 (active + 4 previous): reclaims 0.92 GB, a longer rollback window.

Answer: 2 — active + 1 previous
