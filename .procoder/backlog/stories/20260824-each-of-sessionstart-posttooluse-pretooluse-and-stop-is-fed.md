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

- [ ] Each of SessionStart, PostToolUse, PreToolUse and Stop is fed a
      real payload on stdin, and its output parses as the host expects —
      JSON envelope where the host wants one, raw text where it does not.
- [ ] Each hook is also fed empty stdin and a truncated payload, and
      neither produces a crash or a stack trace.
- [ ] The exit code of each hook matches what the host documents it
      reads, for both the clean and the blocking case.

## Evidence
