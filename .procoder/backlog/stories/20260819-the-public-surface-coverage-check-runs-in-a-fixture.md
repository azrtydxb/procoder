# The public-surface coverage check runs in a fixture repository whose go.mod names another module, and reports exported surface that no document mentions.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The identity gate is gone.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The public-surface coverage check runs in a fixture repository whose go.mod names another module, and reports exported surface that no document mentions.

## Evidence

- CommandCoverage and isProcoderRepo deleted; SurfaceCoverage runs anywhere. TestSurfaceCoverageRunsInAnyRepositoryAndNamesUndocumentedSurface green in a module example.com fixture; it reports up to 20 undocumented exported symbols, entrypoints ranked first.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
