# With the hook installed, a `git commit` on a tree with a blocking finding is stopped and the refusal names the finding.

Status: done 2026-08-19
Created: 2026-08-19
Epic: gate-enforcement
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The gate now actually stops a bad commit.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With the hook installed, a `git commit` on a tree with a blocking finding is stopped and the refusal names the finding.

## Evidence

- Live in a fixture repo: an unformatted file made `hook pre-tool-use` return permissionDecision deny naming the unformatted file. TestBlockingFindingStopsTheCommitAndNamesIt green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
