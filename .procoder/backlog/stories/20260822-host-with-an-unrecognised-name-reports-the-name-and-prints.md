# `--host` with an unrecognised name reports the name and prints no workflow.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

A partial workflow is worse than none: YAML that parses but omits
half the checks is a CI that silently tests less, and nothing in the
run says so.

Done means an unrecognised `--host` names what it was given and prints
nothing at all.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `--host` with an unrecognised name reports the name and prints no workflow.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
