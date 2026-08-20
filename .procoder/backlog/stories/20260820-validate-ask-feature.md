# Final validation of ask feature

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Run the full validation suite end-to-end on procoder's own repo and a test scenario to ensure the ask feature works correctly.

## Acceptance criteria

- [ ] `procoder ask` runs on procoder's own repo without error
- [ ] `procoder ask` on a fixture repo with unresolved questions collects them correctly
- [ ] `procoder check` after `procoder ask` with answers passes (questions accepted)
- [ ] `procoder ask` without TTY writes `.procoder/ask/QA.md` and exits 1
- [ ] `procoder ask --file ans.md` reads answers and exits 0
- [ ] Hook output on a write event includes `== q&a` section when questions exist
- [ ] `procoder format` on new files passes
- [ ] `procoder lint` — no blocking findings
- [ ] `procoder check` — gate must pass after all changes
- [ ] `procoder test` — all tests in the repo pass

## Evidence
