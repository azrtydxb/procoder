# Under `method = "bmad"`, `procoder check` produces byte-identical output to the same tree under `method = "procoder"`, asserted on a fixture — the setting governs planning and nothing else.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The seam this whole track rests on: planning moves, governance does not.
It is the only reason a BMad repository would install procoder at all, and
the tempting design — a spectrum where the setting dials procoder back by
degrees — erodes it one exception at a time.

Done means the gate's output is byte-identical across the setting on a
fixture, so the seam cannot drift without a test going red.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] Under `method = "bmad"`, `procoder check` produces byte-identical output to the same tree under `method = "procoder"`, asserted on a fixture — the setting governs planning and nothing else.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
