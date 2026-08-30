# store: the lockfile with stale detection and sorted ordering

Status: closed 2026-08-28
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

- [x] `TestLockIsExclusive` — a second `Lock` on a held path returns an error rather than succeeding.
- [x] `TestStaleLockIsBrokenAndReported` — a lock older than 30s is broken, the write proceeds, and the returned `broken` slice names the path.
- [x] `TestUnreadableLockIsStale` — a lock whose contents do not parse, and one whose timestamp is in the future, are both broken.
- [x] `TestLiveLockRefusesRatherThanBlocks` — a live lock held past 5s causes a refusal naming the file, and `Lock` returns inside 8s rather than blocking.
- [x] `TestLockOrderIsSortedPaths` — two goroutines requesting the same two paths in opposite order, 50 times each, neither deadlocks within 30s.
- [x] A failed acquisition in a multi-path call releases the locks already taken; no partial hold survives the error.
- [x] `procoder check` is clean.

## Evidence

- `internal/store/lock.go` (158 lines) and `internal/store/lock_test.go`
  (146 lines), committed as da8644d. `go test ./internal/store/` passes,
  15.8s — dominated by the two tests that must wait out the 5s timeout.
- Six tests: TestLockIsExclusive, TestStaleLockIsBrokenAndReported,
  TestUnreadableLockIsStale (subtests `unparsable` and `future`),
  TestLiveLockRefusesRatherThanBlocks, TestLockOrderIsSortedPaths,
  TestPartialAcquisitionReleasesWhatItTook.
- Mutation-checked, snapshot taken and restored immediately around each:
  replacing `sort.Strings(paths)` with `_ = sort.StringSlice(paths)` makes
  TestLockOrderIsSortedPaths fail in 5.01s with `could not lock
.procoder/backlog/stories/s1.md within 5s`; dropping `|| age < 0` from
  `stale` fails the `future` subtest; dropping `os.O_EXCL` fails
  TestLockIsExclusive with `second lock succeeded while the first was
held`; replacing the `rel()` on a failed acquisition with `_ = rel`
  fails TestPartialAcquisitionReleasesWhatItTook.
- Plan corrected before implementing, per the skill: the plan had
  specified a package-level `Broken() []string`, which is shared mutable
  state read by concurrent `Lock` callers — a data race under exactly the
  concurrency this package exists to handle. `Lock` now returns `broken`
  as its second value. The plan and this task's criteria were updated
  first, and `plan check` re-run.
- `procoder check` clean at commit time; the commit gate passed both
  commits on this branch.

## Correction, 2026-08-28 (after this task closed)

Task 5's TestConcurrentAppendsBothSurvive found a defect in this task's
lock. `O_EXCL` creates the lock file empty and writes the pid and
timestamp a moment later, so for that moment every LIVE lock has contents
that do not parse. The rule shipped here — unparsable contents mean stale
— let a second caller steal a newborn lock, which is two holders of one
lock: the exact defect this package exists to remove.

The criterion "a lock file whose contents do not parse is treated as
stale" was wrong as written, not merely incompletely implemented. It is
now: the file's own mtime decides staleness first, and the contents get a
say only when they parse. TestNewbornLockIsNotStolen pins the case
directly rather than leaving it to surface as a lost append somewhere
downstream. Fixed in 3ff036d; the spec and plan were corrected rather
than left reading as though the first attempt had been right.

