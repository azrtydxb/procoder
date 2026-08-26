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
