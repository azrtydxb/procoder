# CI runs `maintain`, `debt` and `deps` over the whole tree and fails the job on a blocking finding from any of them.

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

- [x] CI runs `maintain`, `debt` and `deps` over the whole tree and fails the job on a blocking finding from any of them.

## Evidence

- `.github/workflows/ci.yml` whole-tree pass step runs `procoder maintain`, `procoder debt` and `procoder deps`; the gate job has passed on every PR since. (#132)
