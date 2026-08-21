# A failing test suite blocks the commit where the repository set the test policy to block, and reports without blocking otherwise.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

`[test] policy` has always governed whether a failing suite stops the
close controllers. It should mean the same thing at the commit.

Done means a failing suite blocks where the repository asked for block
and reports otherwise — and a suite that could NOT run blocks either way,
because the policy governs whether a FAILING test stops a commit and "no
answer" is not a verdict it has an opinion about.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A failing test suite blocks the commit where the repository set the test policy to block, and reports without blocking otherwise.

## Evidence

- `go test ./internal/testrun/ -run TestASuiteThatCouldNotRun` — findingFor blocks a Fail only under `policy = "block"`, reports otherwise, and blocks a NotRun regardless. (#136)
