# Known limitations

procoder gates other people's code. That obligates candour about where the
gate is weak. Every entry below was verified by running the tool against a
throwaway file, at 0.2.0, and quotes the number that run produced. Where a
previous edition of this page disclosed something that has since been fixed,
the entry is gone rather than kept as false modesty — a document that
overstates weakness is as untrustworthy as one that hides it.

## Taint tracking: one file, forward only

Binding a query to a variable before consuming it no longer defeats rung 1.
`hooks/checks/lang/taint.js` carries a local dataflow pass, and all six packs
run it. Verified, each the same violation bound to a name first:

| Written this way | Reported |
|---|---|
| `const q = "SELECT * FROM t WHERE id=" + id;`<br>`db.query(q);` | `safe/sql-injection` |
| `q = f"SELECT * FROM t WHERE id={id}"`<br>`cur.execute(q)` | `safe/sql-injection` |
| `q := "SELECT * FROM t WHERE id=" + id`<br>`db.Query(q)` | `safe/sql-injection` |
| `let q = format!("SELECT * FROM t WHERE id={}", id);`<br>`conn.query(&q);` | `safe/sql-injection` |
| `String q = "SELECT x FROM t WHERE id=" + id;`<br>`stmt.executeQuery(q);` | `safe/sql-injection` |
| `var q = $"SELECT * FROM t WHERE id={id}";`<br>`var cmd = new SqlCommand(q);` | `safe/sql-injection` |
| `const cmd = "ls " + dir;`<br>`exec(cmd);` | `safe/shell-injection` |

The scan is no longer a line scan. Physical lines are joined into logical
statements, a binding lives at the level it was *declared* at rather than the
level it was written on, bindings are dotted paths resolved through their
prefixes, and one pass learns which of the file's own functions return a
tainted value so a second can use it. Nine shapes this page used to list as
missed now hold, verified in **all six packs**: a field or property
(`o.q = "SELECT id=" + id; db.query(o.q)`), a helper's return value
(`const q = build(x)`), a value returned straight into the sink
(`db.query(b(1))`), a binding made inside a branch (`if (x) { q = … }`), one
built in a loop (`for (…) { q = q + p }`), a right-hand side a formatter
wrapped onto the next line, a transformation at the sink (`db.query(q.trim())`),
a container literal (`const parts = ["SELECT id=", id]` then `parts.join("")`),
and an inner binding of the same name, which no longer clears the outer one.
So do the four older ones: aliasing, accumulation (`q = q + id`, `q += id`),
interpolation into a template or f-string, and an annotated binding.

What remains missed is small, and each row is a file that reports **nothing**:

| Shape that is missed | The file |
|---|---|
| A parameter arriving already tainted | `function f(q) { db.query(q); }` — deliberate, and the single largest false positive available here. Nothing inside the file separates the untrusted caller from the one passing a constant, so reporting it would fire on every data-access helper ever written. Verified silent in all six. |
| A transformation at the sink, **in Go only** | `q := "SELECT id=" + id`<br>`db.Query(strings.TrimSpace(q))` — `strings.TrimSpace(q)` is a free function, syntactically identical to `sanitize(q)`, and following it would be following every wrapper. The method form is caught: `db.Query(q.Trim())` reports. The other five packs catch both. |
| A helper reached through a receiver | `self.build(id)` in Python, `this.build(id)` in Java, `R.build(1)` in JS — the return propagation is keyed by the function's own bare name, and a bound receiver prefix (`self`, `this`) ends the path lookup before it gets there. The bare call is caught in all six: `cur.execute(build(id))` and `stmt.executeQuery(build(id))` both report. |
| A JS/TS method shorthand, as a helper | `class R { build(id) { return "SELECT id=" + id; } run(id) { db.query(this.build(id)); } }` reports nothing. The ts pack recognises `function build(` and `const build = (` as function definitions and deliberately not `build(a) {`, whose name is ambiguous with any call. Java, C#, Go, Rust and Python all measure their methods. |
| A container read back by index or key | `const parts = ["SELECT id=", id]; db.query(parts[0] + parts[1]);` and `const m = { q: "SELECT id=" + id }; db.query(m["q"]);` are both silent. The container *literal* is a source, which is what makes the `join` form work; element-wise reads would need a real value model. `m.q` written as a dotted path is caught. |
| Anything cross-file | there is no resolver. In-file call propagation is one level deep, by the callee's own name; `import { build } from "./b.js"; db.query(build(1));` is silent. |

