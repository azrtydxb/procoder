# `procoder run --exec` on a single candidate naming a server verb (`npm run dev`) refuses, exits 2, and prints the command with the instruction to run it in the agent's own background shell.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

A server belongs to the shell that owns it.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder run --exec` on a single candidate naming a server verb (`npm run dev`) refuses, exits 2, and prints the command with the instruction to run it in the agent's own background shell.

## Evidence

- TestExecRefusesALongRunningCandidate and TestMakefileRecipeDecidesLongRunning: the recipe decides, not the target name; a Procfile web entry is long-running by definition.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
