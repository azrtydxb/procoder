# A lockfile made unreadable yields a NOT-checked line naming it and exit 1, while the other lockfiles in the same fixture still report their verdicts.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

One unreadable file does not silence the rest.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A lockfile made unreadable yields a NOT-checked line naming it and exit 1, while the other lockfiles in the same fixture still report their verdicts.

## Evidence

- TestUnreadableLockfileIsNotCheckedWhileOthersStillReport (skipped on Windows and as root).

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
