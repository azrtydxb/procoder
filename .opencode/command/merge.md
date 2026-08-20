---
description: "Finish a PR properly: every check green, every review addressed — human and bot — then merge and clean up."
---

The user invoked /procoder:merge with arguments: $ARGUMENTS
(The argument is the PR number or branch; with none, use the PR for the
current branch: `gh pr view --json number`.)

First read .procoder/github/WORKFLOW.md and follow it — the repo's rules win
over the defaults below. If it is missing, get the default via
procoder templates, write it, then follow it.

A PR merges when EVERYTHING answers — never before.

0. Don't block on the waiting: spawn a BACKGROUND agent whose only job is to
   watch and report. The watcher never fixes, never replies, never merges —
   it gathers information; you act on it. Continue other work while it
   watches, and pick up the loop below each time it reports. Give the
   watcher the merge-watching protocol from .procoder/github/WORKFLOW.md
   verbatim: calibrate against previous runs of the same workflow, poll PER
   JOB in the foreground (`gh run view <run-id> --json jobs` — never a
   fire-and-forget monitor), report the FIRST failing job immediately with
   its `--log-failed` excerpt instead of waiting for the rest, poll
   dynamically (fast early and near the calibrated finish, never slower
   than 90s), and report on every state change.

Loop until done:

1. Checks: `gh pr checks <pr>` — every required check must pass. For a
   failing check, open its run (`gh run view <id> --log-failed`), fix the
   cause in the code, commit, push, and wait for the re-run.
2. Reviews: `gh pr view <pr> --json reviews,reviewDecision` and the review
   comments via `gh api repos/{owner}/{repo}/pulls/<pr>/comments`. Treat BOT
   reviewers (Copilot included) exactly like humans: read every comment and
   recommendation, and for each one either
   - fix it (commit, push, then reply to the thread saying what changed), or
   - reply with a concrete reason why not.
     No comment is skipped silently. Resolve threads only after fixing or
     answering (`gh api graphql` resolveReviewThread), never to hide them.

   How to receive review feedback — bot or human:
   - VERIFY before implementing: check each claim against the actual code.
     Reviewers (Copilot especially) lack full context; a finding can be
     factually wrong for this codebase. Wrong → push back in the thread
     with the technical reason, not a fix.
   - If ANY comment in a review is unclear, ask about it before
     implementing the others — comments are often related, and partial
     understanding produces the wrong fix.
   - Fix one finding at a time and re-run the covering tests after each.
   - Replies state facts: "Fixed in <sha> — <what changed>." No "great
     point", no thanks, no "you're absolutely right" — gratitude is not a
     verification, and performative agreement erodes the reviewer's trust
     in real agreements.
   - A reviewer asking for an unused "proper" feature gets the YAGNI
     answer: show it is uncalled and propose removal instead.

2b. The reflection step — MANDATORY whenever step 2 found anything real
(bot or human). An escaped finding is a bug in our gates; fixing only
the finding leaves its class open:

- For each real finding, name the layer that should have caught it
  BEFORE the PR existed: a linter (lint baseline/config), the pre-PR
  rubric (.procoder/github/REVIEW.md), a quality controller, a pinning
  test, or CI itself.
- ADAPT that layer now, in this same PR: enable the linter rule, add
  the rubric line, tighten the controller, write the pinning test.
- Record each as an entry in .procoder/github/LESSONS.md (shape:
  `procoder templates`), with the adaptation named. Then run
  `procoder lessons` — it flags any entry left unlearned, and an
  unlearned lesson is not done.
- False positives get no ledger entry — reflect only on real escapes.
- Then run `procoder copilot-leak`. Copilot's auto-review sometimes
  opens ISSUES rather than PR comments, and those escape the reading in
  step 2 entirely: the finding gets fixed and nothing records that the
  class happened. The command finds them, strips every trace of the
  code from them, and asks before it files or records anything. This is
  the only place a procoder command talks to GitHub about reviews — the
  gate reads the resulting ledger offline and never queries.

3. Re-run steps 1-2 after every push until: all checks green, reviewDecision
   is APPROVED (or no reviews are required), and no unaddressed comments
   remain.
4. Only then merge: `gh pr merge <pr> --squash` (or the
   repo's configured strategy). The merge commit message follows the commit
   template and carries no AI-attribution line — run `procoder scrub` on
   it if you wrote one.
5. Clean up after the confirmed merge: delete the remote branch with
   `git push origin --delete <branch>` (the merge flag's local step fails
   when the default branch is checked out in a worktree, silently skipping
   the remote delete — do it explicitly), delete the local branch, remove
   the worktree if the work lived in one
   (`git worktree remove <path>`), run `git fetch --prune`, and return to an
   updated default branch (`git checkout <default> && git pull`).

Never merge over a red check. Never merge with an unaddressed review comment.
If a required gate cannot be satisfied, say exactly which one and why, and
stop — the user decides.
