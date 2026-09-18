# Fix deferred review instructions

Status: closed 2026-09-18
Created: 2026-09-18

## Description

Address PR270's command syntax reviews and clarify that deferred follow-up
work cannot waive blocking findings or immediate reflection. Keep all host
command copies consistent with the canonical command.

## Acceptance criteria

- [x] All three command copies include the required task title.
- [x] Deferral explicitly preserves blocking checks and immediate reflection.
- [x] Command parity tests, the suite, and the gate pass.

## Evidence

- Read all three command copies: `todo add <title>` is present and the
  printed template must be written before replying with its id.
- Reviewed the deferral path against step 2b: blocking checks and required
  changes cannot be deferred, and immediate detecting-layer adaptation remains.
- `procoder check`: seven clean files, no unchecked or blocking findings.
- `procoder test`: Go passed, 57 packages; JavaScript NOT run (no test script).
- `go test ./internal/portability -run 'Test(OpenCode|Kilo)CommandParity'`: passed.
- `procoder lessons`: 43 learned, zero unlearned.
- `procoder review` adversarial pass found the print-only template gap;
  instructions now require persistence. Edge-case pass checked blocking versus
  non-blocking findings, required review changes, and reflection failure.
- Existing release-date question is intentionally unchanged and unanswered.
