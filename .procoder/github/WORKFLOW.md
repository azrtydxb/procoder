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

The watcher follows this protocol, not naive fixed-interval polling:

- **Calibrate first.** Read the last few completed runs of the same workflow
  (`gh run list --workflow=<file> --limit 5 --json conclusion,createdAt,updatedAt`)
  to learn how long each job normally takes and which steps have failed
  recently. That sets the polling budget and where to look first.
- **Poll per job, in the foreground.** `gh run view <run-id> --json jobs`
  reports each job the moment it concludes — do not wait for the whole run.
  Poll inline in a loop; never arm a fire-and-forget monitor and go idle:
  a watcher that is not actively polling is not watching.
- **Fail fast, report fast.** The moment ANY job concludes `failure`, pull
  its failing step (`gh run view <run-id> --log-failed`) and report
  immediately with the log excerpt — do not sit on a known failure while
  other jobs finish.
- **Poll dynamically.** Short intervals (15–30s) during the early phase where
  installs and setup fail fast, and again near the calibrated finish time;
  longer intervals in the quiet middle. Never slower than 90s while anything
  is pending.
- **Report on change.** Any check flipping state, any new review comment, and
  the final all-concluded summary each warrant a report — silence is only for
  "nothing changed".

## After a successful merge

Clean up: delete the remote branch (merge with `--delete-branch`), delete the
local branch, remove the worktree if one was used, run `git fetch --prune`,
and return to an updated default branch.

