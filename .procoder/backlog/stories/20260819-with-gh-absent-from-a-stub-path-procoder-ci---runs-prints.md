# With gh absent from a stub PATH, `procoder ci --runs` prints the NOT-checked line naming gh and exits 1, and the same fixture run as bare `procoder ci` prints the unchanged hygiene findings.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Honest without gh.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With gh absent from a stub PATH, `procoder ci --runs` prints the NOT-checked line naming gh and exits 1, and the same fixture run as bare `procoder ci` prints the unchanged hygiene findings.

## Evidence

- TestGhAbsentFromPathYieldsNotChecked builds a stub PATH with no gh and asserts the NOT-checked line and its reason.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
