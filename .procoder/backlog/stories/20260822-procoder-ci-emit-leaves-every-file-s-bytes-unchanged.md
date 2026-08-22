# `procoder ci --emit` leaves every file's bytes unchanged, asserted by comparing a digest of the tree before and after.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

P-CONTROL, at the surface where breaking it would be most tempting.
A tool that writes CI is a tool that can disable CI, and a generator
that edits files is one bad merge away from replacing a workflow
somebody depends on.

Done means the command prints, and a digest of the tree is identical
before and after.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `procoder ci --emit` leaves every file's bytes unchanged, asserted by comparing a digest of the tree before and after.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
