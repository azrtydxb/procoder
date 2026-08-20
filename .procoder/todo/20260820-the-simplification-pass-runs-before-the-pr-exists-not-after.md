# the simplification pass runs before the PR exists, not after it is green

Status: closed 2026-08-20
Created: 2026-08-20

## Description

/procoder:pr already dispatches a fresh-context reviewer against
REVIEW.md before the PR is opened, but that rubric hunts correctness:
error paths, traversal, output paths, guards that do not guard. Nothing
in the flow asks the other question — should this code exist at all —
until somebody thinks to run /procoder:simplify by hand.

On PR#75 that ran after the PR was open, reviewed and green. Two of its
findings were in the branch's own new code (a title derived twice, a
comment claiming a shared list that was not shared), so fixing them cost
two extra commits and a fourth CI run on a pull request that was already
finished. Every one of those minutes was avoidable: the same reading,
one step earlier, and the PR opens clean.

The pass belongs in /procoder:pr step 2b, over the branch diff, beside
the review it already runs. The repo-wide sweep is a different job on a
different clock — it answers about code this change never touched, and
attaching it to every PR would bury the diff's own findings under a
backlog of pre-existing ones. That belongs to /procoder:release, before
a tag.

Done means a PR cannot be opened without the diff having been read for
code that should not exist, and the two scopes are documented as the
different jobs they are.

## Acceptance criteria

- [x] commands/pr.md step 2b instructs the simplification pass over the
      branch diff before the PR is opened, with its OpenCode and Kilo
      twins regenerated through the generation rule and the twin-parity
      tests green.
- [x] The pass is diff-scoped there, and commands/simplify.md says
      plainly that the repo-wide sweep is a separate cadence, not a
      per-PR step.
- [x] commands/release.md carries the repo-wide sweep before a tag, so
      the whole-tree question has an owner and a clock.
- [x] The findings shape stays P-CONTROL: the pass lists cuts and the
      agent decides, in the PR flow exactly as in the skill today —
      verified by reading the instruction back, not by a new binary
      command.
- [x] A pre-PR run on a deliberately padded branch (a wrapper with one
      caller added to a fixture) surfaces that finding before the PR is
      opened, and the same branch with the wrapper removed reports the
      null result rather than inventing one.

## Evidence

- commands/pr.md step 2b dispatches the second lens in the same pass as the
  correctness rubric, quoting the five tags, the one-line finding format and
  the exact null string the way the rubric is already quoted. Its own
  pre-PR review caught that dispatching the lens by NAME only would come
  back as prose with no replacements — the earlier trials only worked
  because the format was pasted in by hand.
- commands/simplify.md states the routing in one line; commands/release.md
  runs the repo sweep at step 2, before the first fix. All six OpenCode and
  Kilo twins regenerated through the generation rule —
  TestOpenCodeCommandParity and TestKiloCommandParity green (both caught a
  hand-rolled regeneration during this work).
- P-CONTROL preserved: the step reads "decide each cut — taking it or saying
  why not", and no binary command was added; the pass remains an instruction
  to an agent that lists cuts for a human to judge.
- Trial 1, padded diff (an interface with one implementation, a constructor
  for an empty struct, a single-use helper): all three surfaced with
  replacements and `net: -24 lines possible.`
- Trial 2, the same fixture with the padding removed but a speculative
  empty-string guard left in: one finding, `delete: speculative
empty-message placeholder — nothing sets Message to ""`. Defensible
  against that fixture, not an invention.
- Trial 3, a diff with genuinely nothing to cut (a one-word comment typo
  fix): exactly `Lean already. Ship.` — the null path returns the null
  string rather than inventing a finding.
- This branch's own pre-PR run produced 3 Important + 2 Minor correctness
  findings and 13 lines of cuts, every one of them in the instruction text
  this task added. Fixed in 8f63a41 and d4074da before the PR was opened,
  which is the behaviour the task exists to produce.
- docs/workflow.md sections 7 and 11 and docs/quality-chain.md now describe
  both lenses and the release sweep — a docs escape the pre-PR review
  caught against the flow's own mandatory step 2a.
