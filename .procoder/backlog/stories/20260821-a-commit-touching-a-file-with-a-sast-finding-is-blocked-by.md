# A commit touching a file with a SAST finding is blocked by the gate at the configured severity, where previously only CI saw it.

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

- [x] A commit touching a file with a SAST finding is blocked by the gate at the configured severity, where previously only CI saw it.

## Evidence

- `go test ./internal/security/ -run TestASastFindingInAChangedFileBlocks` — a stubbed ERROR finding in a changed file blocks; `[security] sast_blocks_at` selects the bar. (#133)
