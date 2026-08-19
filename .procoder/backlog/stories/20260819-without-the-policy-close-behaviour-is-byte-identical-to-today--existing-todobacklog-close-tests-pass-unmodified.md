# Without the policy, close behaviour is byte-identical to today — existing todo/backlog close tests pass unmodified.

Status: done 2026-08-19
Created: 2026-08-19
Epic: test-domain
Sprint: 001-the-test-domain-procoder-test-coverage-close-wiring-0270

## Description

No policy, no change.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Without the policy, close behaviour is byte-identical to today — existing todo/backlog close tests pass unmodified.

## Evidence

- Close/CloseStory delegate to the With variants with suite nil; every pre-existing todo and backlog close test passes unmodified (go test ./internal/todo ./internal/backlog green).

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
