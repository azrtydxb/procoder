# Add version check to SessionStart hook (non-blocking)

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Extend the principles SessionStart hook to check for version updates in the background. Warnings are printed to stderr (not captured by the hook's stdout), so they appear in the AI coder's output without corrupting the `additionalContext` JSON.

## Acceptance criteria

- [x] `internal/principles/principles.go` `RunHook` spawns a goroutine for version check
- [x] The goroutine has a 1-second timeout via `releases.Latest(root, 1*time.Second)`
- [x] If newer version found, prints to stderr: `== procoder: newer version X.Y.Z is available (current: A.B.C) — run \`procoder self-upgrade\` to update`
- [x] Goroutine does not block stdout or `RunHook`'s return
- [x] If version check fails (network error, timeout): silently ignored (no stderr output — keep logs clean)
- [x] `RunHook` still returns within the 3-second budget (goroutine runs independently)
- [x] No race conditions: goroutine only writes to stderr (thread-safe)

## Evidence

- internal/principles.RunHook starts the check before assembling the payload and reports it after — the check runs alongside the session start, never in front of it (N-03).
- TestTheVersionCheckNeverHoldsTheSessionOpen: against a server that never answers in time, the hook still returns in well under the budget and prints its payload; a check that did not answer says nothing at all.
- TestTheVersionWarningStaysOutOfTheHookPayload: the warning reaches stderr and never the stdout payload, which three of the four hosts parse as JSON (R-07).
- TestTheConfigKnobSilencesTheCheckEntirely: `[version] check = "off"` asks GitHub nothing — the stub server fails the test if it is called.
- R-08: with no terminal the warning still prints and no prompt is sent.
