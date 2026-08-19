# `procoder env --sync` writes exactly one file (`.procoder/state/env.json`) — a test snapshots the tree before and after and asserts a single added path — and the written JSON contains no `.env` value.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

--sync writes one file, atomically.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder env --sync` writes exactly one file (`.procoder/state/env.json`) — a test snapshots the tree before and after and asserts a single added path — and the written JSON contains no `.env` value.

## Evidence

- TestSyncWritesExactlyOneFileAndNoValue: exactly one file appears, written temp-then-rename, and the planted secret value is absent from it. Live: `env --sync` recorded 48 lockfiles and the next run was clean.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
