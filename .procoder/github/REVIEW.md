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
  A failed Close/flush after a write IS a failed write.
- State computed twice that must agree (time.Now called twice across a
  boundary, a value re-derived instead of passed).
- Paths in OUTPUT (printed lines, error messages, test assertions on
  either) built with filepath.Join — output uses forward slashes on
  every platform (ToSlash); only real filesystem calls stay native.
- A file-discovery walk that keeps its own skip list instead of asking
  git what it ignores — gitignored trees (agent worktrees, vendored
  copies) are not this repository's content.
- A rule that can RAISE an obligation but not CLEAR it (or the reverse):
  exclusions must be symmetric, or the tool asks for something no action
  can satisfy.
- A benchmark that does not assert its work happened — an early return
  times at nanoseconds and reads as fast.
- A check whose question is about a CHANGE ("does a doc mention what you
  touched") reused over a whole-tree sweep, where it answers about
  everything and buries the real findings.
- A directory walk that swallows the ROOT's error: one unreadable
  subdirectory is skippable, an unreadable root is no survey at all, and
  "nothing there" must never read the same as "could not look".
- Loops doing per-iteration work that belongs outside (regex compilation,
  allocations, file opens).
- Temp files and permissions: CreateTemp over predictable names; modes no
  wider than needed.
- New surface wired everywhere it must appear: dispatch, usage text,
  canonical lists, docs, tests that pin them together. A function whose
  only callers are its own tests is not wired — green tests and real
  coverage make it read as live code; `procoder maintain` names these
  as "referenced only by tests".
- One concept, one owner: a path, format, or constant that two packages
  both declare will drift, and the drift is silent until one writes
  where the other no longer reads. The package that READS the file owns
  it, and a test carries a value across the seam.
- Parsers and scanners against hostile shapes: empty input, binary input,
  the terminator variants, the case the happy path skips.
- Test fixtures that trip our own scanners: assemble marker/secret-like
  content at runtime, never as a literal.
- Prose and markdown: code spans unbroken, lists formatted, wording that
  says what the code actually does — names and paths in docs match the
  code exactly (every variant, full paths).
- The product's story: does this diff change what the README or docs
  site must tell a reader? Pages updated in this diff, or the absence
  concretely justified — a feature that ships with only a reference
  mention is a docs escape.
- Docs voice: professional product documentation — no personal names,
  no project-history anecdotes, no first-person diary; history belongs
  in the changelog and lessons ledger, not the docs.
- A third-party tool's semantics come from its own help or docs, not from
  what the syntax looks like it should mean. `gh`'s `path#text` sets an
  asset's display LABEL; the name still comes from the file. One line of
  `--help` separates a working release job from a red one.
- Guards test the property they claim to test. `-s` is size, not
  content; a zero exit is "the command ran", not "the command found
  what you wanted"; a non-nil slice is not a non-empty one. A guard that
  passes on the input it exists to reject is worse than no guard,
  because the message beside it says the case is handled.
- Rendered output was LOOKED AT. A change to CSS, client-side JS, a
  diagram, a template, or Markdown that renders as a table or a figure
  is not reviewed until someone has seen it rendered — both colour
  schemes where the site has two. A passing build and a green assertion
  both report on what the code says, not on what the reader sees; when
  the two disagree, the screenshot is right.

End with a verdict line: findings counted by severity, or exactly
"Nothing found — open the PR."
