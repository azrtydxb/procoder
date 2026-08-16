---
name: procoder-audit
description: Audit a whole repository against procoder's four rungs — SAFE, TRUE, OBVIOUS, ALONE. Use when the user says "procoder audit", "audit this codebase", "audit the whole repo", "audit the entire codebase", "where is this codebase weakest", "how bad is this repo", or invokes /procoder-audit. Whole-repo sweep; use procoder-review for a diff.
---

# procoder-audit

The engine sweeps; you rank. A repo has too many findings to list — the job is
to say which three mistakes are being repeated.

## Procedure

1. **Sweep with the engine.** `node <plugin>/bin/procoder.js check .` (or the path
   in `$ARGUMENTS`). These findings are deterministic and already reviewed.
   Report them as-is. Never re-derive them by reading files, and never drop one
   because it looks minor.
2. **Group and count.** By rung, then by directory. A rung's worst directory is
   the one with the most findings, not the most files.
3. **Sample-read the three worst files** for what the engine cannot compute:
   rung-3 naming quality, rung-4 semantics (is this symbol genuinely
   unreachable?), rung-1 trust-boundary reasoning. Name a line for each judgment
   finding or drop it.
4. **Rank.** Order findings SAFE → TRUE → OBVIOUS → ALONE, then by count of the
   same rule. Cap the individual list at the **top 15**.
5. **Name the top three systemic patterns.** One repeated mistake matters more
   than thirty instances of it. For each, give the single change that fixes the
   whole class.

## Output

First a rung summary table:

| Rung | Count | Worst directory |
|---|---|---|
| 1 SAFE | 12 | `api/` |
| 2 TRUE | 31 | `workers/` |
| 3 OBVIOUS | 84 | `legacy/` |
| 4 ALONE | 22 | `legacy/` |

Then the ranked top 15, one line per finding:

```
[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup
[4 ALONE]   legacy/old.ts:6   createUserV1 still exported, no caller → delete
```

Then the three systemic patterns, one line each: pattern → the one change that
fixes the class → how many findings it clears.

Close with: `N findings total — showing top 15. X blocking (SAFE/TRUE).`

## The adoption path

If the total exceeds 50, say plainly that a repo this size cannot be fixed in one
pass, and offer the ratchet:

```
node <plugin>/bin/procoder.js baseline .
```

Every current finding is recorded as accepted; new code is gated from now on; the
count may shrink but never grow, enforced in CI by
`node <plugin>/bin/procoder.js verify .`.

**Ask before writing the baseline file.** It is a commitment the user makes, not
one you make for them.

## Do not

- Do not list all 4,000 findings. The cap is 15 individual lines plus the table.
- Do not fix anything during an audit. This skill reports.
- Do not write the baseline without asking first.
- Do not re-derive engine findings by eye, or restate what the code does.
- Do not report style the project's formatter owns.