It errs towards missing on purpose, and the pure-literal cases hold: `const q =
"SELECT * FROM t"`, `"SELECT " + "* FROM t"`, a template with no interpolation,
and a literal reassignment after a tainted one (`q = "SELECT 1"`) are each
silent at the sink. Constant arguments no longer fire either: `os.system("ls
/tmp")`, `exec.Command("sh", "-c", "ls /tmp")` and
`Command::new("sh").arg("-c").arg("ls /tmp")` are all silent. Nor does a
*name* that this file only ever binds to a literal: `const TABLE = "users";
const q = "SELECT * FROM " + TABLE; db.query(q);` is silent in all six packs,
as is its shell twin (`const DIR = "/tmp"; exec("ls " + DIR);`). The analysis
stays conservative in the safe direction — a name reassigned from input
anywhere in the file, in any scope, taints again, verified.

Three shapes are **false positives on correct code**. The first two are the
price of the widened propagation, and both are new on this page:

| Probe | Reported | Why it is wrong |
|---|---|---|
| `let q = "SELECT id=" + id;`<br>`q = sanitizeSql(q);`<br>`db.query(q);` | `safe/sql-injection` | A right-hand side that names a tainted variable carries the taint on — that is what makes `q = q + id` work, and it cannot tell an escaper from any other wrapper. So the *fix* an author writes at the binding does not clear the finding, and neither does the driver's own escaper: `const q = "SELECT * FROM t WHERE id=" + mysql.escape(id); db.query(q);` reports too. Verified in all six packs, each with its own spelling — `sanitize(q)` in Python, Go, Rust, Java and C# alike. A literal reassignment *does* clear it (`q = "SELECT 1"`), so the discharge exists; it is only calls that cannot be judged. |
| A cache, queue or job-runner call in a file that also talks to a database | `safe/sql-injection` | The database gate asks about the *file*, and once any evidence in it answers yes, every `execute`, `query` and `raw` call in that file is a SQL sink. Verified: a module importing `pg` and running one parameterized `pool.query(…)`, with `const cmd = 'SET user:' + key; redis.execute(cmd);` three lines below, reports `safe/sql-injection` against the redis call. So does `runner.execute(step)` in a file whose other function runs `cur.execute("SELECT 1")`. The return propagation carries it one call further — `cache.execute(cacheKey(id))` in the same file reports, built at the helper's `return`. Each of those three is silent in a file with no database evidence in it at all. |
| `q = f"run job {x}"`<br>`pool.execute(q)` | `safe/sql-injection` | `execute` is not a database verb here. One of the three things that count as file-level evidence is a *receiver name* — `db.`, `cur.`, `conn.`, `stmt.`, `session.`, `pool.`, `repo.`, `tx.`. Those are ordinary names outside a database: a worker `pool`, an HTTP `session`, a git `repo`. Verified: `runner.execute(q)` and `cmd.execute(q)` on the same input are silent, `pool.execute(q)`, `session.execute(q)` and `repo.execute(…)` report. |

The reverse of that gate is the matching **silent coverage loss**: a genuine
injection in a file that contains no database vocabulary at all is missed.
Verified, two files that report **nothing**:

```
const q = base + req.query.id;
api.execute(q);
```

