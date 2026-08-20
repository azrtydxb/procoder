# auto-copilot-leak: command shell, sanitisation, and prompt

Status: open 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20
Sprint: 006-auto-copilot-leak-capture-copilots-auto-review-findings-as

## Description

Implement the `copilot-leak` command shell and the sanitisation layer.
No issue creation yet. This story delivers the foundation: a new
`internal/copilot` package with `Find`, `Sanitise`, and `Prompt`
functions, wired to the CLI under `cmd/procoder/main.go`.

## Steps

1. Create `internal/copilot/` package (new directory).
2. Implement `Sanitise(Finding, root) Sanitised`:
   - Strip fenced code blocks from the body.
   - Redact secret patterns (`ghp_`, `gho_`, `sk-`, `AIza`, `gh_`).
   - Replace user project paths with `.` (keep only relative structure).
   - Preserve: defect description, file:line, suggested fix.
   - Tests in `copilot/sanitise_test.go` over a fixture containing
     raw code blocks, secrets, and full paths, asserting they are all
     stripped/redacted.
3. Implement `Find(root, since time.Duration) []Finding`:
   - Uses `gh issue list --json author,labels,state,body,title,createdAt,url --state all`
     to query issues matching:
     - author matches `copilot.*\[bot\]`
     - OR label `auto-copilot` present
     - OR body contains `---\n> **Copilot**`
     - Filtered to results created within `since`.
   - For each match, calls `gh issue view <number> --json body` to
     fetch the full body.
   - Parses each into a `Finding` struct.
   - Test over a fixture with recorded `gh` output (JSON fixtures in
     `copilot/testdata/`).
4. Implement `Prompt() bool`:
   - Prompts on stdin: "N finding(s) from Copilot auto-reviews... [y/N]".
   - Returns false if stdin is not a terminal.
   - Tests: terminal stdin (expects y), piped input (n), empty stdin (n).
5. Add `copilot-leak` to `cmd/procoder/main.go` as a top-level command.
   - Flags: `--since`, `--quiet`, `--from-copilot` (stubs, wired in later).
   - On terminal: calls `Find`, prints count, calls `Prompt`.
   - If user declines: prints "skipped", exits 2.
   - If `--quiet`: prints count only, exits 0.
   - If no findings: prints "no findings since <duration>", exits 0.
   - If `gh` missing: NOT checked, exits 2.
   - If no GitHub remote: silently exits 0.

## Files

- `internal/copilot/copilot.go` — Find, Sanitise, Prompt
- `internal/copilot/sanitise_test.go` — sanitisation tests
- `internal/copilot/find_test.go` — find tests (fixtures)
- `cmd/procoder/main.go` — wire copilot-leak command (edit)
- `commands/copilot-leak.md` — usage doc
- `.opencode/commands/copilot-leak.md` — OpenCode twin

## Verification

`go test ./internal/copilot/...`
