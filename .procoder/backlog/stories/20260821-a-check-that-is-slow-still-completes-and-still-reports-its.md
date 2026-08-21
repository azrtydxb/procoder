# A check that is slow still completes and still reports its findings, asserted against a deliberately slow stub — the gate waits rather than reporting a verdict it did not reach.

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

- [x] A check that is slow still completes and still reports its findings, asserted against a deliberately slow stub — the gate waits rather than reporting a verdict it did not reach.

## Evidence

- `go test ./internal/security/ -run TestASlowCheckStillCompletes` — a semgrep stub delayed two seconds still returns its finding intact, and the call waited 2.55s rather than answering early. Red against a 1s ceiling returning what it reached: "semgrep output unreadable — SAST was NOT run". (#137)
