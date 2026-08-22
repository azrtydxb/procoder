# An empty `.procoder/ci/whole-tree.yml` blocks and names the file, rather than falling back to the built-in.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

Falling back to the built-in when an override cannot be read would
mean a repository believing it had replaced a block that is still
running Procoder's version — a silent green wearing a config file.

Done means an empty or unreadable override blocks and names the file
rather than quietly substituting the default.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] An empty `.procoder/ci/whole-tree.yml` blocks and names the file, rather than falling back to the built-in.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
