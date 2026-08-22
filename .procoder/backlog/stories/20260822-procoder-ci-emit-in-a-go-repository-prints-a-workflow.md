# `procoder ci --emit` in a Go repository prints a workflow containing the whole-tree steps (`check` over tracked files, `security --deep`, `docs --external`, `maintain`, `debt`, `deps`) and a Go suite step.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

The whole point: a repository asks what its CI should contain and is
handed it, rather than translating this repository's `ci.yml` by hand.

Done means the emitted workflow carries the whole-tree tier — `check`
over tracked files, `security --deep`, `docs --external`, `maintain`,
`debt`, `deps` — plus the suite step for the ecosystem actually
present.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `procoder ci --emit` in a Go repository prints a workflow containing the whole-tree steps (`check` over tracked files, `security --deep`, `docs --external`, `maintain`, `debt`, `deps`) and a Go suite step.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
