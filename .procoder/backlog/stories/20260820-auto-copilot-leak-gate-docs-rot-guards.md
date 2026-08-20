# auto-copilot-leak: gate integration, docs, and rot guards

Status: open 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20
Sprint: 006-auto-copilot-leak-capture-copilots-auto-review-findings-as

## Description

Final plumbing: integrate a lightweight check into the gate (run during
`procoder check`), create all docs/skill files, and add rot guard tests
so the command and its outputs don't drift.

## Steps

1. Lightweight gate integration in `internal/gate/gate.go`:
   - After collecting findings, call a new function
     `copilot.NewIssues(root) int` that does a lightweight check:
     queries `gh issue list --state open --limit 50 --json author,labels`
     and counts issues that match `auto-copilot` or `copilot[bot]`
     created in the last 5 minutes (since the last check run).
   - If count > 0, appends an informational line:
     "copilot-leak: N new Copilot auto-review issue(s) opened since last check — run `copilot-leak` to capture".
   - Non-blocking (informational only, same as impact/blast-radius lines).

2. Create `commands/copilot-leak.md` (usage doc).

3. Create `.opencode/commands/copilot-leak.md` (OpenCode twin).

4. Create `skills/copilot-leak/SKILL.md` — a skill that instructs the
   agent on the copilot-leak workflow: when to run it, what to do with
   the output, how to close lessons.

5. Add rot guard tests:
   - `commands/copilot-leak.md` is referenced from `docs.Commands`.
   - `commands/copilot-leak.md` exists in every agent skill file's
     listed commands (AGENTS.md mirrors, `procoder agents` sync).
   - `copilot-leak` appears in usage text printed by the binary.

6. Update `AGENTS.md` and all agent skills to mention `copilot-leak`
   in the toolbox section (same location as other commands).

7. Update `docs/portability.md` if needed (any file that lists commands
   should mention `copilot-leak`).

## Files

- `internal/gate/gate.go` — add copilot.NewIssues() check (edit)
- `internal/copilot/gate.go` — NewIssues() function in the copilot package
- `commands/copilot-leak.md` — new file
- `.opencode/commands/copilot-leak.md` — new file
- `skills/auto-copilot-leak/SKILL.md` — new file
- `AGENTS.md` — mention copilot-leak (edit)
- `commands/merge.md` — merge workflow reference (edit)
- `docs/portability.md` — if it documents commands (edit)
- `internal/docs/docs.go` — Add `copilot-leak` to rot-guard if needed (edit)

## Verification

- `go test ./...` — all tests pass.
- `procoder agents` — no drift on `copilot-leak` mention.
- `procoder check` — informational line shown after first real Copilot
  leak is captured, not shown when no findings exist.
- `procoder skills list` — includes auto-copilot-leak skill.
