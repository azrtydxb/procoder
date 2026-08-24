# the hooks fed real payloads, and the docs held to the binary

Status: closed 2026-08-24
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

## Result

committed: 2
done: 2 (20260824-each-of-sessionstart-posttooluse-pretooluse-and-stop-is-fed, 20260824-every-command-documented-in-docs-commands-md-is-invoked-and)
carried: 0

## Retro

**Two checks passed for a sprint without testing anything, and both were
mine.** The P-CONTROL loop ran `procoder format` only over files that were
already clean, so the branch that prints a whole rewritten file never
executed — the single branch P-CONTROL exists to police. The hook
assertions grepped for "deny" or "block", and "block" sits inside "1
blocking finding(s)" whichever way the decision went, so the check passed
on an allow. Neither was visible in the output: both printed PASS, in a
report with no failures.

**What found them both was the mutation, and only the mutation.** Reading
the assertions did not; they look right. Running the code with the
behaviour deliberately broken did. The rule this sprint hardens: an
assertion is not finished when it passes, it is finished when it has been
watched to fail.

**A mutation has to reach the branch.** The first format mutation applied
cleanly, compiled, and changed nothing observable, because every file the
loop touched was already formatted. That is the same failure as a mutation
that produces no diff, one layer further in — the diff existed, the
execution path did not. Before trusting a green mutation run, check that
the mutated line can actually be reached by the fixture as built.

**Never edit a script while it is running.** bash reads incrementally from
a byte offset, so rewriting the file underneath a live invocation moved
what the next read returned; the docs pass executed its P-CONTROL block
twice and reported 73 passes instead of 50. A count that is too HIGH reads
as better news, which is why it nearly stood. Every harness here now has a
known maximum, and the total is checked against it.

**The adaptation worth keeping: know what the maximum possible count is.**
18 flags + 1 coverage + 6 exit codes + 28 digests = 53. A report that says
53 is complete; one that says 73 is broken; one that says 3 stopped early.
Without the arithmetic all three look like success.
