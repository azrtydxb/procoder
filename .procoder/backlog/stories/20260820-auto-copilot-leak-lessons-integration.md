# auto-copilot-leak: lessons integration and `--from-copilot`

Status: open 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20

## Description

Connect COPILOT-LEAKS.md to the existing lessons system. Entries in
COPILOT-LEAKS.md that have placeholder adaptations (`<...>`) are
UNLEARNED. Provide a way for `procoder lessons` to see them and for
the agent to convert COPILOT-LEAKS.md entries into formal LESSONS.md
entries.

## Steps

1. Extend `internal/lessons/lessons.go`:
   - Add `CopilotLeaksPath = ".procoder/github/COPILOT-LEAKS.md"`.
   - Implement `ParseCopilot(text string) []Entry` — reuses the existing
     Parse logic (same entry shape).
   - Implement `RecordCopilotEntry(path string, entry Entry) error` —
     appends an entry to COPILOT-LEAKS.md.
2. Extend `internal/lessons.Run()` or add `RunFromCopilot()`:
   - When called as `RunFromCopilot()`, reads COPILOT-LEAKS.md.
   - For each entry with `Adaptation` starting with `<` or empty:
     - Prints "UNLEARNED <title> — see also GitHub issue <original-url>".
   - For each entry with a real adaptation:
     - Prints "learned <title>".
   - Exits 1 if any unlearned, 0 if all learned.
3. Add a `--from-copilot` flag to the `copilot-leak` command:
   - `procoder copilot-leak --from-copilot` — same as `lessons` but
     for COPILOT-LEAKS.md. Exits 1 if unlearned.
4. Wire into `cmd/procoder/main.go`:
   - `copilot-leak --from-copilot` calls the adapted lessons check.
5. Update the merge flow (`commands/merge.md` section 2b):
   - At the end of the reflection phase, if `copilot-leak` hasn't run
     today, run `procoder copilot-leak --quiet` (non-blocking).
   - If it reports findings, prompt the agent/merge to run
     `copilot-leak` to capture them.

## Files

- `internal/lessons/lessons.go` — extend with CopilotLeaksPath, ParseCopilot, RecordCopilotEntry, RunFromCopilot
- `cmd/procoder/main.go` — wire `--from-copilot` flag (edit)
- `commands/merge.md` — add copilot-leak step to merge reflection (edit)

## Verification

`go test ./internal/lessons/...`
Test `RunFromCopilot()` over a file with mixed learned/unlearned entries.
`copilot-leak --from-copilot` on the repo reports its current COPILOT-LEAKS.md state.
