---
description: "Finish a PR properly: every check green, every review addressed — human and bot — then merge and clean up."
---

The user invoked /procoder:merge with arguments: $ARGUMENTS
(The argument is the PR number or branch; with none, use the PR for the
current branch: `gh pr view --json number`.)

First read .procoder/github/WORKFLOW.md and follow it — the repo's rules win
over the defaults below. If it is missing, get the default via
"${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" templates, write it, then follow it.

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

1. Checks:  `gh pr checks <pr>` — every required check must pass. For a
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
3. Re-run steps 1-2 after every push until: all checks green, reviewDecision
   is APPROVED (or no reviews are required), and no unaddressed comments
   remain.
4. Only then merge: `gh pr merge <pr> --squash --delete-branch` (or the
   repo's configured strategy). The merge commit message follows the commit
   template and carries no AI-attribution line — run `launcher.sh scrub` on
   it if you wrote one.
5. Clean up after the confirmed merge (--delete-branch removed the remote):
   delete the local branch, remove the worktree if the work lived in one
   (`git worktree remove <path>`), run `git fetch --prune`, and return to an
   updated default branch (`git checkout <default> && git pull`).

Never merge over a red check. Never merge with an unaddressed review comment.
If a required gate cannot be satisfied, say exactly which one and why, and
stop — the user decides.
