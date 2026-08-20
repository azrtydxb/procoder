# Add version check to SessionStart hook (non-blocking)

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Extend the principles SessionStart hook to check for version updates in the background. Warnings are printed to stderr (not captured by the hook's stdout), so they appear in the AI coder's output without corrupting the `additionalContext` JSON.

## Acceptance criteria

- [ ] `internal/principles/principles.go` `RunHook` spawns a goroutine for version check
- [ ] The goroutine has a 1-second timeout via `releases.Latest(root, 1*time.Second)`
- [ ] If newer version found, prints to stderr: `== procoder: newer version X.Y.Z is available (current: A.B.C) — run \`procoder self-upgrade\` to update`
- [ ] Goroutine does not block stdout or `RunHook`'s return
- [ ] If version check fails (network error, timeout): silently ignored (no stderr output — keep logs clean)
- [ ] `RunHook` still returns within the 3-second budget (goroutine runs independently)
- [ ] No race conditions: goroutine only writes to stderr (thread-safe)

## Evidence
