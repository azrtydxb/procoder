# Fix dependency scan review gaps

Status: closed 2026-09-17
Created: 2026-09-17

## Description

Address PR #285's empty Git result, deleted manifest, ignored gap-check,
and tracked fixture review findings before merging the dependency branch.

## Acceptance criteria

- [x] Successful empty Git inventories never fall back to ignored files.
- [x] Deleted tracked manifests do not reach the scanner inventory.
- [x] npm and Python gap checks use the same repository ownership scope.
- [x] Regression tests cover tracked, untracked, ignored, vendored, and quoted paths and non-Git fallback.
- [x] Review, gate, and full tests run over the final changes.

## Evidence

`procoder test` passed all 57 Go packages, including the empty-inventory,
deleted-tracked, gap-ownership, ignored-lockfile and quoted-path regressions
in internal/security/deps_test.go. The existing non-Git monorepo test also
passed. JavaScript has no standalone test script and was explicitly NOT run.

`procoder review --lens adversarial` and `--lens edge-case` guided the defect
pass: an ignored sibling or workspace lockfile could hide a visible package's
gap after being excluded from scanning. Both lockfile coverage checks now use
the shared inventory and TestIgnoredLockfilesCannotCoverVisibleDependencies
covers that failure. Reviewed directory replacement, deleted paths, empty Git
success, quoted paths and fallback. No new debt markers.

`procoder check` reported 4 clean files, zero unformatted or unchecked files,
and zero blocking findings. `git diff --check` passed. The full suite was run
separately from the gate after concurrent lint processes contended for a lock.
