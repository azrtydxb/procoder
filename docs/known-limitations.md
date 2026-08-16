# Known limitations

procoder gates other people's code. That obligates candour about where the
gate is weak. Every entry below was verified by running the tool against a
throwaway file, at 0.2.0, and quotes the number that run produced. Where a
previous edition of this page disclosed something that has since been fixed,
the entry is gone rather than kept as false modesty — a document that
overstates weakness is as untrustworthy as one that hides it.

## The SAFE rules read one line, and only inside the sink

Every rung-1 rule is a regex over a single line, and most of them require the
untrusted expression to sit textually inside the call that consumes it.
Binding the value to a name first — the shape most real code has — defeats
them. Verified, each pair being the same violation written two ways:

| Written this way | Reported | Written this way | Reported |
|---|---|---|---|
| `db.query("SELECT * FROM t WHERE id=" + id);` | `safe/sql-injection` | `const q = "SELECT * FROM t WHERE id=" + id;`<br>`db.query(q);` | **nothing** |
| `cur.execute("SELECT * FROM t WHERE id=" + id)` | `safe/sql-injection` | `q = f"SELECT * FROM t WHERE id={id}"`<br>`cur.execute(q)` | **nothing** |
| `db.Query("SELECT * FROM t WHERE id=" + id)` | `safe/sql-injection` | `q := "SELECT * FROM t WHERE id=" + id`<br>`db.Query(q)` | **nothing** |
| `stmt.executeQuery("SELECT * FROM t WHERE id=" + id);` | `safe/sql-injection` | `String q = "SELECT ... id=" + id;`<br>`stmt.executeQuery(q);` | **nothing** |
| `exec("ls " + dir);` | `safe/shell-injection` | `const cmd = "ls " + dir;`<br>`exec(cmd);` | **nothing** |

There is no dataflow, no scope and no cross-line join anywhere in the engine.
`safe/xss-sink` is the exception that shows why: it fires on the *assignment
target* (`el.innerHTML = …`), so it is reported whichever way the value
arrives — but that also makes it fire on assignments that are safe.

The same single-line reading produces a mislabelled duplicate: the JS/TS SQL
rule's verb list includes `exec`, so `exec("ls " + dir);` is reported as
`safe/sql-injection` **and** `safe/shell-injection`. One of those two is
always wrong, and the engine cannot say which.

Use a real taint-tracking scanner for rung 1 if your threat model needs one.
procoder's rung 1 catches the violation written in one place, which is most of
what an agent writes and much less than what a codebase contains.

## Heuristic scanning, not parsing

`hooks/checks/shape.js` is a brace/indent counter, not a parser for any of the
six languages it measures. Verified consequences:

| Case | Effect |
|---|---|
| Signature wrapped over more than `SIGNATURE_LOOKBACK` (10) lines, or spanning more than `SIGNATURE_MAX_CHARS` (1000) | The rescan gives up and the function is not measured. One parameter per line, the last wrap still measured is **9 parameters** in JS/TS and Go and **8** in Python (`hooks/checks/lang/py.js`, same 10-line bound, one line of it spent on the `def`). A signature wrapped further is invisible to all three function-shape rules. |
| Parameter list longer than the packs' 500-character regex span | Not measured — and only in the three packs that have that span. JS/TS measures a 493-character parameter list (55 parameters) and misses 502 (56); Go misses at 505 (39 parameters); Rust misses at 502 (36). Python, Java and C# have no such ceiling: they stop only at the 4KB per-line shape guard, at 4093 characters (455 parameters in Python, 315 in Java and C#). The ceiling loses precisely the most extreme functions, in half the languages. |
| Python indented 2 spaces per level | Depth is `column / tabWidth` with `tabWidth` hard-coded to 4 in `hooks/checks/lang/py.js`, and not configurable. A 2-space file reports **half** its real depth: 5 real levels measure 2, 7 real levels measure 3 and pass the limit of 3, and the first report arrives at 8 real levels. Consistent 4-space and consistent tabs both measure correctly (5 levels measure 5). |
| Mixed tab/space indentation in Python | Depth is counted as *changes* of indentation rather than as multiples of `tabWidth`, but the column it compares is still tab-expanded at `tabWidth`. A tab level alternating with a 4-space level lands on the same column and measures correctly (5 levels measure 5); a tab level alternating with a 2-space level does not (5 levels measure 3), and a tab-indented block inside an 8-space block measures as no deeper (5 levels measure 3). Python 3 rejects such a file outright (`TabError`), so there is no correct tab width to recover. |

