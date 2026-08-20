# auto-copilot-leak: gate integration, docs, and rot guards

Status: done 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20
Sprint: 006-auto-copilot-leak-capture-copilots-auto-review-findings-as

## Description

Final plumbing: integrate a lightweight check into the gate (run during
`procoder check`), create all docs/skill files, and add rot guard tests
so the command and its outputs don't drift.

## Acceptance criteria

- [x] The gate carries a non-blocking reminder when the leak ledger holds unclassified findings.
- [x] `commands/copilot-leak.md` exists with its OpenCode and Kilo twins, and `copilot-leak` is in docs.Commands.
- [x] AGENTS.md documents `copilot-leak`, and every derived host rule file matches — `procoder agents` reports no drift.
- [x] The docs site documents the command, its flags, and what it does and does not send.
- [x] The rot guards pass: usage, docs.Commands and the twin-parity tests.

## Evidence

- lessons.LeakReminder is wired at internal/gitcmd/gitcmd.go:121; TestLeakReminder* cover the reminder, the silent clean ledger and a repo with no ledger at all.
- `procoder agents` → 'every agent rule file matches AGENTS.md' after regenerating all eleven host files from the edited AGENTS.md.
- docs/commands.md gained a `procoder copilot-leak` section stating the sanitisation guarantee and the no-terminal-no-capture rule.
- `go test ./...` green, `procoder check` clean, `procoder docs` 0 blocking.
- DEVIATION: no `skills/auto-copilot-leak/SKILL.md`. `skills/procoder/SKILL.md` is generated from AGENTS.md by `procoder agents` and per-command guidance lives in `commands/*.md`, which copilot-leak already has — a second skill directory would fight the generator.
