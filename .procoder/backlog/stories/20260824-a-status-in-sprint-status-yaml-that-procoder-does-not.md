# A status in `sprint-status.yaml` that procoder does not recognise is reported by name rather than mapped to a procoder status.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

BMad owns its status vocabulary and may extend it. Mapping an unknown
status to the nearest procoder equivalent — deciding `blocked` is close
enough to `open` — is how a status machine quietly loses a state, and the
report then misrepresents work nobody can see is misrepresented.

Done means an unrecognised status is reported by name.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A status in `sprint-status.yaml` that procoder does not recognise is reported by name rather than mapped to a procoder status.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
