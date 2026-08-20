# auto-copilot-leak: command shell, sanitisation, and prompt

Status: done 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20
Sprint: 006-auto-copilot-leak-capture-copilots-auto-review-findings-as

## Description

Implement the `copilot-leak` command shell and the sanitisation layer.
No issue creation yet. This story delivers the foundation: a new
`internal/copilot` package with `Find`, `Sanitise`, and `Prompt`
functions, wired to the CLI under `cmd/procoder/main.go`.

## Acceptance criteria

- [x] `internal/copilot` provides Find, Sanitise and Prompt, and `copilot-leak` is a top-level command in the usage text.
- [x] Sanitise strips fenced, indented and HTML code, redacts every secret shape, and emits no path outside the repository root.
- [x] Find matches Copilot auto-review issues by author, label or body marker, honours --since, and reports NOT checked when gh is missing or its output is unparseable.
- [x] Prompt refuses without a terminal and treats anything but yes as no.
- [x] `copilot-leak` answers plainly with no findings, and outside a GitHub repository stays quiet.
- [x] `commands/copilot-leak.md` exists with its OpenCode and Kilo twins.

## Evidence

- `go test ./internal/copilot/` green — sanitise_test.go, find_test.go, capture_test.go and adversarial_test.go, ~50 cases including unclosed HTML tags, quoted fences and sibling paths that start with the root's name.
- Live: `procoder copilot-leak --quiet` → 'copilot-leak: no findings since 24h0m0s', exit 0.
- Usage lists `copilot-leak`; TestUsageAndCoverageListAgree pins it against docs.Commands.
- commands/copilot-leak.md, .opencode/command/copilot-leak.md and .kilo/commands/copilot-leak.md all present; TestKiloCommandParity green.
