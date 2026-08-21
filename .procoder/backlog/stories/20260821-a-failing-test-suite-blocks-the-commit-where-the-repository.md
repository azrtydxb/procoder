# A failing test suite blocks the commit where the repository set the test policy to block, and reports without blocking otherwise.

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

- [x] A failing test suite blocks the commit where the repository set the test policy to block, and reports without blocking otherwise.

## Evidence

- `go test ./internal/testrun/ -run TestASuiteThatCouldNotRun` — findingFor blocks a Fail only under `policy = "block"`, reports otherwise, and blocks a NotRun regardless. (#136)
