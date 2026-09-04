# command-api Task 1: the session, not the process, says where and who

Status: closed
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

- [x] `host.DetectIn(env)` reads only the map it is given:
      `TestDetectInReadsOnlyItsArgument` passes with a conflicting
      `CLAUDE_PLUGIN_ROOT` set in the process environment.
- [x] `run(args, session)` writes to the session's buffers and not to
      `os.Stdout`: `TestRunUsesTheSessionNotTheProcess` passes.
- [x] No `os.Getwd`, `os.Getenv`, `os.Stdin`, `os.Stdout`, `os.Stderr` or
      `fmt.Println` remains inside `run` or the functions it calls in
      `cmd/procoder`, except in `processSession`.
- [x] `go test ./...` passes and `procoder check` reports 0 blocking.

## Evidence

`go test ./internal/host/` — ok procoder/internal/host. With
`CLAUDE_PLUGIN_ROOT` set to a VS Code Copilot path in the process,
`DetectIn(Env{"QODER_SESSION_ID": "x"})` returns Qoder.

`go test ./cmd/procoder/` — ok. `TestRunUsesTheSessionNotTheProcess` runs
`config` against a temp repository with os.Stdout swapped for a pipe: the
session's buffer is non-empty and the pipe read back zero bytes.

`grep -n "os.Getwd\|os.Getenv\|os.Stdin\|os.Stdout\|os.Stderr\|fmt.Println"
cmd/procoder/main.go` — the only hits are inside processSession and two
comment lines. Eight fmt.Println calls that bypassed the session were
rewritten to s.out.

`go test ./...` — no failures. `procoder check` — 0 blocking (the
documentation obligation cleared by the commit's `docs: none` line).

Committed as 6747c9f.
