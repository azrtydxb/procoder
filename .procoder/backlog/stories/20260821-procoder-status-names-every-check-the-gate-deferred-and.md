# `procoder status` names every check the gate deferred, and says nothing when none were.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

The gate narrows to the runners it can scope, so some suites are CI's.
That is a reasonable trade and an invisible one: a JavaScript commit
passes a green gate having never run its suite, and green reads as a
suite that passed.

Done means the state-of-play report names what the gate will not run
here, and says nothing when there is nothing to name — a line on every
session of a single-language repository is noise the reader learns to
skip, and a reader who skips this line is the reader it was written
for.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder status` names every check the gate deferred, and says nothing when none were.

## Evidence

- `procoder status` in a fixture with Cargo.toml and a package.json test script prints "gate defers to CI: rust, js suite(s) — the gate runs go, python"; silent in this repository. `go test ./internal/status/ -run TestTheReportNamesWhatTheGateDefers`. (#137)
