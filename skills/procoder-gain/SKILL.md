---
name: procoder-gain
description: Report what measurably improved since a reference point — baseline shrinkage, lines deleted, rot removed, boundaries hardened. Use when the user says "procoder gain", "what did procoder fix", "how much did we clean up", "quality progress", "what improved this release", or invokes /procoder-gain. Every number comes from git or the baseline.
---

# procoder-gain

The ratchet only counts if someone reads it. This is the readout.

## Procedure

1. **Pick the reference point.** Previous tag: `git describe --tags --abbrev=0 HEAD^`.
   No tags: `git rev-list -1 --before="30 days ago" HEAD`. State which you used.
2. **Baseline shrinkage.** Count fingerprints then and now:
   `git show <ref>:.procoder-baseline.json` versus the working copy. Report both
   counts and the difference. If the file did not exist at `<ref>`, say so.
3. **Net lines.** `git diff --stat <ref>..HEAD` — added versus deleted.
4. **Rot removals.** `git log <ref>..HEAD --oneline` plus
   `git log <ref>..HEAD -p --diff-filter=D --name-only` for whole files removed.
   Count commits that deleted exports, settled feature flags, or deprecated
   paths — verify by reading the diff, not the commit message.
5. **Boundaries hardened.** `git diff <ref>..HEAD -U0 | grep '^+'` for added
   validation, authz checks, and parameterized queries at entry points. Count
   only ones you can point at a line for.

## Output

Four numbers, each with the command that produced it:

```
reference       v1.4.0 (git describe --tags --abbrev=0 HEAD^)
baseline        312 → 287, 25 fewer accepted findings
lines           +1,840 / -3,102 (net 1,262 deleted)
rot removed     7 commits deleted dead exports or settled flags
boundaries      4 entry points gained validation
```

Then the three most valuable individual changes, one line each, with the commit
SHA and what it removed or hardened.

If a number cannot be derived, print `—` and say why in the same line. Never
substitute an estimate.

## Do not

- Do not report a number you cannot derive from git or the baseline.
- Do not turn this into a rating, a letter, or a percentage of "quality".
- Do not claim credit for changes procoder did not influence — report what
  changed, not who caused it.
- Do not count a commit twice across categories; pick the one it best fits.
