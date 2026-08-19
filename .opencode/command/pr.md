---
description: "Prepare and open a pull request the senior way: gate, template, scrubbed, everything visible."
---

The user invoked /procoder:pr with arguments: $ARGUMENTS

Every procoder command below is the `procoder` binary on PATH.

0. Read .procoder/github/WORKFLOW.md and follow it — the repo's rules win
   over this skill's defaults. If it is missing, get the default via
   `procoder templates`, write it, then follow it. Default rule: feature
   work happens in a git worktree (one per branch/agent), never dirtying the
   default-branch checkout — if this work isn't in one yet, note it and keep
   the branch clean.

1. Run `procoder git` and `procoder check`. Fix everything BLOCKING
   before going further — a PR that fails its own gate wastes the reviewer.
   The gate's `info` impact lines are the change's blast radius: open each
   referencing file it names and verify the change breaks nothing there
   (or run `procoder index impact` for the full list).
2. Read the real diff (`git diff <default-branch>...HEAD`) and the commits.
   Summarise what actually changed, not what you remember intending.

2b. The pre-PR self-review — the first fresh pair of eyes is OURS, not the
downstream reviewer's:

- Read .procoder/github/REVIEW.md (missing → `procoder templates`
  prints the default; write it first).
- Dispatch a FRESH-context reviewer subagent — not yourself; the
  author's context hides the author's blind spots — with: the rubric
  verbatim, the branch diff, and the instruction to report findings as
  file:line, what breaks, and the fix, ending with a severity-counted
  verdict or exactly "Nothing found — open the PR."
- Fix every Critical/Important finding (commit them), re-run
  `procoder check`, and only then continue. Downstream bot reviews
  are the fallback net — anything they catch later becomes a lesson
  (see /procoder:merge's reflection step).

3. Fill .procoder/github/PULL_REQUEST_TEMPLATE.md section by section from that
   diff. If the template is missing, get it via `procoder templates`,
   write it, then fill it.
4. Write the drafted title and body to a temp file and run
   `procoder scrub <file>`. It must say clean — no Co-Authored-By, no
   "generated with", nothing presenting the work as an AI's. Fix and re-scrub
   until clean.
5. Keep the title at 72 characters or fewer — it becomes the squash
   commit's subject and our own gate flags longer ones.
6. Show the user the final title and body BEFORE creating anything.
7. Create the PR with `gh pr create --title ... --body-file ...`. If gh is
   missing, run `procoder init` first.
8. Hand off to /procoder:merge for the gates-and-reviews phase, or offer to
   run it now.
