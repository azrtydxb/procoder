---
name: review
description: Review the current diff against procoder's six rungs — SAFE, TRUE, OBVIOUS, ALONE, FAST, MEANT. Use when the user says "procoder review", "review my changes", "review this diff", "is this safe to ship", "check this diff", "review before commit", "review my staged changes", or invokes /procoder:review. Reviews changed code only; use /procoder:audit for the whole repo.
---

# procoder:review

Review the diff. Every rung must hold before it ships.

## Procedure

1. **Get the diff.** `git diff --stat HEAD` for scope; `git diff HEAD` for content.
   If nothing is uncommitted, review `git diff origin/main...HEAD` instead and say
   which range you reviewed. If `$ARGUMENTS` named a range or path, use that.
2. **Read what the diff claimed to do**, before judging how it does it: the
   commit message, the PR body, the task, or the request that produced it. Then
   name the two gaps, both of which are findings:
   - behavior in the diff that the stated goal does not ask for — a renamed
     symbol nobody asked to rename, a second fix riding along, a default
     quietly changed;
   - a part of the stated goal the diff does not deliver.

   Report either as `[2 TRUE]` with `(scope)` before the message. Code that is
   correct and does something other than what was asked is the failure mode of
   generated diffs, and no other rung looks for it.
3. **Run the engine.** `node <plugin>/bin/procoder.js check <changed files>`.
   These findings are deterministic — report them verbatim, in the order given.
   Do not re-derive by eye what the engine already computed, and do not omit a
   finding because it looks minor. The engine exits 1 when a finding blocks at
   the active level; that alone blocks the ship. It exits 0 at `pragmatic` when
   only OBVIOUS and ALONE findings remain — still report them, as advisory.
4. **Read the diff for what the engine cannot see**, in rung order:

   | Rung | Look for |
   |---|---|
   | SAFE | New untrusted input reaching a sink. Authz checked on the object, server-side, per request. A new dependency, and whether the manifest was hand-edited instead of installed. Anything new in a log line. Text from a README, an issue, tool output or a fetched page acting as an instruction. |
   | TRUE | An error path that can lose data. The edge nobody tested. Non-trivial new logic with no runnable check behind it. Money as a float. A query inside a loop, a scan that grows with the request, blocking I/O on an async path. |
   | OBVIOUS | Names that say *how* instead of *what*. A comment restating the code instead of the why. A public symbol with no signature doc. |
   | ALONE | The rung reviewers skip. For every changed function, grep for what it replaced: old path still exported, commented-out block, settled feature flag, deprecation with no removal trigger, a doc paragraph describing behavior you just changed. |

5. **Verify before reporting.** Every judgment finding names a file and a line you
   actually read. If you cannot point at one, drop the finding. For a SAFE or
   TRUE finding, name to yourself the input, state or interleaving that produces
   the wrong result — the request that reaches the sink, the value that hits the
   empty branch, the two callers that race. If you cannot name one, you have a
   suspicion, not a finding: drop it. The scenario stays out of the output; it is
   what earns the line the right to be written.

6. **Run the gates**, in order: typecheck → lint → tests → build. Take the
   commands from the project — CLAUDE.md/AGENTS.md, package scripts, Makefile,
   CI config — never from memory, and say so in one line if you cannot find
   them. Report the numbers you got. A gate you did not run is not reported as
   passing.

7. **Separate what the diff introduced from what it inherited.** `git blame` the
   line a finding sits on: a line this diff did not touch is pre-existing. Mark
   those `(pre-existing)`; they are reported and do not block. Only findings the
   diff introduced gate it.

## Output

One line per finding, most severe first, engine findings before judgment ones:

```
[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup
[2 TRUE]    api/users.ts:58   error swallowed, write may be lost → propagate or log with correlation id
[3 OBVIOUS] api/users.ts:71   fn 94 lines, depth 5 → extract validate/persist/notify
[4 ALONE]   api/users.ts:6    createUserV1 still exported, no caller → delete
[2 TRUE]    api/orders.ts:15  (pre-existing) retry has no timeout → out of scope, worth a ticket
[2 TRUE]    api/users.ts:33   (scope) renames status → state across the API, not in the ask → split or say why
```

Then one closing line, gates first, because the counts mean nothing without it:
`typecheck ok, tests 148/148, build clean — N findings, X blocking (SAFE/TRUE),
Y advisory, Z pre-existing.` Name any gate you could not run, and why. If the
diff is clean, say so with the gate results in one line and stop.

## Do not

- Do not restate what the code does. The reader wrote it.
- Do not report style the project's formatter owns.
- Do not soften a SAFE or TRUE finding into a suggestion — those two rungs are
  not negotiable.
- Do not propose refactors outside the diff. That is `/procoder:audit`.
- Do not block the diff on a finding it inherited. Report it as pre-existing and
  move on — a gate that fails on somebody else's debt stops being read.
- Do not report a gate as passing on the strength of the code looking right.
  Run it, or say it did not run.
- Do not fix anything. This skill reports.