and the same with `client.query(q)`. Neither `api` nor `client` is a canonical
handle, and neither file holds a SQL keyword, a driver name or an ORM name, so
the whole rule stands down. Add one line of evidence and it comes back —
`const { Client } = require('pg');` above the same `client.query(q)` reports.
A per-call escape hatch exists for the unambiguous method forms
(`executeQuery`, `rawQuery`, `QueryRowContext`, `CommandText`), but it only
reaches the *line* rules: the taint sinks match `query`, `execute` and `raw` as
whole words, so `api.executeQuery(q)` on a bound name is silent in a
vocabulary-free file too, verified. This bought back a large class of false
positives on Command-pattern and job-runner code; the price is that a query
assembled from fragments, in a file whose SQL lives elsewhere, is not seen.

`safe/xss-sink` fires on the *assignment target* (`el.innerHTML = …`), so it is
reported whichever way the value arrives. Constant markup is silent now —
`el.innerHTML = "<b>static</b>";`, the backtick form, and a name bound only to
literal markup are each verified silent — but an **already-escaped value still
reports**: `const safe = escapeHtml(x); el.innerHTML = safe;` is a finding, and
so is `el.innerHTML = render(x);`. The rule cannot tell an escaper from any
other call.

Use a real taint-tracking scanner for rung 1 if your threat model needs one.

## Heuristic scanning, not parsing

`hooks/checks/shape.js` is a brace/indent counter, not a parser for any of the
six languages it measures.

Python's indent reading was rebuilt: nesting depth is now the height of a stack
of enclosing indentation columns, with no unit, no division and nothing asked
of a column but its order. `MAX_INDENT_STEP` and the commonest-step guess are
gone with it, and `SIGNATURE_LOOKBACK` is deleted — each `def` matches its own
closing paren. Three cases this page used to list are closed, verified: a
`def` indented 9, 10 or 12 columns per level is silent where it used to report
`nesting depth 7 (limit 3)`; a file mixing four-space and two-space regions
reports `nesting depth 7` where it used to report nothing; and a `def` wrapped
over 9 or 20 lines is measured (`20 parameters (limit 4)`) where 8 was the
ceiling. Uniform 2-space, 4-space and tab files all measure correctly.

What is left:

| Case | Effect |
|---|---|
| **A JS/TS method shorthand named after a JS control-flow keyword** | Invisible to every shape rule. The keyword sweep is now scoped to the one pattern that cannot decide it by shape — the ts pack's bare-name head, `name(a, b) {` with nothing in front of it — and to the seven words that really are JS statements: `if`, `for`, `while`, `switch`, `case`, `catch`, `with`. Verified, each a 6-parameter class method reporting **nothing** in JS and TS, and each of `lock(…)`, `using(…)`, `match(…)`, `when(…)` measured on the same file. Every other pack is clear by construction, because its head demands a declarator or two tokens: C# `void using(…)` and `void lock(…)`, Java `void match(…)`, Go `func match(…)` and `func with(…)`, JS's own `function with(…)`, and Python's `def match(…)` all report `6 parameters (limit 4)`. A method literally named `with` or `catch` is the realistic remnant — a builder, a promise-like. |
| A Rust raw identifier | `fn r#match(a0…a5)` and `fn r#try(a0…a5)` are invisible to every shape rule: the rust head reads `fn` then a word, and `r#` is not one. `fn matcher(a0…a5)` on the same file reports `6 parameters (limit 4)`. Raw identifiers are exactly how a Rust author names a function after a keyword, so this is the case the keyword sweep above was narrowed to avoid, arriving by a different route. |
| **A template literal containing a quote character** | Desynchronises the brace scan for the rest of the file. `` new RegExp(`(?:^|["'/])${x}`) `` — a template literal whose body holds `"` or `'` — leaves the enclosing function's closing brace uncounted, and the next function is swallowed into its span: measured here as `function is 57 lines (limit 40)` reported against a four-line function. Found by procoder against procoder while writing `hooks/checks/deps.js`, and worked around there by building the pattern from two plain strings. Every span-derived shape metric is affected; the line rules and the universal pack are not. |
| A parameter list on a very long line | No ceiling remains. `obvious/too-many-params` is not span-derived, so the 4KB per-line shape guard no longer covers it: a 1,000-parameter single-line signature (over 8KB) is measured and reported in JS/TS. |

