---
description: "The security pass: secrets (blocking), SAST, dependency vulns — plus the index's entry points to review from."
---

The user invoked /procoder:security with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

First read .procoder/security/RULES.md and follow it — the repo's rules win.
If it is missing, get the default via `procoder templates`, write it, then
continue.

1. Run `procoder security --deep`. Three instruments answer:
   - **secrets** (gitleaks) — every finding BLOCKS; remove the secret AND
     tell the user to rotate the credential (removal alone is not enough —
     assume it leaked the moment it was written).
   - **SAST** (semgrep, community rules) — ERROR severity blocks; judge
     WARNING/INFO honestly: fix what is real, explain what is not.
   - **dependency vulns** (osv-scanner) — high/critical blocks; upgrade the
     package or explain why not.
2. Rank what you review first with the index: `procoder index entrypoints`
   is where data enters; `procoder index callers <symbol>` traces how
   input reaches a finding (`procoder index graph` for the full walk).
3. NOT-checked lines mean a scanner is missing — run `procoder init`.
   A security check that silently didn't run is worse than a red one.
4. Fix, then re-run `procoder security --deep` until no blocking findings
   remain, and show the user the final report with your judgments.

Never print a secret's value — name the rule, the file, the line, and the
rotation duty.
