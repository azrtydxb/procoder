# `procoder analyze check` refuses a hollow analysis document — a section left as its template comment is not a filled section — and passes a filled one.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The analysis phase is only worth having if its documents are worth
reading. A brief whose sections are still template comments has recorded
nothing, and passing it would make the phase a formality that costs time
and buys nothing.

Done means `analyze check` refuses a hollow document the way `spec check`
already refuses a hollow spec — same standard, same reason.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `procoder analyze check` refuses a hollow analysis document — a section left as its template comment is not a filled section — and passes a filled one.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
