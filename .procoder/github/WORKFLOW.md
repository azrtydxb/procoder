# Workflow rules

Repo-level rules the procoder skills read and follow. Edit freely — what is
written here wins over the skills' built-in defaults.

## Worktrees

Feature work happens in a git worktree (one per branch/agent) so the default
branch checkout stays clean and parallel agents never collide. Use the
harness's native worktree support when available, `git worktree add` otherwise.

## Merge watching

While a PR's checks and reviews run, delegate the waiting to a background
agent that only watches and reports. The main agent keeps working and acts on
the report: every fix, every reply, and the merge itself stay with the main
agent — watchers gather information, the agent acts.

## After a successful merge

Clean up: delete the remote branch (merge with `--delete-branch`), delete the
local branch, remove the worktree if one was used, run `git fetch --prune`,
and return to an updated default branch.

