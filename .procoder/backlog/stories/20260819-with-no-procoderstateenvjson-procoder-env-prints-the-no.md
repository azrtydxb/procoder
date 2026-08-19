# With no `.procoder/state/env.json`, `procoder env` prints the no-baseline line naming `--sync` and exits 0; after a `--sync` run on the same tree, a second bare run reports no changes and exits 0.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

No baseline is said plainly.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With no `.procoder/state/env.json`, `procoder env` prints the no-baseline line naming `--sync` and exits 0; after a `--sync` run on the same tree, a second bare run reports no changes and exits 0.

## Evidence

- TestNoBaselineSaysSoThenSyncMakesASecondRunClean. Live: the first run on this repo pointed at --sync.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
