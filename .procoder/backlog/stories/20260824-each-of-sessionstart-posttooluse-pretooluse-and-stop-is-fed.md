# The hooks, fed real payloads on stdin

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 016-the-hooks-fed-real-payloads-and-the-docs-held-to-the-binary

## Description

The hooks are how procoder reaches a repository that never types
`procoder`, and they are the surface least covered by the suite, because
the suite calls the functions rather than the process. A hook that
returns the right verdict in a unit test and the wrong envelope on stdout
is broken everywhere it matters.

Each of SessionStart, PostToolUse, PreToolUse and Stop is fed a real
payload on stdin — the shape the host actually sends — and its output is
parsed the way the host parses it. JSON where the host wants JSON, raw
text where it does not, exit code where the host reads one.

Malformed input is part of the contract: a truncated payload, an empty
stdin, a payload naming a file that does not exist. A hook that crashes
on those takes the host's session with it.

## Acceptance criteria

- [x] Each of SessionStart, PostToolUse, PreToolUse and Stop is fed a
      real payload on stdin, and its output parses as the host expects —
      JSON envelope where the host wants one, raw text where it does not.
- [x] Each hook is also fed empty stdin and a truncated payload, and
      neither produces a crash or a stack trace.
- [x] The exit code of each hook matches what the host documents it
      reads, for both the clean and the blocking case.

## Evidence

- `scripts/e2e-hook-pass.sh`: **20 assertions, 0 failures**, over the five
  registrations in `hooks/claude-hooks.json` — SessionStart (`principles
--hook`), PostToolUse, PreToolUse, Stop and PreCompact (which shares
  `hook stop`).
- PreToolUse on a `git commit` returns `hookSpecificOutput` with
  `permissionDecision: "deny"`, `hookEventName: "PreToolUse"`, and a
  `permissionDecisionReason` naming the unformatted file and the command
  that fixes it. On `ls -la` it returns no decision at all and exits 0.
- PostToolUse returns `additionalContext` carrying gofmt's output and the
  sentence "the file itself was NOT modified" — and the file, read back
  afterwards, is unmodified. P-CONTROL holds at the process boundary,
  which is the only place a host can see it.
- The assertions read the envelope's fields rather than grepping its text.
  The first version used `grep -qi 'deny\|block'`, which passes on an
  ALLOW just as happily, because "block" sits inside "1 blocking
  finding(s)" whichever way the decision went.
- Empty stdin, a truncated payload (`{"tool_name":"Write","tool_inp`), and
  a payload naming a file that does not exist: none of the three hooks
  exits above 2 and none prints a panic or a goroutine dump.
- **Exit codes against the host contract:** Claude Code reads a PreToolUse
  decision from either the JSON envelope with exit 0 or exit 2 with a
  reason on stderr. procoder takes the first path, so the assertion
  accepts either and requires one of them — silence with exit 0 fails it.
  SessionStart and Stop exit 0, which is what a non-error hook must do or
  the host reports a startup failure.
- **Two mutations, both run and both fatal.** Forcing the gate's verdict to
  "allow" in `internal/hook/commit.go` fails the denial assertion
  (`decision=allow`). Emptying `AdditionalContext` in `internal/hook/hook.go`
  fails three. An earlier attempt to mutate the fixture's `config.toml`
  proved nothing, because the pass rebuilds the fixture before it starts
  and wiped the file — a mutation that leaves no diff is not a mutation.
