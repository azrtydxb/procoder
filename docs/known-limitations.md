# Known limitations

procoder gates other people's code. That obligates candor about where the
gate is weak. This page lists the failure modes that are known and
unresolved as of 0.1.0 — verified by reading the source cited next to each
entry, not by report or assumption.

## Heuristic scanning, not parsing

`hooks/checks/shape.js` is a brace/indent counter, not a parser for any of
the six languages it measures. Specific, verified consequences:

| Case | Effect |
|---|---|
| `case`/`default` label immediately followed by `{` (`case 1: {`) | The block's own brace is misread as a data literal (the code that classifies a `{` looks at the text right before it, and a trailing `:` reads the same whether it introduced an object or a label), so it does not add a nesting level. A three-deep `if` inside a `case` block is reported as two deep. |
| Function/method signature that spans more than one line | The function is dropped from `function-too-long` / `too-many-params` / `complexity` entirely — not merely mis-attributed. The signature match and the block-open brace land on different line numbers, they never coincide, and the pack only measures blocks whose start line has a matching signature. A 200-line function with a wrapped parameter list gets no shape finding at all. |
| Mixed tabs and spaces in Python | Indent depth is computed by expanding tabs to a fixed width and dividing by that width, so a file that mixes tab- and space-indented blocks produces a depth number that does not track the interpreter's actual block structure. Verified: a block nested 4 levels deep with tabs alone measures 4; the same structure with spaces at odd levels and tabs at even ones measures 2.

None of these are close to a full parse — they are the shape of the false
positive/negative space a regex-based scanner has, by construction. Swapping
in a real per-language parser is the fix; it hasn't happened yet in 0.1.0.

## Line-oriented rules, and what fixes and doesn't

<!-- procoder: literal alone/orphan-todo the marker this sentence quotes as an example, not one left in the tree -->
A comment reading `// TODO: reduce Math.random() usage` can trip the
weak-random rule: most rules in the language packs (`hooks/checks/lang/*.js`)
match against raw source lines, so a rule name or trigger phrase mentioned in
a comment or a string literal can produce a finding.

`ts.js` runs two of its rules (`safe/xss-sink`, `obvious/nested-ternary`)
against comment-and-string-stripped lines instead. That closes the
comment/string false-positive class for those two rules specifically — but
the same stripping blanks out template-string (backtick) contents, so a real
sink written *inside* a template literal (for example, an HTML fragment
built with a live `.innerHTML =` assignment interpolated into a larger
template string) is invisible to the stripped-line check. The other
line-rules in `ts.js` (SQL injection, shell injection, dynamic eval, TLS
disabled, weak random, debug leftovers) still run on raw lines and keep the
comment/string false-positive exposure that stripping would have removed,
because some of them (SQL injection) need to see the raw `${` interpolation
inside a template string to fire at all. This is a trade the code makes
deliberately in each direction, not an oversight, but neither side is free.

Rust, Go, Python, JVM and .NET packs run their line rules on raw lines
throughout — none of them do the comment-stripped pass ts.js does for those
two rules.

## The self-scan is scoped, and that scope hides a real gap

`tests/dogfood.test.js` runs procoder against its own source, but only over
`hooks/`, `bin/`, and `scripts/` — not `tests/`, `skills/`, `docs/`, or
`.procoder.toml`. The reason is not to make the number look good: those
excluded paths are full of *text that describes violations* rather than
violations — test fixtures containing real-looking AWS keys, the doctrine
teaching what a bad suppression comment looks like, `.procoder.toml` naming
the rule IDs it excludes, planning documents that quote example code with
fake secrets. procoder cannot currently tell "this line is an AWS key" from
"this line is a test fixture asserting that procoder detects an AWS key."

That is a real, unsolved false-positive class, not a subset chosen to flatter
the tool. Running the same check over the whole repository confirms the
size of it:

```
$ node bin/procoder.js check .
procoder: 185 findings.
```

Of those 185, 106 are in `tests/` (fixtures and assertions that intentionally
contain the exact patterns the rules look for), 56 are in `docs/`
(specification and planning documents that quote credential-shaped example
code), 10 are in `skills/` (the doctrine text itself), 7 are in `examples/`
(the before/after rung demonstrations, which are supposed to contain
violations), and the rest are single findings scattered across
`procoder-mcp/`, `package.json`, `commands/`, `README.md`, and
`.procoder.toml` for the same reason. procoder has not solved "distinguish
code from text about code," and the scoped dogfood gate is a workaround, not
a fix.

