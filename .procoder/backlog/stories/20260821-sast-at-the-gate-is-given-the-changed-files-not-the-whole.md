# SAST at the gate is given the changed files, not the whole tree, asserted by a fixture where an untouched file carries a finding and the commit is not blocked by it.

Status: done
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] SAST at the gate is given the changed files, not the whole tree, asserted by a fixture where an untouched file carries a finding and the commit is not blocked by it.

## Evidence

- `go test ./internal/security/ -run TestOnlyFindingsInChangedFilesBlockTheCommit` — a finding in an untouched file does not block. Note the implementation scans the tree and filters, because naming targets changed semgrep's own file selection. (#133)
