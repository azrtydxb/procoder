# Every command shipped in 0.29.0 appears in AGENTS.md, docs/configuration.md carries all config keys the binary reads, and docs/domains.md, workflow.md, index.md, README.md, and the mkdocs navigation describe the backlog, test, adr, deps, bench, and release capabilities.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The backfill.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Every command shipped in 0.29.0 appears in AGENTS.md, docs/configuration.md carries all config keys the binary reads, and docs/domains.md, workflow.md, index.md, README.md, and the mkdocs navigation describe the backlog, test, adr, deps, bench, and release capabilities.

## Evidence

- AGENTS.md gained a work-chain section and all 19 missing commands (and the 10 derived host rule files were regenerated — `procoder agents` says every file matches); configuration.md carries all six missing config keys; domains.md is ten domains with Testing added and bench/deps/adr placed; workflow.md rewritten as the real sequence; index.md, getting-started.md, README.md and the mkdocs nav updated. Four false claims fixed, including the unbacked 'start in a worktree'.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
