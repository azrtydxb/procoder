# `[planning] method = "bmad"` with a fixture BMad install reports sprint state from `sprint-status.yaml`, with each story's status, and does not report from `.procoder/backlog/`.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The whole promise of track 2. A repository that plans in BMad should see
its own sprint reflected back by procoder, not an empty procoder backlog
next to a BMad one that is actually being worked.

Done means sprint state comes from `sprint-status.yaml` with each story's
status, and `.procoder/backlog/` is not the source.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `[planning] method = "bmad"` with a fixture BMad install reports sprint state from `sprint-status.yaml`, with each story's status, and does not report from `.procoder/backlog/`.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
