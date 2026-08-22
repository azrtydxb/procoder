# A repository carrying `.procoder/ci/whole-tree.yml` gets that content in place of the built-in block, and a repository without the file gets the built-in block unchanged.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

D-OVERRIDE, applied here like everywhere else: a repository that
disagrees with a block replaces it, without forking the emitter or
abandoning the rest of what it generates.

Done means the override file's content appears in place of the built-in
block, and a repository without the file gets the built-in unchanged.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] A repository carrying `.procoder/ci/whole-tree.yml` gets that content in place of the built-in block, and a repository without the file gets the built-in block unchanged.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
