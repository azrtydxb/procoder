# `[planning] method = "bmad"` with no BMad installed produces a blocking finding naming both the setting and the missing installation.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: -

## Description

Silently falling back to procoder's own chain would leave a repository
believing BMad governs its planning while procoder quietly governed it
instead — and the first they would learn of it is a report that does not
match the artifacts on disk.

Done means the mismatch is named: a blocking finding citing both the
setting and the missing installation.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `[planning] method = "bmad"` with no BMad installed produces a blocking finding naming both the setting and the missing installation.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
