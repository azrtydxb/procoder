# A commit adding a function past the complexity limit is blocked.

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

- [x] A commit adding a function past the complexity limit is blocked.

## Evidence

- `go test ./internal/maintain/` — ComplexityChanged reports a function over the limit for a changed file and blocks when `[maintain] policy = "block"`. (#134)
