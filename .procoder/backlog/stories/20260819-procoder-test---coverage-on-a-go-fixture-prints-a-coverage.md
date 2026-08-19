# `procoder test --coverage` on a Go fixture prints a coverage percentage; on an ecosystem without native coverage it prints "not measured".

Status: done 2026-08-19
Created: 2026-08-19
Epic: test-domain
Sprint: 001-the-test-domain-procoder-test-coverage-close-wiring-0270

## Description

Coverage reported, never enforced.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder test --coverage` on a Go fixture prints a coverage percentage; on an ecosystem without native coverage it prints "not measured".

## Evidence

- TestGoCoverageIsReported green. Live: `procoder test --coverage` printed `coverage 57.0%` (mean of 29 covered packages) for go and no number for js.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
