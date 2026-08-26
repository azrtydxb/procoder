# An adopting repository loses nothing

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

The constraint the whole epic is built around, and the one that could
fail silently. A repository that adopted procoder asked for every one of
these checks, and must still get them — same findings, same words, same
blocking.

Asserted over the same fixture as the story above with `.procoder/` added,
so the two runs differ in adoption and nothing else. Any other difference
is the test lying about what it compared.

## Acceptance criteria

- [x] In an adopting repository the unformatted file, missing agent rule
      file, README without the version, absent linter, debt marker and
      failing suite all still report, unchanged.
- [x] The adopting and non-adopting fixtures are the same tree apart from
      `.procoder/`.

## Evidence

`TestTheSameFixtureKeepsItsHouseRulesWhenAdopting`, over the fixture built
by `houseRuleFixture(t, adopting)` — one function, both runs, so "the same
tree apart from `.procoder/`" is structural rather than a claim in a
comment. Formatting, debt, the agent layer, templates and ignore-coverage
all still report, and the run still exits 1. Killed by removing the
`houseRules` call.
