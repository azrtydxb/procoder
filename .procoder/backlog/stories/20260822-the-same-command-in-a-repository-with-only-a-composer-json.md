# The same command in a repository with only a `composer.json` and PHP sources prints the PHP suite step and no Go step.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

Modular means the emitter answers about the repository in front of it.
A PHP project handed a Go workflow learns nothing except that this tool
was built for somebody else.

Done means a repository with only `composer.json` and PHP sources gets
the PHP suite step and no Go step at all.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] The same command in a repository with only a `composer.json` and PHP sources prints the PHP suite step and no Go step.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
