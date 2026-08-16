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

Three shapes that used to be missed now hold in all six packs, verified in each:
aliasing (`const b = a`), accumulation onto a name already holding the literal
(`q = q + id`, `q += id`), and interpolation into a template or f-string. A
parameter list binds fresh names, so a parameter no longer inherits an outer
name's taint.

The scope is one file, one name, forward only, and everything outside it is a
miss. Each row below is a two-to-five-line file that reports **nothing**:

| Shape that is missed | The file |
|---|---|
| An annotated binding, in the ts and py packs only | `const q: string = "SELECT id=" + id;`<br>`db.query(q);` — also `let q: string = …` and `q: str = f"…"`. Go's `var q string = …`, Rust's `let q: String = …`, Java's `final String q = …` and C#'s `string q = …` are all measured. |
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
`Command::new("sh").arg("-c").arg("ls /tmp")` are all silent.

Two shapes are not silent, and both are **false positives on correct code**:

| Probe | Reported | Why it is wrong |
|---|---|---|
| `const TABLE = "users";`<br>`const q = "SELECT * FROM " + TABLE;`<br>`db.query(q);` | `safe/sql-injection`, "built at line 2" | `TABLE` is a string literal bound to a name. Taint does no constant propagation: a *name* on the right-hand side taints the result even when the name holds a literal. Verified in all six packs — `TABLE`/`table` in JS, TS, Python, Go, Rust, Java and C# each report — and in the shell twin, `const DIR = "/tmp"; const cmd = "ls " + DIR; exec(cmd);`. This is the most likely reason procoder reports a rung-1 finding against code that is fine. |
| `q = f"run job {x}"`<br>`pool.execute(q)` | `safe/sql-injection` | `execute` is not a database verb here. The Python sink list cannot tell a cursor from any other object. A constant argument — `pool.execute("run job 1")` — is silent, so it takes an interpolation to trip. |

The single-line reading also still produces a mislabelled duplicate: the JS/TS
SQL rule's verb list includes `exec`, so `exec("ls " + dir);` is reported as
`safe/sql-injection` **and** `safe/shell-injection`. One of those two is always
wrong, and the engine cannot say which. The taint path does not repeat the
mistake — `const cmd = "ls " + dir; exec(cmd);` reports only shell.

`safe/xss-sink` remains the rule that fires on the *assignment target*
(`el.innerHTML = …`), so it is reported whichever way the value arrives — and
also on assignments that are safe. Verified: `el.innerHTML = "<b>static</b>";`
and `el.innerHTML = safe;` after `const safe = escapeHtml(x);` are both
reported.

Use a real taint-tracking scanner for rung 1 if your threat model needs one.

## Heuristic scanning, not parsing

`hooks/checks/shape.js` is a brace/indent counter, not a parser for any of the
six languages it measures. Verified consequences:

| Case | Effect |
|---|---|
| **A Java or C# method whose body opens on the signature's own line** | Invisible to every shape rule in `hooks/checks/lang/jvm.js` and `dotnet.js`. `void m(int a, … int h) { return; }` and `void m(int a, … int h) {}` each report **nothing**; move the `{` to the next line and the same eight parameters report `8 parameters (limit 4)`. C#'s expression-bodied form (`void M(…) => a + b;`) is silent too. Verified up to a 300-parameter single-line signature: still nothing. The other four packs measure the same-line form correctly — `function m(a…h) {}`, `func M(a…h int) {}`, `pub fn m(a: i32 …) {}` and `def m(a…h): pass` all report. |
| A Python `def` wrapped over more than `SIGNATURE_LOOKBACK` (10) lines | Python is the only pack still bounded here. One parameter per line, the last wrap Python measures is **8 parameters** (`def`, eight lines, `):` — ten lines); at 9 the function is invisible to all three Python function-shape rules. The five brace packs match a parameter list to its own `)`: a 400-parameter wrap is measured in JS/TS. |
| Python indented more than 8 columns per level | `MAX_INDENT_STEP` is 8, so a wider step is not a candidate for the file's indent unit and the code falls back to `tabWidth` (4). **A false positive:** a `def` containing two real levels of `if` — real depth 3 — reports `nesting depth 7 (limit 3)` at 10 columns per level and `nesting depth 6` at 9. At 8 columns and narrower it is correct and silent. |
| One file, two indent widths | The step is the *commonest* one in the file, so a region indented differently is measured against someone else's unit — and the error runs both ways. Verified: a file of twelve four-space functions plus one function nested **six** levels deep at two spaces reports **nothing**, and at seven levels reports `nesting depth 4` rather than 8. Uniform 2-, 3-, 4-space, tab, and tab-mixed-with-spaces files all measure correctly. |
| A parameter list on a very long line | No ceiling remains. `obvious/too-many-params` is not span-derived, so the 4KB per-line shape guard no longer covers it: a 1,000-parameter single-line signature (over 8KB) is measured and reported in JS/TS. |

