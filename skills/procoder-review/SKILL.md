---
name: procoder-review
description: Review the current diff against procoder's four rungs — SAFE, TRUE, OBVIOUS, ALONE. Use when the user says "procoder review", "review my changes", "review this diff", "is this safe to ship", "check this diff", "review before commit", "review my staged changes", or invokes /procoder-review. Reviews changed code only; use procoder-audit for the whole repo.
---

# procoder-review

Review the diff. Every rung must hold before it ships.

## Procedure

1. **Get the diff.** `git diff --stat HEAD` for scope; `git diff HEAD` for content.
   If nothing is uncommitted, review `git diff origin/main...HEAD` instead and say
   which range you reviewed. If `$ARGUMENTS` named a range or path, use that.
2. **Run the engine first.** `node <plugin>/bin/procoder.js check <changed files>`.
   These findings are deterministic — report them verbatim, in the order given.
   Do not re-derive by eye what the engine already computed, and do not omit a
   finding because it looks minor. The engine's exit code is 1 when it found
   anything; that alone blocks the ship.
3. **Read the diff for what the engine cannot see**, in rung order:

   | Rung | Look for |
   |---|---|
   | SAFE | New untrusted input reaching a sink. Authz checked on the object, server-side, per request. A new dependency. Anything new in a log line. |
   | TRUE | An error path that can lose data. The edge nobody tested. Non-trivial new logic with no runnable check behind it. Money as a float. |
   | OBVIOUS | Names that say *how* instead of *what*. A comment restating the code instead of the why. A public symbol with no signature doc. |
   | ALONE | The rung reviewers skip. For every changed function, grep for what it replaced: old path still exported, commented-out block, settled feature flag, deprecation with no removal trigger, a doc paragraph describing behavior you just changed. |

4. **Verify before reporting.** Every judgment finding names a file and a line you
   actually read. If you cannot point at one, drop the finding.

## Output

One line per finding, most severe first, engine findings before judgment ones:

```
[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup
[2 TRUE]    api/users.ts:58   error swallowed, write may be lost → propagate or log with correlation id
[3 OBVIOUS] api/users.ts:71   fn 94 lines, depth 5 → extract validate/persist/notify
[4 ALONE]   api/users.ts:6    createUserV1 still exported, no caller → delete
```

Then one closing line: `N findings — X blocking (SAFE/TRUE), Y advisory.` If the
diff is clean, say so in one line and stop.

## Do not

- Do not restate what the code does. The reader wrote it.
- Do not report style the project's formatter owns.
- Do not soften a SAFE or TRUE finding into a suggestion — those two rungs are
  not negotiable.
- Do not propose refactors outside the diff. That is `/procoder-audit`.
- Do not fix anything. This skill reports.
