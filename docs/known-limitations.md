# Known limitations

procoder gates other people's code. That obligates candour about where the
gate is weak. Every entry below was verified against the source cited next to
it, or by running the tool, as of 0.1.0. Where a previous edition of this page
disclosed something that has since been fixed, the entry is gone rather than
kept as false modesty — a document that overstates weakness is as untrustworthy
as one that hides it.

## Heuristic scanning, not parsing

`hooks/checks/shape.js` is a brace/indent counter, not a parser for any of the
six languages it measures. Verified consequences:

| Case | Effect |
|---|---|
| A `#` anywhere on a line in a brace language | `stripNoise` blanks from the `#` to end of line for every language, after comments and strings are already blanked. A JS/TS private class member (`#count`, `#bump(a) {`) therefore erases the rest of its own line, including a block-opening `{`. Verified: `#bump(a, b, c, d, e, f) {` gets no params/length/complexity finding at all, while the identical `bump(...)` does; a class containing a private method measures nesting depth 5 where the public twin measures 6. C# `#region`/`#if` on their own lines are harmless (no braces follow), Rust `#[attr]` likewise. |
| Signature wrapped over more than `SIGNATURE_LOOKBACK` (10) lines, or spanning more than `SIGNATURE_MAX_CHARS` (1000) | The rescan gives up and the function is not measured. Ten lines reaches a seven-parameter wrap at one parameter per line, so the bound is past every useful `params` threshold — but a signature wrapped further is invisible to all three function-shape rules. Python's `def` rescan (`hooks/checks/lang/py.js`) uses the same 10-line bound. |
| Parameter list longer than the packs' 500-character span ceiling | Not measured. Verified: a 60-parameter function (1128 characters of parameters) on one line produces no finding, while a 6-parameter one does. The ceiling loses precisely the most extreme functions. |
| Mixed tab/space indentation in Python | Depth is counted as *changes* of indentation rather than as multiples of `tabWidth`, but the column it compares is still tab-expanded at `tabWidth`. So a narrow level nests correctly (2-space levels interleaved with tab levels measured 4, not 2), while a level whose expanded column is not greater than its enclosing line's still collapses — a tab-indented block inside an 8-space block measures as no deeper. Verified: a 5-level function measured 5 with consistent indentation of either kind, 3 when tab and 4-space levels alternate. Python 3 rejects such a file outright (`TabError`), so there is no correct tab width to recover. |

Swapping in a real per-language parser is the fix; it has not happened in
0.1.0.

## Meta-text: the line marker, and what it cannot cover

Text that *describes* a violation reads to a regex exactly like the violation.
The mechanism for that is a line marker (`hooks/checks/patterns/markers.js`,
applied in `hooks/checks/universal.js` for every pack):

`<comment syntax> procoder: literal <rule-id>[, <rule-id>…] <reason>` <!-- procoder: literal alone/blanket-suppression the marker syntax written out, not a suppression -->

Trailing on a line, it covers that line; standing alone on its own line, it
covers that line and the next. It must name its rules and give a reason of at
least two words, or it parses as a blanket or unexplained suppression and
silences nothing. Limits, all verified:

| Limit | Effect |
|---|---|
| Rule ids must match `[a-z]+/[a-z-]+` | An external linter's finding id carries the tool's own rule id with a colon and often digits — `true/eslint:no-eval`, `true/ruff:E501`. Those cannot be named. The marker still parses as a named suppression, so nothing is reported: it is a **silent no-op**. Use `[exclude] rules` in `.procoder.toml` for those. |
| Rule ids are never validated | A typo (`safe/hardcoded-secrets`) silences nothing and says nothing. |
| Reach is one line, or two for the standalone form | There is no block or file form, deliberately. A fixture file that is nothing but violating input cannot be marked line by line without changing the input — `tests/fixtures/` is excluded by path in `.procoder.toml` for exactly that reason, and that is the one case the marker cannot serve. |
| No wildcard | There is no bare or `*` form; the bare form is routed to `alone/blanket-suppression` and reported. |

## The self-scan, and what is held out

`tests/dogfood.test.js` derives its file list from `git ls-files`, so a file is
in the gate the day it lands. Two categories are held out:
`docs/superpowers/` (planning documents for work already executed) and
`examples/*/before.*` (files whose documented purpose is to violate a rung,
paired with an `after.*` that does not; `examples/README.md` and every
`after.*` stay in the scan).

Measured today, `procoder check` over the whole tracked tree reports **66**
findings: 56 in `docs/superpowers/`, 10 in `examples/*/before.*`, and none
anywhere else. The scan minus the two hold-outs is clean, and that is the hard
gate the test enforces.

What this does *not* give you:

- The two `HELD_OUT` entries have no expiry test. Only the `PENDING` list —
  temporary hold-outs, currently empty — is checked for still having the
  finding that put it there. A `HELD_OUT` path that went clean would not be
  noticed.
- `docs/superpowers/` is held out because procoder still cannot tell a
  document quoting a violation from a file committing one, at the scale of a
  1500-line planning document. The line marker solves that case by case; it
  was not applied to historical documents.

## `cargo clippy` cannot be scoped to one file