Swapping in a real per-language parser is the fix; it has not happened in
0.2.0.

## `safe/hardcoded-secret` matches the credential word, not the name

`hooks/checks/universal.js` requires a word boundary immediately before the
credential word, so a credential whose name is *prefixed* by anything is
missed. Each of these is a one-line file that reports **nothing**:

- `const dbPassword = "…";` <!-- procoder: literal safe/hardcoded-secret the missed shape named, not a credential -->
- `const userPassword = "…";` <!-- procoder: literal safe/hardcoded-secret the missed shape named, not a credential -->
- `const apiSecret = "…";` <!-- procoder: literal safe/hardcoded-secret the missed shape named, not a credential -->
- `const api_secret = "…";` <!-- procoder: literal safe/hardcoded-secret the missed shape named, not a credential -->

The unprefixed and separator-joined forms are caught — `password`, `api_key`,
`apiKey`, `authToken`, `clientSecret` all report `credential assigned a literal
value`. The rule is otherwise language-agnostic and applies to every file
procoder reads: verified on the same line in 27 extensions, including `.env`,
`Dockerfile`, `.yml`, `.json`, `.tf`, `.sql`, `.md` and `.txt`, all 27 reported.

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
| **A cross-line taint finding must be marked at the sink, not where the string is built** | The marker matches the line the finding is *reported* on, and a taint finding is reported at the sink even though its message says "built at line N". `const q = "SELECT id=" + id; // procoder: literal safe/sql-injection …` on line 1, with `db.query(q);` on line 2, suppresses **nothing** — the marker has to go on line 2, which is not where a reader would put it. Verified both ways. |
| The *rule* half of a colon id is never validated | The tool half now is: `true/nosuchtool:nosuchrule` and `safe/dynamic-eval:x` both warn on stderr, because neither `nosuchtool` nor `dynamic-eval` is a configured tool. What follows the colon is still accepted on shape alone — `true/eslint:no-such-rule-at-all` and `true/ruff:ZZ999` are both accepted in silence and both silence nothing. A typo in the rule half is the one marker mistake that is still a **silent no-op**. |
| An unknown colon-free id is named, but only once per line number per run | A typo (`safe/dynamic-evall`) prints `procoder: unknown rule id "…" in the literal marker at line N — it suppresses nothing` on stderr, and the finding still reports. `unknownRuleId` in `universal.js` *can* key on the file and name it, but only when it is handed a `relPath`, and it also stores the file-less key alongside — so the pack's own file-less pass runs first, prints the file-less message, and de-duplicates the per-file pass that would have named it. Through the CLI the message therefore never says which file, and a typo on the same line of two files warns once. Verified: three files with the typo, on lines 1, 1 and 2, produce two warnings, neither naming a file. |
| Reach is one line, or two for the standalone form | There is no block or file form, deliberately. A fixture file that is nothing but violating input cannot be marked line by line without changing the input — `tests/fixtures/` is excluded by path in `.procoder.toml` for exactly that reason, and that is the one case the marker cannot serve. |
| No wildcard | There is no bare or `*` form; the bare form is routed to `alone/blanket-suppression` and reported. |

