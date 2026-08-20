# Add `--check` flag to `version` command

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Modify the `procoder version` command to accept a `--check` flag that queries GitHub for the latest version, compares, and optionally upgrades.

## Acceptance criteria

- [x] `cmd/procoder/main.go` modified: the `"version"` case parses `--check` flag
- [x] Without `--check`: prints version string and exits 0 (existing behavior unchanged)
- [x] With `--check`: queries GitHub via `releases.Latest()` with 1-second timeout
- [x] If newer version found: prints warning `== procoder: newer version X.Y.Z is available (current: A.B.C)`
- [x] If TTY: prompts user with yes/no question (using existing `copilot.Prompt` pattern)
- [x] If user says yes: calls `upgrade.DownloadAndInstall(latest, out)`
- [x] If user says no / no TTY: exits 0 silently
- [x] If no newer version: prints "up to date" (or silent)
- [x] On network error: prints brief message to stderr, exits 0 (never fails)
- [x] Exit code 0 always (version check failure does not fail the gate)
- [x] Usage text updated: `version [--check]`

## Evidence

- `procoder version --check` prints the version, then GitHub's newest, then either "is the latest release" or the warning naming both versions.
- A check that could not run prints "the latest version is NOT known — <reason>" and exits 2. An unanswered check is never an up-to-date verdict.
- `[version] check = "off"` short-circuits it, printing where the setting lives.
- Live on a build stamped 0.9.0: warning printed, upgrade offered, declined cleanly.
- Live on a dev build: "this build carries no version, so there is nothing to compare", exit 0.
- TestUsageAndCoverageListAgree pins the flag's command against docs.Commands.
