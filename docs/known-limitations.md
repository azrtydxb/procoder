# Known limitations

procoder gates other people's code. That obligates candour about where the
gate is weak. Every entry below was verified by running the tool against a
throwaway file, at 0.2.0, and quotes the number that run produced. Where a
previous edition of this page disclosed something that has since been fixed,
the entry is gone rather than kept as false modesty — a document that
overstates weakness is as untrustworthy as one that hides it.

## Taint tracking: one file, one name, forward only

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

Four shapes that used to be missed now hold in all six packs, verified in each:
aliasing (`const b = a`), accumulation onto a name already holding the literal
(`q = q + id`, `q += id`), interpolation into a template or f-string, and an
*annotated* binding — `const q: string = …`, `let q: string = …` and Python's
`q: str = …` are each measured now, as Go's `var q string = …`, Rust's
`let q: String = …`, Java's `final String q = …` and C#'s `string q = …`
already were. A parameter list binds fresh names, so a parameter no longer
inherits an outer name's taint.

The scope is one file, one name, forward only, and everything outside it is a
miss. Each row below is a two-to-five-line file that reports **nothing**:

| Shape that is missed | The file |
|---|---|
| A field or property | `o.q = "SELECT id=" + id;`<br>`db.query(o.q);` |
| A helper's return value | `function build(id) { return "SELECT id=" + id; }`<br>`const q = build(x);`<br>`db.query(q);` |
| A value returned straight into the sink | `function b(id) { return "SELECT id=" + id; }`<br>`db.query(b(1));` |
| A binding made inside a branch | `let q = "SELECT";`<br>`if (x) { q = "SELECT id=" + id; }`<br>`db.query(q);` |
| Built in a loop | `let q = "SELECT";`<br>`for (const p of ps) { q = q + p; }`<br>`db.query(q);` — the same `q = q + p` is caught when it is not inside a block |
| A right-hand side that wraps to the next line | `const q =`<br>`  "SELECT id=" + id;`<br>`db.query(q);` |
| Any transformation at the sink | `const q = "SELECT id=" + id;`<br>`db.query(q.trim());` |
| A container | `const parts = ["SELECT id=", id];`<br>`const q = parts.join("");`<br>`db.query(q);` |
| An inner binding of the same name, which clears the outer one permanently | `let q = "SELECT id=" + id;`<br>`function f() { let q = "static"; }`<br>`db.query(q);` |
| A parameter arriving already tainted | `function f(q) { db.query(q); }` |
| Anything cross-function or cross-file | there is no call graph and no resolver |

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

One shape is still a **false positive on correct code**:

| Probe | Reported | Why it is wrong |
|---|---|---|
| `q = f"run job {x}"`<br>`pool.execute(q)` | `safe/sql-injection` | `execute` is not a database verb here. SQL rules are now gated on file-level database evidence, and one of the three things that count as evidence is a *receiver name* — `db.`, `cur.`, `conn.`, `stmt.`, `session.`, `pool.`, `repo.`, `tx.`. Those are ordinary names outside a database: a worker `pool`, an HTTP `session`, a git `repo`. Verified: `runner.execute(q)` and `cmd.execute(q)` on the same input are silent, `pool.execute(q)`, `session.execute(q)` and `repo.execute(…)` report. |

The reverse of that gate is the matching **silent coverage loss**, and it is
the newer of the two: a genuine injection in a file that contains no database
vocabulary at all is now missed. Verified, two files that report **nothing**:

```
const q = base + req.query.id;
api.execute(q);
```

and the same with `client.query(q)`. Neither `api` nor `client` is a canonical
handle, and neither file holds a SQL keyword, a driver name or an ORM name, so
the whole rule stands down. This bought back a large class of false positives
on Command-pattern and job-runner code; the price is that a query assembled
from fragments, in a file whose SQL lives elsewhere, is not seen at all.

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
six languages it measures. Verified consequences:

