# `procoder hook install-git` prints a working pre-commit script that runs the gate and returns non-zero on blocking findings, and writes nothing itself.

Status: done 2026-08-19
Created: 2026-08-19
Epic: gate-enforcement
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The gate holds outside any agent too.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder hook install-git` prints a working pre-commit script that runs the gate and returns non-zero on blocking findings, and writes nothing itself.

## Evidence

- TestInstallGitPrintsAndWritesNothing green: the pre-commit script runs `procoder check` and exits non-zero on blocking findings; nothing is written to .git/.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
