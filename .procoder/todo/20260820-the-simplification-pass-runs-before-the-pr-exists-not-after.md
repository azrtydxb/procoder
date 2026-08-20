# the simplification pass runs before the PR exists, not after it is green

Status: open
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

- [ ] commands/pr.md step 2b instructs the simplification pass over the
      branch diff before the PR is opened, with its OpenCode and Kilo
      twins regenerated through the generation rule and the twin-parity
      tests green.
- [ ] The pass is diff-scoped there, and commands/simplify.md says
      plainly that the repo-wide sweep is a separate cadence, not a
      per-PR step.
- [ ] commands/release.md carries the repo-wide sweep before a tag, so
      the whole-tree question has an owner and a clock.
- [ ] The findings shape stays P-CONTROL: the pass lists cuts and the
      agent decides, in the PR flow exactly as in the skill today —
      verified by reading the instruction back, not by a new binary
      command.
- [ ] A pre-PR run on a deliberately padded branch (a wrapper with one
      caller added to a fixture) surfaces that finding before the PR is
      opened, and the same branch with the wrapper removed reports the
      null result rather than inventing one.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the task open. -->