| Case | Effect |
|---|---|
| **A function or method whose name is a control-flow keyword** | Invisible to every shape rule, in five of the six packs. `shape.js` refuses to read `if (`, `for (`, `switch (`, `catch (`, `using (`, `lock (`, `match (`, `with (`, `when (`, `case (` and their siblings as a signature — which removed 104 false positives from one 486-file TypeScript tree, and takes with it every real function that happens to bear one of those names. Verified, each a 6-parameter definition reporting **nothing**: C# `void match(…)`, `void lock(…)`, `void using(…)`; Java `void match(…)`, `void catch(…)`; Go `func match(…)`, `func with(…)`; Rust `fn r#match(…)`; JS/TS `function match(…)` and the class method `match(…)`, `with(…)`, `when(…)`, `case(…)`. Only the Python pack is unaffected — `def match(a…f)` reports `6 parameters (limit 4)` — because `def` anchors it. Capitalise the name (`void Match(…)`, C#'s and Java's own convention) and it is measured again. `match` as a method name is the realistic case: a matcher, a router, a `String`-like API. |
| A Python `def` wrapped over more than `SIGNATURE_LOOKBACK` (10) lines | Python is the only pack still bounded here. One parameter per line, the last wrap Python measures is **8 parameters** (`def`, eight lines, `):` — ten lines); at 9 the function is invisible to all three Python function-shape rules. The five brace packs match a parameter list to its own `)`: a 400-parameter wrap is measured in JS/TS. |
| Python indented more than 8 columns per level | `MAX_INDENT_STEP` is 8, so a wider step is not a candidate for the file's indent unit and the code falls back to `tabWidth` (4). **A false positive:** a `def` containing two real levels of `if` — real depth 3 — reports `nesting depth 7 (limit 3)` at 10 columns per level and `nesting depth 6` at 9. At 8 columns and narrower it is correct and silent. |
| One Python file, two indent widths | The step is the *commonest* one in the file, so a region indented differently is measured against someone else's unit — and the error runs both ways. Verified in Python: a file of twelve four-space `def`s plus one nested **six** levels deep at two spaces reports **nothing**, and at seven levels reports `nesting depth 4` rather than 8. Uniform 2-space, 4-space and tab files all measure correctly. The five brace packs are not affected — they count braces, and the same mixed-width file in JS reports `nesting depth 7` and `8` correctly. |
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
| The *rule* half of a colon id is never validated | The tool half now is: `true/nosuchtool:nosuchrule` and `safe/dynamic-eval:x` both warn on stderr, because neither `nosuchtool` nor `dynamic-eval` is a configured tool. What follows the colon is still accepted on shape alone — `true/eslint:no-such-rule-at-all` and `true/ruff:ZZ999` are both accepted in silence and both silence nothing. A typo in the rule half is the one marker mistake that is still a **silent no-op**. |
| An unknown colon-free id is named, but only once per line number per run | A typo (`safe/dynamic-evall`) prints `procoder: unknown rule id "…" in the literal marker at line N — it suppresses nothing` on stderr, and the finding still reports. `unknownRuleId` in `universal.js` *can* key on the file and name it, but only when it is handed a `relPath`, and it also stores the file-less key alongside — so the pack's own file-less pass runs first, prints the file-less message, and de-duplicates the per-file pass that would have named it. Through the CLI the message therefore never says which file, and a typo on the same line of two files warns once. Verified: three files with the typo, on lines 1, 1 and 2, produce two warnings, neither naming a file. |
| Reach is the finding's own one or two lines, never a region | There is no block or file form, deliberately. A fixture file that is nothing but violating input cannot be marked line by line without changing the input — `tests/fixtures/` is excluded by path in `.procoder.toml` for exactly that reason, and that is the one case the marker cannot serve. |
| No wildcard | There is no bare or `*` form; the bare form is routed to `alone/blanket-suppression` and reported. |

## The self-scan, and what is ignored

`tests/dogfood.test.js` derives its file list from `git ls-files`, so a file is
in the gate the day it lands. Both of its hold-out lists — `HELD_OUT` and
`PENDING` — are empty, and both are now checked for still earning their place:
the test at `tests/dogfood.test.js:73` fails if a `HELD_OUT` path matches no
tracked file or has gone clean.

