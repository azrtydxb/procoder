# CI runs `maintain`, `debt` and `deps` over the whole tree and fails the job on a blocking finding from any of them.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

`maintain`, `debt` and `deps` answer about the tree, not the change:
complexity across a codebase, the debt ledger, dependency freshness.
Asking them of every commit would either repeat the same finding forever
or rescan everything each time somebody edits a line.

Until now they ran nowhere unless a person typed them, which meant they
protected whoever already knew they existed. Done means CI runs all three
over the whole tree and the job fails on what they find — a command that
reports into a log nobody reads is not a check.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] CI runs `maintain`, `debt` and `deps` over the whole tree and fails the job on a blocking finding from any of them.

## Evidence

- `.github/workflows/ci.yml` runs `procoder maintain`, `procoder debt` and `procoder deps` in the whole-tree pass. Exit codes verified by hand: maintain returns 1 when a check could NOT run, deps returns 1 when a tool errored, debt returns 1 on a marker with no revisit trigger (#138 — it returned 0 unconditionally before, so the step could only ever pass). (#132, #138)
