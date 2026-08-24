# the hooks fed real payloads, and the docs held to the binary

Status: active
Created: 2026-08-24

## Goal

Test the two surfaces the suite cannot reach: the process boundary, and
the prose.

The hooks are how procoder reaches a repository where nobody types
`procoder`, and they are the least covered thing it ships — because the
suite calls the functions while the host runs the process. A hook that
returns the right verdict in a unit test and the wrong envelope on stdout
is broken everywhere it matters. Each of SessionStart, PostToolUse,
PreToolUse and Stop gets a real payload on stdin, and its output is parsed
the way the host parses it. Empty stdin and a truncated payload are part
of the contract: a hook that crashes on those takes the session with it.

The docs are the other half. `docs/commands.md` is written by hand and
correct exactly as often as somebody remembered, and it is the first thing
an adopter reads. Every documented command gets invoked and its actual
behaviour compared with the documented one — flags, exit codes, whether it
writes anything — with each disagreement recorded against the docs or the
binary by name. Fixing the binary to match a sentence somebody wrote is
how a documentation error becomes a behaviour regression, so which side is
wrong gets decided before anything changes.

Sprint 015 found three commands missing from `procoder help` by comparing
the dispatch against the docs. This sprint does that comparison properly,
in both directions, for what each command actually does rather than
whether it is mentioned.
