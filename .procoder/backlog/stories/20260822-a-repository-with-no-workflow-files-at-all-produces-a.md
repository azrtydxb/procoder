# A repository with no workflow files at all produces a finding naming the whole-tree tier as missing, where `ciops.Check` previously returned nil.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

A repository that adopts Procoder gets a commit gate and no second
tier, and today nothing tells it so. `ciops.Check` opens
`.github/workflows`, finds nothing, and returns nil — the repository is
reported clean by the domain whose subject is CI.

Done means it is told: a finding that names the whole-tree tier as
missing, so somebody who has never read this spec learns the tier exists
at the moment it is absent.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A repository with no workflow files at all produces a finding naming the whole-tree tier as missing, where `ciops.Check` previously returned nil.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
