# store: the lockfile with stale detection and sorted ordering

Status: open
Created: 2026-08-28

## Description

Plan task 1 of .procoder/plans/service-state-seam.md.

There is no locking anywhere in internal/ — the only mutex in the tree is
in a test. dispatch.json, claims.json and the ask ledger are all
read-modify-write, so two writers lose one of the two updates. That is
rare today only because every hook is its own short-lived process; the
daemon in #117 makes it ordinary.

go.mod has no require block, so golang.org/x/sys and therefore the
portable flock/LockFileEx pair are unavailable. The lock is an O_EXCL
lockfile under .procoder/state/locks/, named by the first 16 hex
characters of the SHA-256 of the repo-relative slash path plus .lock,
carrying the pid and the unix time on two lines.

Done means Lock(root, relPaths...) takes every named path in sorted
repo-relative order, breaks locks it can prove are dead, refuses rather
than blocks when one is live, and reports what it broke.

## Acceptance criteria

- [ ] `TestLockIsExclusive` — a second `Lock` on a held path returns an error rather than succeeding.
- [ ] `TestStaleLockIsBrokenAndReported` — a lock older than 30s is broken, the write proceeds, and the returned `broken` slice names the path.
- [ ] `TestUnreadableLockIsStale` — a lock whose contents do not parse, and one whose timestamp is in the future, are both broken.
- [ ] `TestLiveLockRefusesRatherThanBlocks` — a live lock held past 5s causes a refusal naming the file, and `Lock` returns inside 8s rather than blocking.
- [ ] `TestLockOrderIsSortedPaths` — two goroutines requesting the same two paths in opposite order, 50 times each, neither deadlocks within 30s.
- [ ] A failed acquisition in a multi-path call releases the locks already taken; no partial hold survives the error.
- [ ] `procoder check` is clean.

## Evidence

