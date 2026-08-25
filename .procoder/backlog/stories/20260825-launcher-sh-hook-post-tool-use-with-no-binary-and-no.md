# A hook that cannot fetch degrades instead of breaking the session

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

A hook fires inside somebody's editing session. If it fails hard
because a laptop is on a plane, procoder has broken the thing it exists to
help.

So a hook that cannot get its binary writes the reason to stderr, writes
nothing at all to stdout, and exits 0. No stdout is "no decision" to
PreToolUse and "no context" to PostToolUse — both safe, both visible. The
user can see the gate is not running; they are not stopped from working.

## Acceptance criteria

- [x] `launcher.sh hook post-tool-use` with no binary and no network
      writes nothing to stdout, writes the reason to stderr, and exits 0.
- [x] The same holds for `hook pre-tool-use` and `hook stop`, so no
      registered hook can take a session down.

## Evidence

- `TestEveryWiredHookShapeDegradesInsteadOfBreakingTheSession` covers
  `hook post-tool-use`, `hook pre-tool-use`, `hook stop` and `principles
--hook` — every invocation wired in `hooks/claude-hooks.json`. Each
  exits 0, writes nothing to stdout, and says on stderr that the gate is
  not running.
- Empty stdout is deliberate: it is "no decision" to PreToolUse and "no
  context" to PostToolUse, so the host proceeds normally.
- proved by: making `give_up` exit 1 for hooks — every subtest then
  reports the exit code the host stops a session on.
