---
description: "Onboard an existing codebase: every domain's checks over the whole tree, then a triaged plan to bring it in line."
---

The user invoked /procoder:audit.

The command below is the `procoder` binary on PATH.

This is the onboarding sweep for a repository that was not built under
procoder. Nothing is modified by the audit itself — you triage, plan, and
fix with the user.

1. Run `procoder doctor`; if tools are missing, run `procoder init`
   first — an audit full of NOT-checked lines wastes everyone's time.
2. Run `procoder index build`, then `procoder audit`. Read the whole
   scorecard.
3. Triage in this order, and present the plan to the user BEFORE fixing:
   1. **Secrets** — every one needs removal AND rotation; check whether it
      is a false positive first (a pinned SHA, a fixture) and use
      `gitleaks:allow` or `.gitleaksignore` for those, with a reason.
   2. **Other blocking** — unformatted files (apply `procoder format`
      output), conflict markers, junk, template drift, failing terraform.
   3. **Judged info findings** — lint, complexity, docs gaps: fix what is
      real in priority order, record reasons for what you leave.
4. Fix incrementally — one theme per commit, gate-clean after each — and
   re-run `procoder audit` until the scorecard says the repository
   would pass the gate.
5. Finish by writing the repo's `.procoder/` files (`procoder
templates` prints defaults) so the standard holds from here on.
