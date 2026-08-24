# `procoder review` over a fixture diff prints all five lenses with the content in scope, and leaves every file's bytes unchanged, asserted by comparing a digest of the tree before and after.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: -

## Description

The gate reads formatting, secrets, linters and hygiene — every one of
them mechanical. Nothing in procoder applies judgment to a change, so
whether anyone asks "is this the right shape, what breaks at the edges"
depends on who happens to be looking.

Done means `procoder review` exists and prints all five lenses over the
content in scope, for the agent to judge. And it writes nothing: the
binary prints, the agent decides — the same contract `procoder format`
already holds, asserted by comparing a digest of the tree before and
after.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `procoder review` over a fixture diff prints all five lenses with the content in scope, and leaves every file's bytes unchanged, asserted by comparing a digest of the tree before and after.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
