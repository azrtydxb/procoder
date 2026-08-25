# SessionStart is a hook, even though it does not say `hook`

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

The trap this story exists for. SessionStart is wired as `launcher.sh
principles --hook` — not `hook <sub>` — so a split that recognised only
the first shape would refuse loudly at session start on any machine that
could not fetch. The mechanism written to keep sessions alive would have
broken them at the one moment it was written for.

An invocation is a hook when its first argument is `hook` OR any argument
is `--hook`. Both shapes are wired today and both are tested, so a future
narrowing fails rather than ships.

## Acceptance criteria

- [x] `launcher.sh principles --hook` with no binary and no network exits
      0 and writes nothing to stdout.
- [x] Every invocation named in `hooks/claude-hooks.json` is covered by a
      case, so the test fails if a new hook shape is wired without being
      considered here.

## Evidence

- The `principles --hook` subtest of
  `TestEveryWiredHookShapeDegradesInsteadOfBreakingTheSession`: exit 0,
  stdout empty.
- Every shape in `hooks/claude-hooks.json` is a subtest, so wiring a new
  hook without considering this split fails here rather than in somebody's
  session.
- **proved by: deleting the `--hook` arm from the `is_hook` test — the
  principles subtest exits 1 with the failure on stderr.** That mutation
  was run: this is the trap the story was written for, and it was real.
