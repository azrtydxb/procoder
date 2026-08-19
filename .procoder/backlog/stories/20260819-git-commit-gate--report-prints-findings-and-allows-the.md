# `[git] commit_gate = "report"` prints findings and allows the commit; `"off"` skips the check entirely.

Status: done 2026-08-19
Created: 2026-08-19
Epic: gate-enforcement
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The policy knob works both ways.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `[git] commit_gate = "report"` prints findings and allows the commit; `"off"` skips the check entirely.

## Evidence

- TestReportPolicyAllowsAndStillNamesTheFindings and TestOffPolicySkipsTheCheckEntirely green; [git] commit_gate is parsed in internal/config with block as the default.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
