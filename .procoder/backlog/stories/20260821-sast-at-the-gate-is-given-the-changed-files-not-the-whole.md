# SAST at the gate is given the changed files, not the whole tree, asserted by a fixture where an untouched file carries a finding and the commit is not blocked by it.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

SAST at the gate must answer about the commit. A developer blocked by a
finding in a file they have never opened cannot act on it, and learns
that the way to commit is to turn the gate off.

Done means an untouched file's finding does not block the commit,
asserted by a fixture that carries one. Note the implementation scans the
tree and filters the results: naming target files changes what semgrep
itself chooses to scan, which produced findings the real tool never
reports.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] SAST at the gate is given the changed files, not the whole tree, asserted by a fixture where an untouched file carries a finding and the commit is not blocked by it.

## Evidence

- `go test ./internal/security/ -run TestOnlyFindingsInChangedFilesBlockTheCommit` — a finding in an untouched file does not block. Note the implementation scans the tree and filters, because naming targets changed semgrep's own file selection. (#133)
