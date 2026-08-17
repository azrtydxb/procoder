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
4. **Read the baseline.** If `.procoder-baseline.json` exists, every entry in
   it names the rule it silenced, the file it sits in, and the date it was
   accepted. Report the count, and run
   `node <plugin>/bin/procoder.js verify --aging 90 .` for the entries that have
   been accepted longest — those are the ledger's oldest lines, and the only
   part of it nobody wrote a trigger for. An entry dated `unknown` predates
   dated baselines; the next `procoder baseline` stamps it.

   Accepted debt belongs in the same table as every marker below: a baseline
   entry is a suppression that happens to live in a generated file.

## Output

One table, oldest first:

| age | file:line | marker | trigger | status |
|---|---|---|---|---|
| 14 months | api/users.ts:42 | HACK: skip authz in dev | — | undated |  <!-- procoder: literal alone/orphan-todo the doctrine names this pattern, it is not an instance of it -->
| 3 months | queue/worker.go:88 | procoder: single lock | throughput > 1k/s | dated |
| 8 months | legacy/auth.ts | baseline: safe/weak-hash | — | undated (baseline) |

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