Swapping in a real per-language parser is the fix; it has not happened in
0.2.0.

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
| A rule id containing a colon is never validated | The id grammar is wide enough for an external linter's own rule id — `true/eslint:no-eval`, `true/ruff:E501`, `true/eslint:@typescript-eslint/no-explicit-any` — and any id matching `<rung>/<word>:<anything>` is accepted on shape alone. Nothing checks that the tool exists, that the rule exists, or that the tool is even configured. Verified: `true/nosuchtool:nosuchrule` and `safe/hardcoded-secret:x` are both accepted in silence and both silence nothing. A typo inside a colon id is the one marker mistake that is still a **silent no-op**. |
| An unknown colon-free id is named, but only once per line number per run | A typo (`safe/hardcoded-secrets`) prints `procoder: unknown rule id "…" in the literal marker on line N — it suppresses nothing` on stderr, and the finding still reports. The de-duplication key is the id and the line number with **no file in it**, so scanning a tree where the same typo sits on the same line of two files warns once, and the message never says which file. Verified: three files with the typo, on lines 1, 1 and 2, produce two warnings. |
| Reach is one line, or two for the standalone form | There is no block or file form, deliberately. A fixture file that is nothing but violating input cannot be marked line by line without changing the input — `tests/fixtures/` is excluded by path in `.procoder.toml` for exactly that reason, and that is the one case the marker cannot serve. |
| No wildcard | There is no bare or `*` form; the bare form is routed to `alone/blanket-suppression` and reported. |

## The self-scan, and what is ignored

`tests/dogfood.test.js` derives its file list from `git ls-files`, so a file is
in the gate the day it lands. Both of its hold-out lists — `HELD_OUT` and
`PENDING` — are empty, and both are now checked for still earning their place:
the test at `tests/dogfood.test.js:73` fails if a `HELD_OUT` path matches no
tracked file or has gone clean.

Measured today over 198 tracked files, `procoder check .` reports **0**
findings, exits 0, and skips 9 files across two `.procoderignore` files: 5
under `docs/superpowers/` and 4 `examples/*/before.*`. The repository root
also carries a `.procoderignore` for `.claude/` and `.superpowers/`; both are
untracked agent scratch, so the count it skips is whatever the machine
happens to be holding and is not a reproducible number.

What this does *not* give you:

- A `.procoderignore` has no expiry test, and it is now the only hold-out
  mechanism without one. One covering a path that has gone clean, or that no
  longer exists, is not reported. The per-run stderr line naming each ignore
  file and its file count is what makes it visible.
- `docs/superpowers/` is ignored because procoder still cannot tell a
  document quoting a violation from a file committing one, at the scale of a
  1500-line planning document. The line marker solves that case by case; it
  was not applied to historical documents.
- `examples/*/before.*` are ignored by path, so an ordinary audit does not
  report ten deliberate violations. They are still checked on every test run,
  through `procoder check --no-ignore`, and `examples/*/after.*` stays in the
  ordinary gate.
- `.claude/` is not in procoder's built-in defaults, so every user starting a
  scan at their repository root will see findings from whatever their agent
  tooling leaves there, until they write an ignore file of their own. That is
  deliberate: `.claude/` also holds hand-written hooks and scripts, and a
  default exclusion would un-gate them silently for everybody.

## External linters: two of the four report nothing at all

`hooks/checks/resolve.js` runs a project's own linter and lets its findings
replace the pack's `obvious/*` rules for the files it answers for. It never
replaces the SAFE rules. A linter that times out or crashes counts as not
having answered, so the pack covers the whole file.

That design holds. Two of the four integrations do not:

| Tool | What happens | Net effect |
|---|---|---|
| `cargo clippy` | `runToolResult` spawns with `stdio: ['ignore', 'pipe', 'ignore']` and parses stdout. cargo writes every diagnostic to **stderr**, which is discarded. `parse()` therefore always returns zero findings, and clippy exits 0, so the run counts as *answered* — which drops the Rust pack's `obvious/*` rules. | A Rust crate with clippy configured gets **less** than one without. Verified on a trivial crate: `pub fn wide(a0..a5)` reports `obvious/too-many-params` with no `[lints.clippy]` in `Cargo.toml` and reports **nothing at all** with it, while `cargo clippy` run by hand on the same crate takes 26ms and prints a `length comparison to zero` warning procoder never shows. |
| `golangci-lint` v2 | The v2 invocation `run --output.json.path stdout <file>` appends a human-readable tail (`1 issues:\n* typecheck: 1`) after the JSON document, so `JSON.parse` throws and `parse()` returns []. | No golangci-lint finding is ever reported. Verified: golangci-lint finds a `typecheck` issue that `procoder check` does not print. The Go pack still covers the file, so this loses coverage rather than granting a silent pass. |

`eslint` and `ruff` were not installed on the machine this was verified on, so
their integrations are undisclosed rather than cleared.

## `cargo clippy` cannot be scoped to one file

