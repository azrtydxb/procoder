# Decisions waiting on a human

Written by the agent, read by `procoder ask`. One `## ` heading per
decision; the lines under it are its options.

## Does the decisions queue and its principles change ship in v3.1.1, or wait?

Context: v3.1.1 was scoped to #172, #175, #177 and #181. This work is none
of them — it came out of a correction during the sprint.

- in v3.1.1: it is small, tested, and the principles half is worth having
  in agents' hands sooner rather than later. ADR 0003 governs what makes a
  change major, and 2.0.1 already shipped new enforcement in a patch.
- hold for v3.2.0: keeps v3.1.1 exactly the four issues that were scoped,
  which is easier to describe in a changelog and to review.

## Which of #177 and #181 is next, given #172 and #175 are both in review?

- #181 (`procoder prune`): already specified in the issue with guardrails
  agreed, so it can start immediately.
- #177: not yet read this session. Reading it first may change the answer.

## Does `procoder prune` delete, or print what it would delete?

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

## How many cached plugin versions does `procoder prune` keep?

- 3 (active + 2 previous): reclaims 1.01 GB here, two rollback targets.
- 2 (active + 1 previous): reclaims 1.05 GB, one rollback target — the
  minimum that still counts as a rollback.
- 5 (active + 4 previous): reclaims 0.92 GB, a longer rollback window.

## How is v3.1.1 landed, and does procoder prune --apply run on this machine?

- Landing: merge #184 (#175) and #187 (#172, #181, decisions queue) once CI
  is green, then tag v3.1.1. #177 already landed via PR #178.
- The sweep: whether to reclaim the 1.03 GB here, which is irreversible.

## Is v3.1.1 tagged once the PRs are merged?

Standing instruction from the maintainer on 2026-08-26: no.

- merge only: land #184 and #187 on main and stop there. The tag is the
  maintainer's call and waits for them to say so.
- tag when merged: NOT what was asked. Recorded so a later session cannot
  read "merge both, then cut v3.1.1" from earlier in the day and act on it.
