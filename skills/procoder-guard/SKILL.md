---
name: procoder-guard
description: Install procoder as a pre-commit hook and a CI check so the gate holds without the agent. Use when the user says "procoder guard", "add procoder to CI", "pre-commit hook", "enforce procoder in CI", "enforce procoder without the agent", or invokes /procoder-guard. Writes files, but only after naming them and getting a yes.
---

# procoder-guard

Rules the agent enforces are a habit. Rules CI enforces are a guarantee. This
installs the second.

## Procedure

1. **Detect what already exists.** Never add a second mechanism beside a working one.

   | Look for | If present |
   |---|---|
   | `.pre-commit-config.yaml` | add a `local` repo hook entry; do not write `.git/hooks/pre-commit` |
   | `.husky/` | append the procoder line to `.husky/pre-commit` |
   | `lefthook.yml` / `lefthook.yaml` | add a `pre-commit.commands.procoder` entry |
   | `.git/hooks/pre-commit` (bare, no manager) | append to it; never overwrite it |
   | none of the above | write `.git/hooks/pre-commit` from the template, `chmod +x` |
   | `.github/workflows/` | write `.github/workflows/procoder.yml` |
   | `.gitlab-ci.yml`, `.circleci/`, `azure-pipelines.yml` | translate the CI template's two steps into that provider's syntax |

2. **Check the starting position, before writing anything.**
   Run `node <plugin>/bin/procoder.js check .`. If no `.procoder-baseline.json`
   exists and the count exceeds ~50, say so plainly and offer
   `node <plugin>/bin/procoder.js baseline .` first — a guard that fails on its
   first run gets removed the same day. Ask before writing the baseline.

3. **Name every file, then wait.** List each file you will create or modify, one
   line each, with create-or-modify marked. Ask for confirmation and do not
   write until you get a yes. This step is not optional and not skippable, no
   matter how routine the change looks.

   ```
   create   .git/hooks/pre-commit            procoder check on staged files
   create   .github/workflows/procoder.yml   check + verify ratchet
   modify   .gitignore                       ignore nothing new (no change needed)
   ```

4. **Write from the templates.** `scripts/templates/pre-commit.sh` and
   `scripts/templates/procoder-ci.yml`. Adjust only the invocation, to match how
   procoder is reachable in that repo:

   | Availability | Invocation |
   |---|---|
   | global install | `procoder` |
   | dev dependency | `npx --no-install procoder` |
   | plugin only | `node <plugin>/bin/procoder.js` — set `PROCODER_BIN` |

   The CI ratchet step must stay `procoder verify` — it compares fingerprints.
   Never replace it with a count comparison; that lets a new violation ride in
   behind an unrelated fix.

5. **Verify.** Run the pre-commit script once by hand against the current index
   and report what it did.

## Output

- The files written, one line each.
- The bypass command: `git commit --no-verify`.
- One line on updating the baseline: `procoder baseline <paths>`.

## Do not

- Do not write any file before naming it and getting a yes.
- Do not add a second hook manager to a repo that already has one.
- Do not overwrite an existing `pre-commit` hook — append to it.
- Do not enable the guard on a repo whose baseline you have not recorded.
- Do not replace `procoder verify` with a count comparison, in any provider.
