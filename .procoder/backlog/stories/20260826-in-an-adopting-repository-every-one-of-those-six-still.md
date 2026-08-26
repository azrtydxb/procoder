# An adopting repository loses nothing

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

The constraint the whole epic is built around, and the one that could
fail silently. A repository that adopted procoder asked for every one of
these checks, and must still get them — same findings, same words, same
blocking.

Asserted over the same fixture as the story above with `.procoder/` added,
so the two runs differ in adoption and nothing else. Any other difference
is the test lying about what it compared.

## Acceptance criteria

- [ ] In an adopting repository the unformatted file, missing agent rule
      file, README without the version, absent linter, debt marker and
      failing suite all still report, unchanged.
- [ ] The adopting and non-adopting fixtures are the same tree apart from
      `.procoder/`.

## Evidence