Swapping in a real per-language parser is the fix; it has not happened in
0.2.0.

## Meta-text: the line marker, and what it cannot cover

Text that *describes* a violation reads to a regex exactly like the violation.
The mechanism for that is a line marker (`hooks/checks/patterns/markers.js`,
applied in `hooks/checks/universal.js` for every pack):

`<comment syntax> procoder: literal <rule-id>[, <rule-id>…] <reason>` <!-- procoder: literal alone/blanket-suppression the marker syntax written out, not a suppression -->

Trailing on a line, it covers that line; standing alone on its own line, it
covers that line and the next. A finding reported at one line but *built* at
another — the taint scan's, which carries the build line as a structured
`sourceLine` field rather than in its prose — is covered by a marker on either
of the two lines it names. Verified all four ways: with `const q = "SELECT
id=" + id;` on line 1 and `db.query(q);` on line 2, a trailing marker on line
1, a trailing marker on line 2, a standalone marker above line 1 and a
standalone marker above line 2 each suppress it. There is no new syntax and no
wildcard, and the reach is still the finding's own lines: a standalone marker
two lines above the build line suppresses nothing.

It must name its rules and give a reason of at least two words, or it parses as
a blanket or unexplained suppression and silences nothing. Limits, all
verified:

| Limit | Effect |
|---|---|
| A well-shaped id for a rule that does not exist | The tool half of a colon id is a closed set and is checked: `true/nosuchtool:nosuchrule` and `safe/dynamic-eval:x` both warn on stderr. The rule half is now checked against that tool's own id **grammar**, which catches the realistic mistake — the cross-tool paste. Verified, each warning `unknown rule id … — it suppresses nothing`: `true/ruff:no-eval`, `true/eslint:E501`, `true/clippy:E501`. What is deliberately still accepted is a well-formed id for a rule nobody wrote: `true/eslint:no-such-rule-at-all` and `true/ruff:ZZ999` are both accepted in silence and both silence nothing. Closing that needs a registry procoder cannot hold — eslint's namespace is open by construction, and pinning ruff's linter prefixes would warn on every correct marker the week ruff adds one. So a one-character typo *inside* a real tool's namespace remains a **silent no-op**. |
| The warning is per file and line, not per marker text | A typo (`safe/dynamic-evall`) prints `procoder: unknown rule id "…" in the literal marker at <file>:<line> — it suppresses nothing` on stderr, naming the file, and the finding still reports. Verified: three files carrying the same typo, on lines 1, 1 and 2, produce three warnings, each naming its own file. Two different unknown ids on one line each get their own warning, verified; only a repeat of the same id at the same place is de-duplicated. |
| Reach is the finding's own one or two lines, never a region | There is no block or file form, deliberately. A fixture file that is nothing but violating input cannot be marked line by line without changing the input — `tests/fixtures/` is excluded by path in `.procoder.toml` for exactly that reason, and that is the one case the marker cannot serve. |
| No wildcard | There is no bare or `*` form; the bare form is routed to `alone/blanket-suppression` and reported. |

## The self-scan, and what is ignored

`tests/dogfood.test.js` derives its file list from `git ls-files`, so a file is
in the gate the day it lands. Both of its hold-out lists — `HELD_OUT` and
`PENDING` — are empty, and both are now checked for still earning their place:
the test at `tests/dogfood.test.js:73` fails if a `HELD_OUT` path matches no
tracked file or has gone clean.

Measured today over **203** tracked files, `procoder check .` reports **0**
findings and exits 0, having skipped **18** of them and said so on stderr for
every one:

- **9 by `[exclude] paths` in `.procoder.toml`**, one line per pattern with
  its count: 7 `tests/fixtures/*/dirty.*`, and one each for
  `.opencode/command/rot.md` and `.openclaw/commands/rot.md`. The dirty
  fixtures exist to be scanned by the tests rather than by the scan, and a line
  marker in one would change the input the test reads. The other two carry
  exactly one finding each, named in `.procoder.toml` next to the entry, with
  the source fix named too.
