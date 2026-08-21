# Raising a default prints no such line — strengthening is not a warning.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

A line for every tightened setting would train the reader to skim exactly the place the relaxations appear. Done means strengthening is silent.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Raising a default prints no such line — strengthening is not a warning.

## Evidence

Same test, second half: `sast_blocks_at = "WARNING"` is a LOWER bar and therefore a strengthening, and produces no relaxation line. Proved by mutation: making isRelaxed return true for any value differing from the default prints a relaxation for a repository that made its gate stricter, and the test names it.
