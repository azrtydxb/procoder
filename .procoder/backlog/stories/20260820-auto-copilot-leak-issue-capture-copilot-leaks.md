# auto-copilot-leak: issue capture and COPILOT-LEAKS.md

Status: open 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20

## Description

Complete the capture path: when the user says yes, create GitHub issues
for each sanitised finding and append entries to a new COPILOT-LEAKS.md
scratch ledger. No integration with lessons yet — that comes in a later
story.

## Steps

1. Create `internal/copilot/capture.go` (or extend `copilot.go`):
   - Implement `Capture(finds []Sanitised, root string) (int, int, []string)`:
     - For each sanitised finding, calls `gh issue create` with:
       - Title: the sanitized title
       - Label: `auto-copilot` via `--label auto-copilot`
       - Body: sanitised body, plus a "Original issue" link and
         timestamp as a separate paragraph
       - Uses `--title` and `--body` flags for `gh issue create`
     - For each finding, appends an entry to
       `.procoder/github/COPILOT-LEAKS.md`:
       ```markdown
       ## <date> <original-url> — <title>

       - Source: Copilot auto-review
       - Original: <url>
       - Sanitised:
         <sanitised body text>
       - Adaptation: <the concrete change that catches this class from now on>
       ```
     - Returns (issuesCreated, lessonsWritten, pathsChanged).
   - Test: fixture with recorded `gh issue create --json` output.
   - COPILOT-LEAKS.md path: `.procoder/github/COPILOT-LEAKS.md`.
     Created if it does not exist, initialised with a header.
2. Wire capture into the command handler in `main.go`:
   - After `Prompt()` returns true, calls `Capture(sanitizedFinds, root)`.
   - Prints summary: "Created N issue(s), recorded N lesson(s)".
   - On issue creation failure: notes the failure but continues with
     remaining findings (does not block on one failure).
   - On COPILOT-LEAKS.md write failure: continues, prints warning.
3. Create `.procoder/github/COPILOT-LEAKS.md` initial file with header
   and one example entry (unindented, like LESSONS.md).
4. Create `.github/ISSUE_TEMPLATE/copilot-leak.md`:
   - Template for creating auto-copilot issues (used as the body
     template for `gh issue create`).

## Files

- `internal/copilot/capture.go` — Capture function
- `internal/copilot/capture_test.go` — capture tests
- `.procoder/github/COPILOT-LEAKS.md` — new scratch ledger
- `.github/ISSUE_TEMPLATE/copilot-leak.md` — issue template master (will be mirrored to `.github/ISSUE_TEMPLATE/`)
- `cmd/procoder/main.go` — wire Capture into command (edit)
- `internal/gitcmd/gitcmd.go` — mirrorSync() gain for the issue template (edit)

## Verification

`go test ./internal/copilot/...`
`procoder copilot-leak --quiet` runs without panic.
