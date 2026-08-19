# Pre-PR review rubric

A fresh-context reviewer (a subagent, not the author) reads the full
branch diff against this list BEFORE the PR is opened. The author fixes
Critical/Important findings first; downstream reviewers are the fallback,
not the net. Findings name file:line, what breaks, and the fix.

Check every hunk for:

- User-supplied strings reaching a path, command, or query — validated as
  the plain value they claim to be (no separators, no dot-dot, quoted)?
- Error paths: any error swallowed, any unreadable input silently
  skipped, any failure reported as success? Honesty beats convenience.
- State computed twice that must agree (time.Now called twice across a
  boundary, a value re-derived instead of passed).
- Loops doing per-iteration work that belongs outside (regex compilation,
  allocations, file opens).
- Temp files and permissions: CreateTemp over predictable names; modes no
  wider than needed.
- New surface wired everywhere it must appear: dispatch, usage text,
  canonical lists, docs, tests that pin them together.
- Parsers and scanners against hostile shapes: empty input, binary input,
  the terminator variants, the case the happy path skips.
- Test fixtures that trip our own scanners: assemble marker/secret-like
  content at runtime, never as a literal.
- Prose and markdown: code spans unbroken, lists formatted, wording that
  says what the code actually does.

End with a verdict line: findings counted by severity, or exactly
"Nothing found — open the PR."
