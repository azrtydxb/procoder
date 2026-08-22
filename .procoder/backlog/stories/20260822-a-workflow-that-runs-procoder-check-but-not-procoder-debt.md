# A workflow that runs `procoder check` but not `procoder debt` is told that `debt` is missing and is not told that `check` is.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

Most repositories will not adopt a generated file wholesale; they
have a workflow already and want to know what it is missing. A report
that says "your CI is incomplete" is useless, and one that says "add
these six steps" when five are present is worse.

Done means the finding names exactly the checks that are absent, and
says nothing about the ones that are there.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A workflow that runs `procoder check` but not `procoder debt` is told that `debt` is missing and is not told that `check` is.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
