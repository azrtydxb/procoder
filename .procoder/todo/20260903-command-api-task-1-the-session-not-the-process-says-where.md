# command-api Task 1: the session, not the process, says where and who

Status: open
Created: 2026-09-03

## Description

Task 1 of `.procoder/plans/command-api.md`. Every command's working
directory and environment come from the process today: `doctor.Root()`
calls `os.Getwd`, `host.Detect()` calls `os.Getenv`, and `run` writes
through a package-level `printLine`. A daemon serving several sessions
cannot answer correctly through any of them — its own cwd and environment
are whichever session started it.

Done means `run` takes a session — stdin, stdout, stderr, cwd, env — and
reaches for no process global, so the CLI and a request envelope can each
construct one. No command's behaviour changes.

## Acceptance criteria

- [ ] `host.DetectIn(env)` reads only the map it is given:
      `TestDetectInReadsOnlyItsArgument` passes with a conflicting
      `CLAUDE_PLUGIN_ROOT` set in the process environment.
- [ ] `run(args, session)` writes to the session's buffers and not to
      `os.Stdout`: `TestRunUsesTheSessionNotTheProcess` passes.
- [ ] No `os.Getwd`, `os.Getenv`, `os.Stdin`, `os.Stdout`, `os.Stderr` or
      `fmt.Println` remains inside `run` or the functions it calls in
      `cmd/procoder`, except in `processSession`.
- [ ] `go test ./...` passes and `procoder check` reports 0 blocking.

## Evidence

<!-- Filled at close time. -->
