# `procoder status` names every check the gate deferred, and says nothing when none were.

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

- [x] `procoder status` names every check the gate deferred, and says nothing when none were.

## Evidence

- `procoder status` in a fixture with Cargo.toml and a package.json test script prints "gate defers to CI: rust, js suite(s) — the gate runs go, python"; silent in this repository. `go test ./internal/status/ -run TestTheReportNamesWhatTheGateDefers`. (#137)
