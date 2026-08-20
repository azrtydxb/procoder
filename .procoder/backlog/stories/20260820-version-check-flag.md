# Add `--check` flag to `version` command

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Modify the `procoder version` command to accept a `--check` flag that queries GitHub for the latest version, compares, and optionally upgrades.

## Acceptance criteria

- [ ] `cmd/procoder/main.go` modified: the `"version"` case parses `--check` flag
- [ ] Without `--check`: prints version string and exits 0 (existing behavior unchanged)
- [ ] With `--check`: queries GitHub via `releases.Latest()` with 1-second timeout
- [ ] If newer version found: prints warning `== procoder: newer version X.Y.Z is available (current: A.B.C)`
- [ ] If TTY: prompts user with yes/no question (using existing `copilot.Prompt` pattern)
- [ ] If user says yes: calls `upgrade.DownloadAndInstall(latest, out)`
- [ ] If user says no / no TTY: exits 0 silently
- [ ] If no newer version: prints "up to date" (or silent)
- [ ] On network error: prints brief message to stderr, exits 0 (never fails)
- [ ] Exit code 0 always (version check failure does not fail the gate)
- [ ] Usage text updated: `version [--check]`

## Evidence
