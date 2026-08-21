# A commit touching a file with a SAST finding is blocked by the gate at the configured severity, where previously only CI saw it.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

A security finding caught in CI is caught after the code has left the
machine, been pushed, and started a review. Caught at the commit it is
still a thought the author is having.

Done means a commit touching a file with a finding at or above the
configured severity is blocked, where previously only CI saw it. The
severity bar is the repository's to set — `[security] sast_blocks_at`.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A commit touching a file with a SAST finding is blocked by the gate at the configured severity, where previously only CI saw it.

## Evidence

- `go test ./internal/security/ -run TestASastFindingInAChangedFileBlocks` — a stubbed ERROR finding in a changed file blocks; `[security] sast_blocks_at` selects the bar. (#133)
