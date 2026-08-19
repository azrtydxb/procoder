# On a fixture whose ecosystem cannot express filtering, the same command reports "NOT filtered" for that ecosystem while the run itself still reports its real verdict.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

NOT filtered is said, never implied.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture whose ecosystem cannot express filtering, the same command reports "NOT filtered" for that ecosystem while the run itself still reports its real verdict.

## Evidence

- TestUnfilterableRunnerSaysNotFiltered: a pattern starting with `-` cannot be passed as a separate argv element to pytest/cargo/gradle, so those report NOT filtered while the run still reports its real verdict. Result.Filtered is forced false on every NOT-run path so the two verdicts never collapse.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
