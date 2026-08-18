---
description: "CI/CD hygiene and run health: pinned actions, timeouts, concurrency, tests — plus the latest runs via gh."
---

The user invoked /procoder:ci.

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

1. Run `launcher.sh ci` — the workflow-hygiene report:
   - **mutable action refs**: a tag can be silently repointed by its owner
     (supply-chain door). Pin the commit SHA: resolve it with
     `gh api repos/<owner>/<repo>/git/ref/tags/<tag> --jq .object.sha`
     (dereference once more if type is "tag") and keep the tag as a trailing
     comment for readability. Blocking when the repo sets
     `[ci] pin_actions_policy = "block"`.
   - **missing timeout-minutes**: a hung job holds the runner for GitHub's
     six-hour default; add a sensible ceiling per job.
   - **no concurrency cancel**: stacked pushes run CI on stale commits; add
     a concurrency group with cancel-in-progress.
   - **no tests in any workflow**: continuous testing is the CT.
2. Then the run health: `gh run list --limit 5` for recent outcomes, and
   `gh run view <id> --log-failed` for anything red — diagnose the cause,
   never just re-run over a real failure.
3. Fix, re-run `launcher.sh ci`, and show the user the clean report.