Verified in `hooks/checks/registry.js` and `hooks/checks/resolve.js`. Every
other supported external linter (eslint, ruff, golangci-lint) takes a single
file as its target. `cargo clippy` has none — it always compiles and lints the
whole crate. The hook still calls it with a file-scoped timeout (half the
hook's budget, clamped to 250–1500ms), so on anything but a trivial crate it
will usually not finish. When it times out the hook falls back to the built-in
Rust pack rather than reporting nothing, so the file still gets its SAFE/TRUE
and shape checks — just not clippy's lints, on most edits to a real crate.
`parse()` additionally discards any finding whose reported path is not the file
being written, so a warning in `src/other.rs` is never attributed to it.

## The ratchet baseline: what it guarantees, and what it doesn't

`hooks/checks/baseline.js` fingerprints each finding from its rule id, file
path, normalized source line text, and an occurrence ordinal — deliberately
excluding the line number.

Holds:

- Reindenting a file, or moving a suppressed line up or down, does not
  resurrect a suppressed finding.
- Copy-pasting a suppressed line does not ride in for free: the occurrence
  ordinal gives each copy its own identity, so only the first is suppressed.

Does not hold:

- **A re-wrapping formatter breaks it.** The fingerprint is over the finding's
  own line text, so a formatter that splits a statement across lines changes
  that text and the accepted finding comes back as new. Re-baseline after a
  format sweep.
- **Moving code between files breaks it.** The path is part of the
  fingerprint; a relocated violation is a brand-new finding, not a tracked
  move.
- **File-level findings key on line 1.** `obvious/nesting-depth` is reported at
  line 1, so its fingerprint is over whatever the file's first line says. Edit
  line 1 — a copyright header, an import — and the accepted finding returns.

## The baseline format is versioned, and old baselines are not migrated

`BASELINE_VERSION` in `hooks/checks/baseline.js` is 2; v1 fingerprints had no
occurrence ordinal and cannot be reconstructed. A stale-version file loads as
an empty accepted set, never a partial one. In `bin/procoder.js`:

- `procoder verify` exits **2**, distinct from the **1** used for "the ratchet
  grew", and says the baseline is the wrong format — CI does not blame the user
  for a backlog they did not add.
- `procoder check` prints the same stderr notice but still reports every
  finding in the repo, because nothing is suppressed. The exit code is the
  ordinary one.
- `procoder baseline` always overwrites with a current-version file, so running
  it is the fix in both cases.

## The TOML parser is a documented subset

`hooks/checks/toml.js` supports `[tables]`, `[dotted.tables]`,
string/int/float/bool values, and arrays of strings — single-line or spanning
multiple lines, with trailing commas and comments inside the array. Verified
working: multi-line arrays, and a comma inside a quoted item
(`paths = ["a, b", "c"]` yields two entries, not three).

Where it still falls short:

- A line the parser cannot recognize at all is **warned on stderr** with file
  and line, then skipped — not silently dropped. Likewise an array that never
  closes.
- An unsupported *value* on a well-formed `key = value` line is **not**
  warned. `parseValue` falls through to the raw text, so an inline table
  becomes the string `{a=1}`, a date becomes the string `1979-05-27`, and a
  multi-line string keeps only its first line. Nothing errors; the config loads
  with a value the author did not intend.
- Keep `.procoder.toml` to the documented subset, and check stderr after
  editing it.

## Deliberate narrowings from performance work

Every long-line path was made linear, each proven behaviour-identical by
differential runs. The narrowings that survived are real, and disclosed here:

| Narrowing | Where | Effect |
|---|---|---|
| 500-character regex spans | the six language packs | A sink whose interpolation, argument list or parameter list runs past 500 characters is not matched. |
| 200-character spans (`SPAN_MAX`) | `hooks/checks/universal.js` | Same, for interpolated log arguments and commented-out-code detection. |
| 20 findings per line | `hooks/checks/run.js` | Overflow is reported as its own finding (`true/findings-suppressed`) naming the count, never silently dropped. No honest line reaches 20. |
| 4KB per-line guard, shape path only | `hooks/checks/run.js` | Function length, params, complexity and nesting are not measured on a line over 4KB. The language packs' SAFE rules and the universal pack read it unguarded — verified: an AWS key and an `eval(` on the same 11KB minified line are both reported. |
| 4MB per-file skip | `hooks/checks/run.js` | Larger files are not checked at all. The CLI says so on stderr. |
| 2s budget | `hooks/checks/run.js` | Checks after the deadline are skipped. The universal pack (rung 1) runs first, before any external linter, so it never loses its budget to one. |

## The PostToolUse hook only reports near the edit

`hooks/procoder-check.js` narrows the language packs' findings to the region
the tool call touched, ±3 lines. The universal pack is exempt — a credential is
a leak wherever it sits — but a file-level finding reported at line 1
(`obvious/nesting-depth`) will not surface from an edit made further down. Run
`procoder check <file>` or `/procoder:review` for the whole-file picture.

## External linter deference

When a project has its own linter configured (`hooks/checks/resolve.js`), that
linter's findings replace the pack's `obvious/*` rules for files it answers
for. It never replaces the SAFE rules. A linter that times out or crashes
counts as not having answered, so the pack covers the whole file — a broken
tool degrades to less coverage, never to silent passing.

## Config keys

`.procoder.toml`'s supported keys are documented in `README.md`, not
duplicated here.