Measured today over **202** tracked files, `procoder check .` reports **0**
findings and exits 0, having skipped **19** of them and said so on stderr for
every one:

- **10 by `[exclude] paths` in `.procoder.toml`**, one line per pattern with
  its count: 7 `tests/fixtures/*/dirty.*`, and one each for
  `tests/fixtures/ts/clean.ts`, `.opencode/command/rot.md` and
  `.openclaw/commands/rot.md`. The dirty fixtures exist to be scanned by the
  tests rather than by the scan, and a line marker in one would change the
  input the test reads. The other three carry exactly one finding each, named
  in `.procoder.toml` next to the entry, with the source fix named too.
- **9 by two `.procoderignore` files**: 5 under `docs/superpowers/` (1500-line
  planning documents for work already executed, quoting rule ids by the
  hundred — 56 findings under `--no-ignore`, all of them meta-text) and 4
  `examples/*/before.*` (files that violate a rung on purpose, and are still
  checked on every test run through `procoder check --no-ignore`).

So **183** files are actually read, and README.md publishes those three
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
| **One whole-crate invocation per Rust file scanned.** `cargo clippy` has no file target, so a directory scan spawns it once per file. Measured on a 33-file crate: `procoder check .` costs 921ms warm, ~28ms per file, since cargo replays a cached build. A crate that takes longer than the per-file timeout to compile gets no clippy findings at all. |

`eslint` and `ruff` were not installed on the machine this was verified on, so
their integrations are undisclosed rather than cleared.

## `cargo clippy` cannot be scoped to one file

Verified in `hooks/checks/registry.js` and `hooks/checks/resolve.js`. Every
other supported external linter (eslint, ruff, golangci-lint) takes a single
file as its target. `cargo clippy` has none — it always compiles and lints the
whole crate. The hook still calls it with a file-scoped timeout, and that
timeout shrinks as the file grows: `min(1500, max(250, floor(budget/2) −
ceil(MB × 200)))` in `hooks/checks/run.js`, so 1000ms for a small file at the
default 2s budget and 600ms for one at the 2MB cap. On anything but a small
crate it will not finish. `parse()` additionally discards any finding whose
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
| 2MB per-file skip | `hooks/checks/config.js`, applied in `run.js` | This came down from 4MB, which measurement did not support. Larger files are not checked at all. Verified: a file of exactly 2,097,152 bytes is checked; at 2,097,153 `procoder check` exits 0 and prints `procoder: skipped over2mb.js (too-large) — not checked.` on stderr, while `procoder verify` over the same tree prints `1 file could not be checked (see above)` and **exits 2**. An unreadable file gets the same treatment and the same kind of line, and `check`, `baseline` and `verify` all report it — the skip notice moved into the shared funnel, so no command drops a file in silence any more. What remains is that `check` still exits **0** over a file it could not read: only `verify` treats an unread file as a hole in its own claim. |
| `[limits] max_file_bytes`, downward only | `hooks/checks/config.js` | A project may lower the cap, never raise it. `max_file_bytes = 999999999` warns `.procoder.toml:2: max_file_bytes 999999999 is above the measured ceiling 2097152 — using 2097152`; `0`, `-5`, `"big"` and `true` each warn `must be a positive number of bytes, ignored` and fall back to 2MB. A cap that is simply *too low* used to be the sharp edge, and is not any more: `max_file_bytes = 10` skips every file including `.procoder.toml` itself, names each one on stderr, and `procoder verify` then prints `3 files could not be checked (see above) — the ratchet cannot hold over files nothing looked at. Raise or remove [limits] max_file_bytes, or exclude the path deliberately.` and **exits 2**. Verified end to end. |
| 2s budget | `hooks/checks/run.js` | Checks after the deadline are skipped. The universal pack (rung 1) runs first, before any external linter, so it never loses its budget to one. Measured headroom on this machine: a 1MB single minified line costs 217ms end to end, a 50,000-line ordinary file 184ms, a 500KB minified line 129ms. A 512KB block comment in the ts and rust packs, which used to cost 124s, now costs 58ms; `tests/perf-guard.js` covers the shape. |

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
