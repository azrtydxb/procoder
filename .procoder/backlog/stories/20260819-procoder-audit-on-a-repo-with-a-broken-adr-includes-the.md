# `procoder audit` on a repo with a broken ADR includes the finding.

Status: done 2026-08-19
Created: 2026-08-19
Epic: adr
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

The audit sweep includes decision-record rot.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder audit` on a repo with a broken ADR includes the finding.

## Evidence

- audit.Run now reports adr.Check findings as a 'decision records (adr)' section when .procoder/adr exists (internal/audit/audit.go); go test ./internal/audit green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
