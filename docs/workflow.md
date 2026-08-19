# The workflow

How work moves from an idea to a merged commit under procoder's
discipline. The skills (`/procoder:pr`, `/procoder:merge`) drive it; the
rules live in `.procoder/github/WORKFLOW.md` and are the repo's to edit.

## Start in a worktree

Feature work happens in a git worktree — one per branch or agent — so the
default-branch checkout stays clean and parallel efforts never collide.

## Before the PR

`procoder git` and `procoder check` must be clean: formatting, hygiene,
lint, secrets, docs, CI and infra rules all answer at once. The pr skill
then summarises the real diff, fills the PR template from
`.procoder/github/`, scrubs attribution, verifies the blast radius the
gate reported (`procoder index impact`), and shows everything before
`gh pr create`. Titles stay ≤ 72 characters — they become squash-commit
subjects.

## While checks run

Merge-gate polling is delegated to a watch-only background agent following
the watcher protocol: calibrate against previous runs of the same
workflow, poll per job in the foreground, report the first failure
immediately with its log excerpt, poll dynamically, report every state
change. The watcher never fixes, replies, or merges — the main agent acts
on its reports.

## Reviews

Every review thread — human or bot, Copilot included — is either fixed
(commit, push, reply saying what changed) or answered with a concrete
reason. Threads are resolved only after, never to hide them. Nothing
merges over a red check or an unaddressed comment.

## Merge and clean up

Squash-merge when everything answers green, then: delete the remote
branch, delete the local branch, remove the worktree, `git fetch
--prune`, and return to an updated default branch. Nothing dangles.
