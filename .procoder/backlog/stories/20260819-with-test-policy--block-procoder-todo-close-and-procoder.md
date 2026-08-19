# With `[test] policy = "block"`, `procoder todo close` and `procoder backlog close story` refuse while the suite fails and pass once it is green — verified by tests walking both.

Status: done 2026-08-19
Created: 2026-08-19
Epic: test-domain
Sprint: 001-the-test-domain-procoder-test-coverage-close-wiring-0270

## Description

The suite verdict joins both close controllers.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With `[test] policy = "block"`, `procoder todo close` and `procoder backlog close story` refuse while the suite fails and pass once it is green — verified by tests walking both.

## Evidence

- TestCloseWithSuiteVerdict (todo) and TestCloseStoryWithSuiteVerdict (backlog): red suite refuses with the summary line, green closes. Live: backlogdemo story close refused on unverifiable suite, closed after adding a passing test.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
