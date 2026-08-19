# Parse tests over recorded `gh run list --json` output cover a failing run (with failing job names), an in-progress run, an empty run list, and a newest run whose headSha differs from a pushed HEAD — the last producing the "newest run predates your latest push" line.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

The gh shapes are pinned.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Parse tests over recorded `gh run list --json` output cover a failing run (with failing job names), an in-progress run, an empty run list, and a newest run whose headSha differs from a pushed HEAD — the last producing the "newest run predates your latest push" line.

## Evidence

- TestRunListParsesTheRecordedShape, TestFailedRunNamesTheFailingJobsAndInProgressClaimsNoConclusion, TestStaleRunSaysCIHasNotJudgedThisCommit, TestEmptyRunListIsNeverReadAsGreen, TestUnparseableRunListIsAnError, TestAgesReadTheWayAReaderThinks.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
