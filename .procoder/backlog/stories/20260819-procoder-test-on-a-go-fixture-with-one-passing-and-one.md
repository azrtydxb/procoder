# `procoder test` on a Go fixture with one passing and one failing package reports FAIL with the failing test named and exits 1; after fixing, it reports PASS and exits 0.

Status: done 2026-08-19
Created: 2026-08-19
Epic: test-domain
Sprint: 001-the-test-domain-procoder-test-coverage-close-wiring-0270

## Description

procoder test detects go and reports honestly.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder test` on a Go fixture with one passing and one failing package reports FAIL with the failing test named and exits 1; after fixing, it reports PASS and exits 0.

## Evidence

- TestGoFailThenPass (internal/testrun): fixture with a red package exits 1 naming TestAlwaysRed; after the fix the same command exits 0. go test ./internal/testrun green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
