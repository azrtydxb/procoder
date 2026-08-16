---
name: debt
description: List every deliberate shortcut in the repo and whether it has a removal trigger. Use when the user says "procoder debt", "what shortcuts did we take", "technical debt ledger", "outstanding procoder markers", "what did we defer", or invokes /procoder:debt. Reports the ledger; it does not fix anything.
---

# procoder:debt

A deliberate shortcut is fine. An undated one is rot. This is the ledger.

## Procedure

1. **Find the markers.**
   - Deliberate: `git grep -n "procoder:"`
   - Informal: `git grep -nE "TODO|FIXME|HACK|XXX|@deprecated|@Deprecated|Obsolete\("`  <!-- procoder: literal alone/orphan-todo, alone/deprecated-no-trigger the doctrine names this pattern, it is not an instance of it -->
2. **Age and author each one.**
   `git log -1 --format="%ar %an" -S "<marker text>" -- <file>`
   Fall back to `git blame -L <line>,<line> -- <file>` when `-S` finds nothing.
3. **Classify each marker.**

   | Status | Test |
   |---|---|
   | dated | has a removal trigger — a version, a date, or a measurable condition |
   | undated | no removal trigger; itself a `[4 ALONE]` violation |
   | overdue | the trigger has passed — version shipped, date gone, condition met |

   Check an overdue trigger against reality: `git tag --sort=-creatordate | head`
   for versions, today's date for dates.
4. **Read the baseline.** If `.procoder-baseline.json` exists, report its
   fingerprint count as accepted debt. It is part of the ledger.

## Output

One table, oldest first:

| age | file:line | marker | trigger | status |
|---|---|---|---|---|
| 14 months | api/users.ts:42 | HACK: skip authz in dev | — | undated |  <!-- procoder: literal alone/orphan-todo the doctrine names this pattern, it is not an instance of it -->
| 3 months | queue/worker.go:88 | procoder: single lock | throughput > 1k/s | dated |

Then one line per undated and overdue marker, standard format:

```
[4 ALONE]   api/users.ts:42   HACK with no removal trigger, 14 months old → add a trigger or remove the shortcut  <!-- procoder: literal alone/orphan-todo the doctrine names this pattern, it is not an instance of it -->
```

Close with: `N markers: X dated, Y undated, Z overdue. Baseline holds W accepted findings.`

## Do not

- Do not propose fixing everything. The ledger's job is visibility.
- Do not treat a dated marker as a problem — it is doing its job.
- Do not compute a debt number, rating, or letter. Report counts and ages only.
- Do not edit or remove a marker. This skill reports.