- **9 by two `.procoderignore` files**: 5 under `docs/superpowers/` (1500-line
  planning documents for work already executed, quoting rule ids by the
  hundred — 56 findings under `--no-ignore`, all of them meta-text) and 4
  `examples/*/before.*` (files that violate a rung on purpose, and are still
  checked on every test run through `procoder check --no-ignore`).

So **185** files are actually read, and README.md publishes those three
numbers; `tests/dogfood.test.js` asserts them against the scan's own output, so
the paragraph fails the build rather than going quietly out of date. The
repository root also carries a `.procoderignore` for `.claude/` and
`.superpowers/`; both are untracked agent scratch, so the count it skips is
whatever the machine happens to be holding and is not a reproducible number.

This list was 46 files until the staleness rules below existed to judge it. Of
the 32 generated rule files then excluded on the "it is generated" argument, 30
held nothing back at all — they are rendered from `skills/procoder/SKILL.md`,
which is itself in the gate, so the markers that keep the source clean render
into them too. They are gated now. The remaining two are one false positive,
not a category.

What this does *not* give you:

- **A `.procoderignore` has no expiry test.** `[exclude] paths` now has a full
  one. `unusedPathExclusions` in `hooks/checks/config.js` reports a configured
  path exclusion three ways — its path no longer exists, it matches no file in
  the tree, or every file it covers is clean — under plain `verify`, failing
  the build only under `--unused-exclusions`, which is the contract
  `unusedRuleExclusions` has always had. Verified in a throwaway repo:
  `paths = ["keep/", "**/*.gen.ts"]` with `keep/` holding one clean file and no
  `.gen.ts` anywhere now names both, and exits non-zero under the flag. The two
  tree-wide rules are decided only when the run's targets covered the whole
  repository — "this glob matches nothing" is a claim about the tree, and a
  `verify` over one file has not seen it — so a partial run still says nothing,
  deliberately. An ignore file gets no staleness test of any kind.
- **`--no-ignore` does not re-include a `[exclude] paths` entry, on purpose.**
  The flag answers "why is this file not being checked?" by turning off every
  `.procoderignore` for the run, and nothing else. `[exclude] paths` is the
  project-wide contract, written once at the root and reviewable in one place,
  and it carries the built-in `node_modules/`, `vendor/`, `dist/` defaults —
  so a flag that reached it would be a back door into `node_modules`, not a
  diagnostic. Verified: with `[exclude] paths = ["sub/"]`, an injected
  `safe/sql-injection` in `sub/x.js` is silent under `procoder check .` and
  under `procoder check --no-ignore .` alike. It is no longer silent *about
  itself*, which is the part that changed: both runs print `2 files skipped by
  [exclude] paths "sub/" in .procoder.toml`, and naming the file on the command
  line prints `skipped sub/x.js ([exclude] paths "sub/" in .procoder.toml)`
  rather than answering a direct question with silence.
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

## External linters: deferring costs you procoder's own thresholds

`hooks/checks/resolve.js` runs a project's own linter and lets its findings
replace the pack's `obvious/*` rules for the files it answers for. It never
replaces the SAFE rules. A linter that times out, crashes, or emits output its
parser cannot read counts as not having answered, so the pack covers the whole
file.

Both integrations that reported nothing at all now work, and cargo's cached
long-format replay is read as well as the short one. Verified against clippy
0.1.93 and golangci-lint 2.12.2:

| Tool | Verified |
|---|---|
| `cargo clippy` | Findings are read from stderr and reported in **either** rendering. Same crate, same file: after `cargo clippy --message-format short`, procoder reports `length comparison to zero …` and `writing &Vec instead of &[_] …`; after `cargo clean` and a plain `cargo clippy`, it reports the same two from the long rendering. A crate that fails to compile declines the run rather than counting as clean — with a type error present, `procoder check src/lib.rs` still reports the Rust pack's `safe/sql-injection`. |
| `golangci-lint` v2 | The human-readable tail after the JSON document is stripped before parsing. `[2 TRUE] main.go:6 typecheck: declared and not used: x` is reported, matching `golangci-lint run main.go` by hand. |

