# A `sprint-status.yaml` that will not parse produces a blocking finding naming the file, distinct from the finding for one that is absent.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

Unreadable and absent are different answers, and only one of them is the
repository's choice. A parse failure reported as "no sprint yet" sends
somebody to plan work that is already planned.

Done means the unparseable case blocks and names the file, distinct from
the finding for a repository that has not planned anything.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A `sprint-status.yaml` that will not parse produces a blocking finding naming the file, distinct from the finding for one that is absent.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
