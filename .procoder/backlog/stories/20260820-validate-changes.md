# Validate all changes with procoder gate and tests

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Run the full procoder gate and test suite to validate all changes from tasks 1-10. This is the final validation step before considering the epic complete. The gate (`procoder check`) must pass, the test suite must pass, and no linting/blocking findings should remain.

## Acceptance criteria

- [ ] `procoder check` passes — no blocking findings
- [ ] `procoder test` passes — suite verifiable and green
- [ ] `procoder format` — all new/modified files are formatted
- [ ] `procoder lint` — no blocking findings
- [ ] All JSON files in the repo parse without syntax errors
- [ ] YAML frontmatter in SKILL.md parses without errors
- [ ] `procoder check` on any modified integration file passes
- [ ] No new conflicts with existing `.procoder/` state

## Evidence
