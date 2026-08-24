# `procoder doctor` under `method = "bmad"` names BMad's installed version, and says plainly that it is absent when it is.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

A repository whose planning depends on a tool procoder does not ship
needs to know that tool is there, and which version — because BMad's
artifact layout is what procoder reads, and a layout change is a version
fact.

Done means doctor names the installed version, and says plainly that it
is absent when it is.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `procoder doctor` under `method = "bmad"` names BMad's installed version, and says plainly that it is absent when it is.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
