# `procoder backlog bug "login 500s" --severity s1` prints a story file with Type, Severity, the repro-prompting description, and the pre-seeded regression-test criterion; writing and closing it without a Severity header is refused.

Status: done 2026-08-19
Created: 2026-08-19
Epic: backlog-extensions
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Bugs are typed, severe, and regression-pinned.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder backlog bug "login 500s" --severity s1` prints a story file with Type, Severity, the repro-prompting description, and the pre-seeded regression-test criterion; writing and closing it without a Severity header is refused.

## Evidence

- TestBugPrintsTypedStoryWithRegressionCriterion, TestBugRefusesInvalidSeverity, TestCloseBugRefusesWithoutSeverity all green. Live: `backlog bug "login 500s on empty password" --severity s1` printed Type: bug, Severity: s1, and the regression criterion.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