## The self-scan, and what is ignored

`tests/dogfood.test.js` derives its file list from `git ls-files`, so a file is
in the gate the day it lands. Both of its hold-out lists — `HELD_OUT` and
`PENDING` — are empty, and both are now checked for still earning their place:
the test at `tests/dogfood.test.js:73` fails if a `HELD_OUT` path matches no
tracked file or has gone clean.

Measured today over 201 tracked files, `procoder check .` reports **0**
findings, exits 0, and skips 9 files across two `.procoderignore` files: 5
under `docs/superpowers/` and 4 `examples/*/before.*`. The repository root
also carries a `.procoderignore` for `.claude/` and `.superpowers/`; both are
untracked agent scratch, so the count it skips is whatever the machine
happens to be holding and is not a reproducible number.

What this does *not* give you:

- **`[exclude] paths` in `.procoder.toml` is the quietest hold-out there is.**
  Unlike a `.procoderignore`, it prints no per-run count, has no expiry test,
  and `--no-ignore` does not defeat it. Verified in a throwaway repo: with
  `[exclude] paths = ["sub/"]`, an injected `safe/sql-injection` in `sub/x.js`
  is silent under `procoder check .`, silent under `procoder check --no-ignore
  .`, and silent when the file is named on the command line — three runs, no
  finding, no stderr, exit 0 each time. `unusedRuleExclusions` in
  `bin/procoder.js` deliberately checks only `rules`, never `paths`. A stale
  or over-broad path exclusion is invisible from every angle procoder offers.
- A `.procoderignore` has no expiry test either. One covering a path that has
  gone clean, or that no longer exists, is not reported. It at least prints a
  per-run stderr line naming the ignore file and its file count.
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
  for a backlog they did not add.
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
| 500-character span, signature head and tail | `go.js` (receiver, and the text between `)` and `{`), `rust.js` (generic list, and the same tail), `ts.js` (return type) | A 6-parameter function is measured with a 200-character tail or generic list, and not with a 700-character one — in Go, Rust and TS alike. Java and C# have no such span; their tails are anchored to end of line instead, which is why a same-line body hides them entirely (see above). |
| 20 findings per line | `hooks/checks/run.js` | Overflow is reported as its own finding (`true/findings-suppressed`) naming the count, never silently dropped. Verified: a line of 3000 minified `try{f()}catch(e){}` blocks reports 20 findings plus `line 1: 2980 further findings suppressed (cap 20 per line)`. No honest line reaches 20. |
| 4KB per-line guard, span-derived shape rules only | `hooks/checks/run.js` | Function length, nesting depth and complexity are not measured on a line over 4KB, because a function on one line has no meaningful span. `obvious/too-many-params` is **not** guarded — it is exact on a minified line. The language packs' SAFE rules and the universal pack read the line unguarded: a credential 1,500 characters into a log call is still reported. |
| 2MB per-file skip | `hooks/checks/config.js`, applied in `run.js` | This came down from 4MB, which measurement did not support. Larger files are not checked at all. Verified: a file of exactly 2,097,152 bytes is checked; at 2,097,153 the run exits 0 and prints `procoder: skipped over2mb.js (too-large) — not checked.` on stderr. An unreadable file gets the same treatment and the same kind of line, and `check`, `baseline` and `verify` all report it — the skip notice moved into the shared funnel, so no command drops a file in silence any more. |
| `[limits] max_file_bytes`, downward only | `hooks/checks/config.js` | A project may lower the cap, never raise it. `max_file_bytes = 999999999` warns `is above the measured ceiling 2097152 — using 2097152`; `0`, `-5`, `"big"` and `true` each warn `must be a positive number of bytes, ignored` and fall back to 2MB. A cap that is simply *too low* is the sharp edge: `max_file_bytes = 10` skips every file including `.procoder.toml` itself, names each one on stderr — and `procoder verify` then prints `0 findings against a baseline of 0 — ratchet holds` and **exits 0**. CI passes having checked nothing. |
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
