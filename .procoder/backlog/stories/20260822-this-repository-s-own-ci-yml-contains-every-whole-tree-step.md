# This repository's own `ci.yml` contains every whole-tree step the emitter emits for it, asserted by a test, so the shipped example and the running CI cannot drift apart.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

This repository's CI is also the worked example. If the emitter and
`ci.yml` drift apart, then either the example is a fiction or our own CI
has quietly lost a check — and both failures are invisible without
something asserting they agree.

Done means a test holds the two together, so the drift cannot happen
silently.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] This repository's own `ci.yml` contains every whole-tree step the emitter emits for it, asserted by a test, so the shipped example and the running CI cannot drift apart.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