## External tool integration: `cargo clippy` cannot be scoped to one file

Verified in `hooks/checks/registry.js` and `hooks/checks/resolve.js`, current
as of this release. Every other supported external linter (eslint, ruff,
golangci-lint) takes a single file as its target. `cargo clippy` has no
per-file invocation — it always compiles and lints the whole crate. The hook
still calls it with a file-scoped timeout (half the hook's overall budget,
capped between 250ms and 1500ms), so on anything but a trivial crate clippy
will usually not finish before that timeout on a real crate. When it times
out, the hook falls back to the built-in Rust pack rather than reporting
nothing — a broken or slow external tool degrades to less coverage, never to
silent passing — so the file still gets `SAFE`/`TRUE` checks and the
built-in shape rules, just not clippy's lints, on most edits to a non-trivial
crate.

## The ratchet baseline: what it guarantees, and what it doesn't

`hooks/checks/baseline.js` fingerprints each finding from its rule ID, file
path, normalized source line text, and an occurrence ordinal — deliberately
excluding the line number. Verified properties:

- Reformatting a file (reindenting, moving a suppressed line up or down)
  does not resurrect a suppressed finding, because the fingerprint doesn't
  depend on line number.
- Cloning a violation (copy-pasting a suppressed line elsewhere in the same
  file) does not ride in for free behind the one accepted entry — the
  occurrence ordinal in the fingerprint gives each copy its own identity, so
  only the first occurrence is suppressed and every further copy is a new,
  gated finding.

What it does not do: the fingerprint has no notion of "this violation moved
from file A to file B." A finding relocated to a different file changes its
path component and is treated as a brand-new finding, not a tracked move —
which means moving code between files to reorganize it can turn a suppressed
finding into a red CI run even though nothing about the violation itself
changed.

## The baseline format is versioned, and old baselines are not migrated

`BASELINE_VERSION` in `hooks/checks/baseline.js` is 2; version 1 fingerprints
had no occurrence ordinal and cannot be reconstructed into the v2 shape.
Loading a stale-version baseline file yields an empty accepted set rather
than a partial one. `bin/procoder.js`:

- `procoder verify` on a stale baseline exits **2**, distinct from the exit
  **1** used for "the ratchet grew" — the message states the baseline is the
  wrong format and asks for a re-run of `procoder baseline`, rather than
  reporting a findings delta that would look like new debt the user
  introduced.
- `procoder check` on a stale baseline does not get this special case: the
  baseline loads as empty and every existing finding in the repo is reported
  as if the repo had never been baselined at all.
- `procoder baseline` always overwrites with a current-version file, so
  running it is the fix in both cases.

## The TOML parser is a documented subset, and its failure mode is silent

`hooks/checks/toml.js` implements exactly what `.procoder.toml` needs:
`[tables]`, `[dotted.tables]`, string/int/float/bool values, and single-line
arrays of strings. Verified behavior on the two edge cases that matter most
for a config that controls what gets excluded from a security gate:

- **A multi-line array** (`paths = [\n  "a",\n  "b",\n]`) is not parsed as an
  array at all. The key/value line regex requires the whole assignment on
  one line, so the first line (`paths = [`) fails to match a complete
  key-value pair or, depending on exact formatting, is captured as a raw
  string starting with `[` and never closing. Either way the array silently
  fails to become the exclusion list the author intended.
- **A comma inside a quoted array item** (`paths = ["a, b", "c"]`) is parsed
  wrong: the array splitter does a plain `split(',')` with no awareness of
  quotes, so `"a, b"` becomes two array entries (`"a` and `b"`) instead of
  one.

Both failure modes are **silent** — nothing errors, nothing is logged, the
config file loads "successfully" with a value the author did not intend.
This is the dangerous direction for a security tool: a silently-mis-parsed
`exclude.paths` entry can either fail to exclude a path the author meant to
exclude (noisy, but safe) or, if the malformed array happens to still
resolve to path-shaped strings, exclude paths the author never listed. Any
config using multi-line arrays or a comma inside a quoted path should be
rewritten onto one line per array, comma-free, until this parser is replaced
with a real TOML library — which the file's own header comment already
flags as the intended fix if the config's needs grow past this subset.

## Config keys

`.procoder.toml`'s supported keys (`exclude.paths`, `exclude.rules`,
`thresholds.function_lines`, `thresholds.nesting_depth`, `thresholds.params`,
`thresholds.complexity`, `baseline.file`) are documented in `README.md`, not
duplicated here.
