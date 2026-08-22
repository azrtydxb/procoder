# A workflow that runs the whole-tree commands under step names Procoder did not choose produces no missing-check finding.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

Detection has to match what runs, not what it is called. Step names
are the most-edited thing in any workflow, and a checker keyed on them
tells every repository that renamed a step that it lost a check it
still runs — which is how a checker gets ignored.

Done means a workflow running the right commands under any names
produces no missing-check finding.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A workflow that runs the whole-tree commands under step names Procoder did not choose produces no missing-check finding.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
