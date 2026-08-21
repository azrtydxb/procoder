# The same commit produces the same findings on a fast and a slow run, asserted by running the gate twice against stubs of different speeds and comparing the output.

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

- [x] The same commit produces the same findings on a fast and a slow run, asserted by running the gate twice against stubs of different speeds and comparing the output.

## Evidence

- `go test ./internal/security/ -run TestFastAndSlowRunsAgree` — two runs differing only in a 2s stub delay produce identical findings. Red against a 1s ceiling: the two runs disagree. (#137)