Verified in `hooks/checks/registry.js` and `hooks/checks/resolve.js`. Every
other supported external linter (eslint, ruff, golangci-lint) takes a single
file as its target. `cargo clippy` has none — it always compiles and lints the
whole crate. The hook still calls it with a file-scoped timeout (half the
hook's budget, clamped to 250–1500ms, so 1000ms at the default 2s budget), so
on anything but a trivial crate it will usually not finish. `parse()`
additionally discards any finding whose reported path is not the file being
written, so a warning in `src/other.rs` is never attributed to it. Both of
those are moot while the stderr defect above stands: nothing is parsed either
way.

## The ratchet baseline: what it guarantees, and what it doesn't

`hooks/checks/baseline.js` fingerprints each finding from its rule id, file
path, normalized source line text, and an occurrence ordinal — deliberately
excluding the line number.

Holds, each verified against a baselined finding:

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
an empty accepted set, never a partial one. Verified against a hand-edited v1
file, in `bin/procoder.js`:

- `procoder verify` exits **2**, distinct from the **1** used for "the ratchet
  grew", and says the baseline is the wrong format — CI does not blame the user
  for a backlog they did not add.
- `procoder check` prints the same stderr notice but still reports every
  finding in the repo, because nothing is suppressed. The exit code is the
  ordinary one.
- `procoder baseline` always overwrites with a current-version file, so running
  it is the fix in both cases.

## The TOML parser is a documented subset

`hooks/checks/toml.js` supports `[tables]`, `[dotted.tables]`, string/int/float/
bool values, and arrays — single-line or spanning multiple lines, with trailing
commas and comments inside the array. Verified working: multi-line arrays,
nested arrays, arrays of ints, and a comma inside a quoted item
(`paths = ["a, b", "c"]` yields two entries, not three).

Where it still falls short. A line the parser cannot recognize at all is
**warned on stderr** with file and line, then skipped — not silently dropped;
likewise an array that never closes. Everything below is a case where a value
loads wrong, or lands somewhere the author did not put it:

| Input | What loads | Warned |
|---|---|---|
| `t = {a=1}` | the string `{a=1}` | no |
| `d = 1979-05-27` | the string `1979-05-27` | no |
| `s = "a\"b"` | `a\"b` — the escape is kept, backslash and all | no |
| `s = """line one`<br>`line two"""` | the string `"""line one`, opening quotes included | the second line only |
| `a.b = 1` (a dotted *key*, not a dotted table) | nothing | yes |
| `[[x]]` (array of tables) | the header is dropped and every key under it lands at the **top level**, not in a table | the header only |

Keep `.procoder.toml` to the documented subset, and check stderr after editing
it.

## Deliberate narrowings from performance work

Every long-line path was made linear, each proven behaviour-identical by
differential runs. The narrowings that survived are real, and disclosed here:

| Narrowing | Where | Effect |
|---|---|---|
| 500-character regex spans | `hooks/checks/lang/ts.js`, `go.js`, `rust.js`, and the sink rules in `py.js`, `jvm.js`, `dotnet.js` | A sink whose interpolation, argument list or parameter list runs past 500 characters is not matched. For the shape rules this is a JS/TS, Go and Rust ceiling only — see the table above for the measured cut-offs. |
| 200-character spans (`SPAN_MAX`) | `hooks/checks/universal.js` | Same, for interpolated log arguments and commented-out-code detection. |
| 20 findings per line | `hooks/checks/run.js` | Overflow is reported as its own finding (`true/findings-suppressed`) naming the count, never silently dropped. Verified: a line of 3000 minified `try{f()}catch(e){}` blocks reports 20 findings plus `line 1: 2980 further findings suppressed (cap 20 per line)`. No honest line reaches 20. |
| 4KB per-line guard, shape path only | `hooks/checks/run.js` | Function length, params, complexity and nesting are not measured on a line over 4KB. The language packs' SAFE rules and the universal pack read it unguarded — verified: an AWS key and an `eval(` on the same 11KB minified line are both reported. |
| 4MB per-file skip | `hooks/checks/run.js` | Larger files are not checked at all. Verified on a 4,600,013-byte file: exit 0 and `procoder: skipped big.js (too-large) — not checked.` on stderr. An unreadable file gets the same treatment and the same kind of line. |
| 2s budget | `hooks/checks/run.js` | Checks after the deadline are skipped. The universal pack (rung 1) runs first, before any external linter, so it never loses its budget to one. Measured headroom: a 1MB single minified line costs 101ms end to end, a 50,000-line ordinary file 140ms. |

## The PostToolUse hook only reports near the edit

`hooks/procoder-check.js` narrows the language packs' findings to the region
the tool call touched, ±3 lines (`CONTEXT_MARGIN`). The universal pack is
exempt — a credential is a leak wherever it sits — but a file-level finding
reported at line 1 (`obvious/nesting-depth`) will not surface from an edit made
further down. Verified against a 57-line file: an edit at line 57 reports only
the secret, an edit at line 1 reports the secret and the nesting depth.

The narrowing is off entirely when the hook cannot locate the tool call's text
in the saved file — a whole-file write, or an edit whose string no longer
matches — in which case every finding surfaces. Run `procoder check <file>` or
`/procoder:review` for the whole-file picture either way.

## Config keys

`.procoder.toml`'s supported keys are documented in `README.md`, not
duplicated here.