Two costs remain, each verified:

| Limitation | Verified |
|---|---|
| **A configured linter's thresholds replace procoder's, including where it has none.** `pub fn wide(a0..a5)` reports `[3 OBVIOUS] 6 parameters (limit 4)` with no `[lints.clippy]` in `Cargo.toml`. Add it and `procoder check src/lib.rs` exits **0**: clippy answered, the Rust pack's `obvious/*` rules are dropped, and clippy's own `too_many_arguments` threshold is 7. That is the design — the project's linter defines OBVIOUS — but the practical effect is that configuring clippy loses every 5- and 6-parameter report. |
| **One whole-crate invocation per Rust file scanned.** `cargo clippy` has no file target, so a directory scan spawns it once per file. Measured today on a 33-file crate: `procoder check .` costs **970ms** warm, ~29ms per file, since cargo replays a cached build. A crate that takes longer than the per-file timeout to compile gets no clippy findings at all. |

`eslint` and `ruff` were not installed on the machine this was verified on, so
their integrations are undisclosed rather than cleared.

## `cargo clippy` cannot be scoped to one file

Verified in `hooks/checks/registry.js` and `hooks/checks/resolve.js`. Every
other supported external linter (eslint, ruff, golangci-lint) takes a single
file as its target. `cargo clippy` has none — it compiles and lints a whole
package (`-p <name>`, read by walking up to the nearest `Cargo.toml` with a
`[package]` name, so a workspace member does not drag in its siblings). The
hook still calls it with a file-scoped timeout, and that timeout is no longer
a size formula: `floor((deadline − now) × 0.6)` in `hooks/checks/run.js`, a
share of whatever is left of the 2s budget at the moment the linter is
invoked. A big file leaves less because the pack spent more, not because a
per-MB constant said so — which is what makes the arithmetic hold on a slow
host instead of only on the one it was measured on. Below 100ms nothing is
spawned at all: the run names `clippy (no budget left for it)` and appends
`true/budget-exhausted`. On anything but a small crate it will not finish.
`parse()` additionally discards any finding whose
reported path is not the file being checked: verified, a `&Vec` warning in
`src/other.rs` is reported by `procoder check src/other.rs` and never by
`procoder check src/lib.rs`.

## The ratchet baseline: what it guarantees, and what it doesn't

`hooks/checks/baseline.js` fingerprints each finding from its rule id, file
path, the normalized text of the *statement* the finding sits in — extended
forward while a bracket or operator is still open — and an occurrence ordinal,
deliberately excluding the line number.

Holds, each verified against a baselined finding:

- Reindenting a file, or moving a suppressed line up or down, does not
  resurrect a suppressed finding.
- **A re-wrapping formatter no longer breaks it.** `statementAt` reads the
  whole open construct and drops a comma added before a closer, so
  `db.query(x)` and the prettier-wrapped form of the same call fingerprint
  alike. Verified: baseline the flat form, wrap it across four lines with a
  trailing comma, and `verify` still says the ratchet holds.
- Copy-pasting a suppressed line does not ride in for free: the occurrence
  ordinal gives each copy its own identity, so only the first is suppressed.

Does not hold:

- **Moving code between files breaks it.** The path is part of the
  fingerprint; a relocated violation is a brand-new finding, not a tracked
  move. Verified: baseline `a.js`, move the function verbatim into `b.js`, and
  `verify` exits 1 with the finding listed as new.
- **File-level findings key on line 1.** `obvious/nesting-depth` is reported at
  line 1, so its fingerprint is over whatever the file's first line says. Edit
  line 1 — a copyright header, an import — and the accepted finding returns.
  Verified end to end.

## The baseline format is versioned, and old baselines are not migrated

`BASELINE_VERSION` in `hooks/checks/baseline.js` is 3; earlier fingerprints
cannot be reconstructed. A stale-version file loads as an empty accepted set,
never a partial one. Verified against a hand-edited v1 file, in
`bin/procoder.js`:

- `procoder verify` exits **2**, distinct from the **1** used for "the ratchet
  grew", and says the baseline is the wrong format — CI does not blame the user
  for a backlog they did not add. Exit 2 is the general "cannot verify" code
  and now has a second cause: a run in which some file could not be read at all
  (see the `max_file_bytes` row below). Both mean "this run proves nothing";
  the stderr line says which.
- `procoder check` prints the same stderr notice but still reports every
  finding in the repo, because nothing is suppressed. The exit code is the
  ordinary one.
- `procoder baseline` always overwrites with a current-version file, so running
  it is the fix in both cases.

## The TOML parser is a documented subset

`hooks/checks/toml.js` supports `[tables]`, `[dotted.tables]`, dotted keys
(`a.b = 1`), basic strings with escapes, literal strings, int, float, bool, and
arrays — single-line or spanning multiple lines, with trailing commas and
comments inside the array. Verified working: `s = "a\"b"` loads `a"b`,
`s = 'C:\src\'` loads the backslashes as written, `paths = ["a, b", "c"]`
yields two entries, nested arrays, mixed-type arrays, CRLF, and a BOM.

Everything the parser cannot read exactly is **warned on stderr** with file and
line, and the key is left unset. Verified refusals, each warned and none
guessed at: inline tables (`t = {a=1}`, including inside an array), dates and
times, hexadecimal (`0x1f`), underscored (`1_000`), exponent (`1e6`), `inf`,
`+1`, bare unquoted words, quoted keys (`"a b" = 1`), multi-line strings,
arrays of tables (`[[x]]`), unknown escapes, unterminated strings, and an array
that never closes.

A duplicate key is refused too, and refused in both directions: `x = 1` then
`x = 2` warns `.procoder.toml:2: duplicate key, both values ignored: x` and
leaves `x` unset, rather than taking the last. A repeated `[table]` header
warns `duplicate table header, section ignored`. Verified end to end: a
`.procoder.toml` with `paths` written twice under `[exclude]` excludes nothing
and says so.

One construct still loads a value the author did not write, silently:

| Input | What loads | Warned |
|---|---|---|
| A scalar and a table of the same name — `a = 1`<br>`[a]`<br>`b = 2` (or `a.b = 2`) | `{a: {b: 2}}`. The scalar is dropped. | no |

Keep `.procoder.toml` to the documented subset, write each key once, and check
stderr after editing it.

## Deliberate narrowings from performance work

Every long-line path was made linear, each proven behaviour-identical by
differential runs. The narrowings that survived are real, and disclosed here:

