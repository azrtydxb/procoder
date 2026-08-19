# AGENTS.md and root-level Markdown are part of the documentation corpus — verified by a test where the only mention of a symbol lives in AGENTS.md.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The corpus includes what other hosts read.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] AGENTS.md and root-level Markdown are part of the documentation corpus — verified by a test where the only mention of a symbol lives in AGENTS.md.

## Evidence

- TestSurfaceCoverageCountsAgentsMarkdownAsDocumentation green; the old README-plus-docs/ corpus filter died with CommandCoverage, so AGENTS.md now counts as documentation.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
