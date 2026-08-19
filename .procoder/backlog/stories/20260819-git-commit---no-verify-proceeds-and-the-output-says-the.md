# `git commit --no-verify` proceeds and the output says the gate was bypassed.

Status: done 2026-08-19
Created: 2026-08-19
Epic: gate-enforcement
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

Bypass is allowed but never silent.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `git commit --no-verify` proceeds and the output says the gate was bypassed.

## Evidence

- Live: `git commit --no-verify -m x` → allow with 'the commit gate was bypassed deliberately'. TestNoVerifyPassesThroughLoudly green; -n is treated as the same bypass.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
