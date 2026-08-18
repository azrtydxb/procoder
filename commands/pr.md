---
description: "Prepare and open a pull request the senior way: gate, template, scrubbed, everything visible."
---

The user invoked /procoder:pr with arguments: $ARGUMENTS

The launcher for every procoder command below is:
"${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

0. Read .procoder/github/WORKFLOW.md and follow it — the repo's rules win
   over this skill's defaults. If it is missing, get the default via
   `launcher.sh templates`, write it, then follow it. Default rule: feature
   work happens in a git worktree (one per branch/agent), never dirtying the
   default-branch checkout — if this work isn't in one yet, note it and keep
   the branch clean.

1. Run `launcher.sh git` and `launcher.sh check`. Fix everything BLOCKING
   before going further — a PR that fails its own gate wastes the reviewer.
   The gate's `info` impact lines are the change's blast radius: open each
   referencing file it names and verify the change breaks nothing there
   (or run `launcher.sh index impact` for the full list).
2. Read the real diff (`git diff <default-branch>...HEAD`) and the commits.
   Summarise what actually changed, not what you remember intending.
3. Fill .procoder/github/PULL_REQUEST_TEMPLATE.md section by section from that
   diff. If the template is missing, get it via `launcher.sh templates`,
   write it, then fill it.
4. Write the drafted title and body to a temp file and run
   `launcher.sh scrub <file>`. It must say clean — no Co-Authored-By, no
   "generated with", nothing presenting the work as an AI's. Fix and re-scrub
   until clean.
5. Show the user the final title and body BEFORE creating anything.
6. Create the PR with `gh pr create --title ... --body-file ...`. If gh is
   missing, run `launcher.sh init` first.
7. Hand off to /procoder:merge for the gates-and-reviews phase, or offer to
   run it now.
