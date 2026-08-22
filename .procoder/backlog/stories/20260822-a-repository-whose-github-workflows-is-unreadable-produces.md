# A repository whose `.github/workflows` is unreadable produces a different, blocking finding that names the directory — not the absent-CI one.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

Absent and unreadable are different answers, and only one of them is
the repository's choice. A permissions failure that reads as "no CI
here" would send somebody to write a workflow that already exists.

Done means the unreadable case blocks and names the directory, distinct
from the finding a repository with genuinely no CI receives.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A repository whose `.github/workflows` is unreadable produces a different, blocking finding that names the directory — not the absent-CI one.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
