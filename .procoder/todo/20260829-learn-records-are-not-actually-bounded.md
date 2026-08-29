# learn: the record file is not actually bounded

Status: open
Created: 2026-08-29

## Description

`internal/learn` shipped a `maxRecords = 5000` constant in 3baaac6 with a
comment saying it "bounds the file so an old repository does not carry an
unbounded one. Oldest dropped." Nothing ever referenced it. Rotation was
specified and never wired, so `.procoder/state/learn.jsonl` grows without
limit on any repository that turns `[learn] record = true` on.

The dead constant was deleted in the state-seam branch (#117 phase 0)
rather than left contradicting the code around it — a comment that claims
a bound the code does not enforce is worse than no comment. Deleting it
does not fix the gap; it stops the tree lying about the gap.

This predates the seam and is not the seam's to fix: that branch is about
where reads and writes go, not about what `learn` retains. But the seam is
now the natural place to fix it, because `store.AppendLearn` already holds
the file's lock, which is exactly what a safe trim needs.

Done means the file has a bound the code enforces, the bound is stated in
`docs/configuration.md` beside the other `[learn]` keys, and appending
stays O(1) in the common case — a trim on every append would put the
length of a repository's history on the cost of every command.

## Acceptance criteria

- [ ] A test appends more records than the bound and asserts the file
      holds at most the bound afterwards, oldest dropped.
- [ ] A test asserts the common-case append does no full read of the
      file — appending to a large file costs the same as appending to a
      small one.
- [ ] `procoder learn measure` reports the same ranking before and after a
      trim that drops only records outside the window.
- [ ] `docs/configuration.md` states the bound and what happens at it,
      beside `[learn] record` and `[learn] min_samples`.
- [ ] `procoder check` is clean.

## Evidence