| Narrowing | Where | Effect, measured |
|---|---|---|
| 500-character span, nested ternary | `hooks/checks/lang/ts.js` | `obvious/nested-ternary` is reported with 400 characters between the two `?`, and not with 499. |
| 500-character span, signature head and tail | `go.js` (receiver, and the text between `)` and `{`), `rust.js` (generic list, and the same tail), `ts.js` (return type) | A 6-parameter function is measured with a 200-character tail or generic list, and not with a 700-character one — in Go, Rust and TS alike. Java and C# have no such span; their tails are anchored to end of line instead, and a method whose body opens on the signature's own line is measured correctly in both — `void m(int a, … int h) { return; }`, `void m(…) {}` and C#'s expression-bodied `void M(…) => a + b;` each report `8 parameters (limit 4)`. |
| 20 findings per line | `hooks/checks/run.js` | Overflow is reported as its own finding (`true/findings-suppressed`) naming the count, never silently dropped. Verified: a line of 3000 minified `try{f()}catch(e){}` blocks reports 20 findings plus `line 1: 2980 further findings suppressed (cap 20 per line)`. No honest line reaches 20. |
| 4KB per-line guard, span-derived shape rules only | `hooks/checks/run.js` | Function length, nesting depth and complexity are not measured on a line over 4KB, because a function on one line has no meaningful span. `obvious/too-many-params` is **not** guarded — it is exact on a minified line. The language packs' SAFE rules and the universal pack read the line unguarded: a credential 1,500 characters into a log call is still reported. |
| **1MB** per-file skip | `hooks/checks/config.js`, applied in `run.js` | This came down from 4MB to 2MB, and again to 1MB. The language pack is the one stage that cannot be abandoned part-way — everything after it is deadline-driven — so the size of what it is handed is the only lever on its cost, and the taint rebuild roughly doubled the ts pack's constant. 1MB of ordinary source costs the worst pack ~400ms of the 2000ms budget here, which still finishes on a host three times slower; 2MB did not. Larger files are not checked at all. Verified: a file of exactly 1,048,576 bytes is checked; at 1,048,577 `procoder check` exits 0 and prints `procoder: skipped over1mb.js (too-large) — not checked.` on stderr, while `procoder verify` over the same tree prints `1 file could not be checked (see above)` and **exits 2**. An unreadable file gets the same treatment and the same kind of line, and `check`, `baseline` and `verify` all report it. What remains is that `check` still exits **0** over a file it could not read: only `verify` treats an unread file as a hole in its own claim. |
| `[limits] max_file_bytes`, downward only | `hooks/checks/config.js` | A project may lower the cap, never raise it. `max_file_bytes = 999999999` warns `.procoder.toml:2: max_file_bytes 999999999 is above the measured ceiling 1048576 — using 1048576`; `0`, `-5`, `"big"` and `true` each warn `must be a positive number of bytes, ignored` and fall back to 1MB. A cap that is simply *too low* is not the sharp edge it once was: `max_file_bytes = 10` skips every file including `.procoder.toml` itself, names each one on stderr, and `procoder verify` then prints `3 files could not be checked (see above) — the ratchet cannot hold over files nothing looked at. Raise or remove [limits] max_file_bytes, or exclude the path deliberately.` and **exits 2**. Verified end to end. |
| 2s budget, and what it cuts | `hooks/checks/run.js` | Checks after the deadline are skipped. The order was reworked: the language pack now runs **before** the project's linter, and the linter is paid a *share* of what is left on the clock (0.6, floor 100ms) rather than a precomputed split, so the total is under budget by arithmetic on any host rather than by a constant measured on one. The per-MB reserve is deleted. Worst case at a 3× slowdown went from 1797ms to 1448ms of 2000ms, and holds to 6×. What the deadline does cut is now loud on both channels: a `true/budget-exhausted` finding appended **after** the per-file cap, so five SAFE findings cannot push it out, plus a stderr line naming the stage — verified with a zero budget, `procoder: package.json: 0ms budget exhausted — not checked: the dependency manifest rules`. The universal pack (rung 1) runs first of all and can never be the stage that is cut. |

The 200-character `SPAN_MAX` that used to bound the universal pack is gone.

## The PostToolUse hook reports near the edit, and at most five findings

`hooks/procoder-check.js` narrows the language packs' findings to the region
the tool call touched, ±3 lines (`CONTEXT_MARGIN`). The universal pack is
exempt — a credential is a leak wherever it sits — but a file-level finding
reported at line 1 (`obvious/nesting-depth`) will not surface from an edit made
further down. Verified against a 56-line file: an edit at line 56 reports only
the secret, an edit at line 1 reports the secret and the nesting depth.

The narrowing is off entirely when the hook cannot locate the tool call's text
in the saved file — a whole-file write, or an edit whose string no longer
matches — in which case every finding surfaces.

What survives the narrowing is then **truncated to 5** (`MAX_FINDINGS` in
`hooks/checks/run.js`), and the count in the message is the truncated one, not
the real one. Verified on a file of 8 `eval(` calls written whole:
`procoder [strict] — 5 findings in many.js`, while `procoder check many.js`
reports 8. The CLI is unbounded; the hook is not, and does not say so. Run
`procoder check <file>` or `/procoder:review` for the whole-file picture.

## Config keys

`.procoder.toml`'s supported keys are documented in `README.md`, not
duplicated here.
