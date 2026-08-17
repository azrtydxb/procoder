# Known limitations

procoder gates other people's code. That obligates candour about where the
gate is weak. Every entry below was verified by running the tool against a
throwaway file, at 0.2.0, and quotes the number that run produced. Where a
previous edition of this page disclosed something that has since been fixed,
the entry is gone rather than kept as false modesty — a document that
overstates weakness is as untrustworthy as one that hides it.

Five audits running, this page went stale by keeping defects that had been
fixed. The audit before this one inverted the failure: the engine grew a
parallel scanner, two machine-readable formats, a `--since`, an `init`, two
rungs and three rules, and the page said nothing about any of it. Silence about
new surface is the same dishonesty in different clothes.

This pass covers three engine changes. SARIF now names the files a run skipped
and `--since` sees a rename; four rung-1 false positives on correct escaping are
gone; and the parallel scan runs one budget, one config and a watchdog down both
paths, with `--jobs` clamped. Ten rows are deleted rather than hedged. What
replaced them is not shorter: an allow-list of escaper names is a decision to
trust a name, and every name it trusts is a miss written up below.

## Rungs 5 and 6 have no engine behind them at all

FAST and MEANT are rungs of the doctrine and nothing else. `RUNGS` in
`hooks/checks/finding.js` lists six names, `[rungs]` in `.procoder.toml`
accepts `fast` and `meant` and defaults both to `warn` — and **no check in the
engine can ever produce a finding at either**. Verified: a grep for a `fast/`
or `meant/` rule id over the whole of `hooks/` matches nothing, and
`BUILTIN_RULE_IDS` in `hooks/checks/patterns/markers.js` holds 44 ids, none of
them under those two prefixes.

The practical consequences, each of them a limitation and not a footnote:

| What you might expect | What actually happens |
|---|---|
| `procoder check` reports a FAST or MEANT finding | It cannot. Every finding the CLI, the hook and the MCP server can emit is at rung 1–4. |
| `[rungs] fast = "error"` promotes rung 5 to blocking | It is inert. The key parses, `levelFor` and `isBlocking` read it, and nothing ever asks the question because nothing ever produces a finding to ask it about. |
| A line marker naming a `fast/*` id suppresses something | There is no id to name. A well-shaped id under either prefix is not in the known set and warns `unknown rule id … — it suppresses nothing`. |
| CI gates on cost or scope creep | It does not. A query per row over a request-sized list and a rename nobody asked for both pass `procoder check` silently. |

This is the structural limitation of the whole tool, stated at its sharpest:
the doctrine is what the model reads, the engine is what CI runs, and for two
of the six rungs the overlap is empty. Rungs 5 and 6 are enforced by a reader
holding the change against the ask, and by nothing else. Neither can be
decided from one file — FAST needs the size the input really reaches, MEANT
needs the request the diff answers to — which is why they default to `warn`
rather than why they are unimplemented, and the default is currently a
statement about a value that never arrives.

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

### Where taint dies

Four rung-1 false positives on correct code that this page used to list are
gone, and the rows are deleted rather than hedged. Verified silent, each on the
file the page used to say reported: `q = sanitizeSql(q)` before the sink;
`mysql.escape(id)` concatenated into a query; `escapeHtml(x)` assigned to
`innerHTML`; and `redis.execute(cmd)` in a module that also runs SQL.

Two mechanisms did it, and the shape of each is what the remaining limitations
are made of.

**Taint dies at a call whose callee is *named* as sanitising, or is a known
per-ecosystem driver escape** — and the sanitiser's whole argument list is
skipped, so the value inside it is not judged on its own either. The verb has
to *begin* the callee's last segment (`sanitizeSql`, `escapeHtml`,
`quoteIdent`), and `escape`, `quote` and `sanitize` spelled bare count only on a
receiver that names the library (`mysql.escape`, `DOMPurify.sanitize`). The
list of per-ecosystem escapes with no verb in the name is in `SANITIZER_VERB`,
`BARE_VERB`, `ESCAPE_RECEIVER` and `NAMED_CLEANER` in
`hooks/checks/lang/taint.js`.

**A call made on a receiver from `NON_DB_RECEIVER` is not a database call,
whatever the file contains** — `redis`, `cache*`, `queue*`, `job`, `runner`,
`api`, `http*`, `kafka`, `s3` and the rest. This subtracts from the file-level
evidence only; an unknown receiver is unchanged.

Both are allow-lists, and an allow-list is a decision to trust a name. Each of
the five rows below is a **false negative** that decision buys — a real defect
the scan now reports nothing for:

| Trusted, and wrong | The file |
|---|---|
| **The wrong escaper for the sink** | `db.query(escapeHtml("SELECT * FROM t WHERE id=" + id));` is silent. The name says escaping; nothing asks escaping *for what*, and HTML escaping does not make a query safe. Every cross-context pairing goes the same way — `escapeSql` into `innerHTML`, `htmlEscape` into `exec`. |
| **Escaping the assembled query rather than the value** | `q = "SELECT * FROM t WHERE id=" + id;` then `q = sanitizeSql(q);` then `db.query(q);` is silent, and it is not a fix: the injection is already in the string by the time the escaper sees it. Verified in JS, and in Python (`sanitize_sql(q)`) and Go (`escapeSql(q)`) alike. This is the same probe that used to be this page's first false positive; closing the false positive opened this. |
| **Anything merely *named* `sanitizeX` / `escapeX` / `quoteX`** | `q = sanitizeNothingAtAll(q); db.query(q);` is silent. The allow-list is a naming convention, not a contract, and a function that only looks like an escaper is trusted exactly as far as one that is. |
| **A sink nested inside a sanitiser's arguments** | `const out = escapeHtml(db.query("SELECT * FROM t WHERE id=" + id));` is silent. The sanitiser's whole argument list is skipped — which is what stops `q = sanitizeSql(q)` reporting — and the inner sink goes with it. The taint path still reaches it when the value was bound on an earlier line: with `const q = "SELECT id=" + id;` above, the same statement reports at the sink, built at line 1. |
| **A real database handle named `cache`, `api` or `queue`** | `const cache = new Pool();` in a file importing `pg`, then `const q = "SELECT * FROM t WHERE id=" + id; cache.query(q);` — silent. So is the same file with the handle named `api` or `queue`. The receiver list is a guess about intent from a name, and a name is the one thing a project is free to choose badly. |

Kept on purpose, and still **false positives on correct code**:

| Probe | Reported | Why it is wrong |
|---|---|---|
| `let q = "SELECT id=" + id;`<br>`q = clean(q);`<br>`db.query(q);` | `safe/sql-injection` | A project-local sanitiser whose name carries no sanitising verb is not on the allow-list. `harden(q)` and `safen(q)` are the same shape, and `clean(q)` and `harden(q)` are both verified reporting. The direction is deliberate: the alternative is a wrapping call clearing taint by default, which is the whole scan giving up at the first unknown call. A miss here is a shipped injection; this is one line a reader can see is wrong. |
| `q = f"run job {x}"`<br>`pool.execute(q)` | `safe/sql-injection` | `execute` is not a database verb here. One of the three things that count as file-level evidence is a *receiver name* — `db.`, `cur.`, `conn.`, `stmt.`, `session.`, `pool.`, `repo.`, `tx.` — and those are ordinary names outside a database: a worker `pool`, an HTTP `session`, a git `repo`. `pool` is not on the non-database list for the same reason it is on the evidence list. Verified: `runner.execute(q)` and `cmd.execute(q)` on the same input are silent, `pool.execute(q)`, `session.execute(q)` and `repo.execute(…)` report. |

The reverse of the database gate is the matching **silent coverage loss**: a
genuine injection in a file that contains no database vocabulary at all is
missed. Verified, a file that reports **nothing**:

```
const q = base + req.query.id;
client.query(q);
```

`client` is not a canonical handle and the file holds no SQL keyword, no driver
name and no ORM name, so the whole rule stands down. Add one line of evidence
and it comes back — `const { Client } = require('pg');` above the same
`client.query(q)` reports, verified. The same file written with `api.execute(q)`
is silent *either way* now, evidence or not, because `api` is on the
non-database receiver list. A per-call escape hatch exists for the unambiguous
method forms (`executeQuery`, `rawQuery`, `QueryRowContext`, `CommandText`), but
it only reaches the *line* rules: the taint sinks match `query`, `execute` and
`raw` as whole words, so `api.executeQuery(q)` on a bound name is silent in a
vocabulary-free file too, verified. This bought back a large class of false
positives on Command-pattern and job-runner code; the price is that a query
assembled from fragments, in a file whose SQL lives elsewhere, is not seen.

`safe/xss-sink` fires on the *assignment target* (`el.innerHTML = …`), so it is
reported whichever way the value arrives. Constant markup is silent —
`el.innerHTML = "<b>static</b>";`, the backtick form, and a name bound only to
literal markup are each verified silent — and an already-escaped value is silent
too: `const safe = escapeHtml(x); el.innerHTML = safe;` and
`el.innerHTML = escapeHtml("<b>" + name + "</b>");` both report nothing. Any
*other* call still does: `el.innerHTML = render(x);` is a finding, verified. The
rule tells a named escaper from an unnamed one, and nothing else.

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

Three more cases this page listed are closed, and are deleted from the table
below rather than hedged. Verified, each on the file this page used to say
reported nothing:

- **A JS/TS method shorthand named after a JS statement is measured.** A brace
  that opens a class body or an object literal holds members, and a member is
  a declaration however it is named; every other brace opens a block. `class R
  { with(a0…a5) { … } }`, the same with `catch`, `for` in TypeScript, and
  `const o = { switch(a0…a5) { … } }` each report `6 parameters (limit 4)`,
  where all four were invisible to every shape rule before.
- **Rust raw identifiers are measured.** `fn r#match(a0…a5)` and `fn
  r#try(a0…a5)` both report `6 parameters (limit 4)`. So does C#'s spelling of
  the same idea, `public int @match(int a0 … int a5)`.
- **A template literal containing a quote character no longer desynchronises
  the brace scan.** Five shapes verified, `` new RegExp(`(?:^|["'/])${x}`) ``
  among them: the enclosing function's closing brace is counted and the next
  function keeps its own span — `function is 48 lines (limit 40)` is reported
  against the 48-line function that follows, at its own line, not against the
  three-line one holding the template. The workaround in
  `hooks/checks/deps.js` is now belt and braces rather than load-bearing.

What is left:

| Case | Effect |
|---|---|
| A parameter list on a very long line | No ceiling remains. `obvious/too-many-params` is not span-derived, so the 4KB per-line shape guard no longer covers it: a 1,000-parameter single-line signature (over 8KB) is measured and reported in JS/TS. |
| A C# type whose whole body is on one line | Not measured at all. `class A { public int m(int a0 … int a5) { return 1; } }` on a single line reports nothing, raw identifier or not; the same class written over five lines reports `6 parameters (limit 4)`. The C# head is anchored to end of line, and a one-line class puts the body there too. |
| Any language outside the six packs | `.kt`, `.kts`, `.rb`, `.php`, `.swift`, `.scala` and everything else resolve to no pack, so no shape rule and no SAFE rule of any pack runs over them. Verified: a Kotlin `fun matcher(a0…a5)` with six parameters reports nothing. Only the universal pack (rung 1, credentials and suppressions) reads them. |

Swapping in a real per-language parser is the fix; it has not happened in
0.2.0.

## The three newest rules, and what each of them misses

`safe/redaction-marker`, `safe/manifest-not-locked` and `true/missing-timeout`
are the three the doctrine had and the engine did not. Each is a single
regex or a single lookup, and each was probed the way the taint rules above
were: smallest file that should fire, smallest file that should not.

### `safe/redaction-marker`

One pattern, `\[REDACTED[:\]]`, over every line of every file the universal
pack reads. Verified reporting, in JavaScript and Python alike:

<!-- procoder: literal safe/redaction-marker the two shapes the rule matches, quoted -->
`const a = "[REDACTED]"` and `const b = "[REDACTED:aws-key]"`.

| Missed | The file |
|---|---|
| Any other spelling a redaction layer might use | `"***REMOVED***"`, `"<REDACTED>"` and `"[REDACTED_BY_SCANNER]"` are each silent. The bracket and the following `:` or `]` are both required, so an underscore, an angle bracket or a different word matches nothing. |
| Lower or mixed case | `"[redacted]"` is silent. The pattern is case-sensitive, alone among the universal pack's rules — `safe/hardcoded-secret` beside it is not. |
| An unterminated marker | `"[REDACTED"` is silent, which is the shape a truncated write leaves behind. |

The false positive is the mirror image and is wide: **any file that talks
about redaction reports one.** Verified, a Markdown file holding one sentence
and no code at all:

<!-- procoder: literal safe/redaction-marker the reproducing input, quoted -->
`The scanner replaces the value with [REDACTED] before the model sees it.`

That file reports `safe/redaction-marker`. So does a comment, a test name, a
changelog entry and this document, which carries two line markers for exactly
that reason. The rule has no notion of code versus prose, and every
documentation file that explains the mechanism has to be marked by hand.

### `safe/manifest-not-locked`

npm only, and text-matched. `checkNpmLocked` in `hooks/checks/deps.js` reads
the lockfile sitting **next to the manifest** as one string and asks whether
each dependency name appears in it delimited by a quote, a slash, an `@` or a
colon. Verified reporting: `left-pad` in `dependencies` with a
`package-lock.json` that does not name it; verified silent when it does.

| Missed | The file |
|---|---|
| Every ecosystem except npm | `Cargo.toml`, `go.mod`, `pyproject.toml` and `Directory.Packages.props` get `safe/missing-lockfile` and nothing else — no entry-by-entry check exists for them. Verified: a `requirements.txt` reports only the missing-lockfile finding. |
| `optionalDependencies` | Outside `DEP_BLOCKS`. Verified: `{"optionalDependencies": {"fsevents": "2.3.3"}}` against an empty lockfile is silent, where the same entry under `dependencies` reports. |
| A direct dependency the lock knows only as somebody else's transitive | The lockfile is one string, not a resolution. Verified: `ms` in `dependencies`, and a lockfile whose only mention of it is inside `debug`'s own `dependencies` map, is silent — the direct edge was never recorded and the rule cannot see the difference. |
| A workspace package | The lookup is `path.dirname(manifestPath)`, so a monorepo's `packages/a/package.json` with the lock at the repository root finds no lockfile, and `checkNpmLocked` returns before checking anything. Verified: it reports `safe/missing-lockfile` — itself a false positive, the lock is one directory up — and never reaches the per-entry rule. |
| A manifest `JSON.parse` cannot read | Returns silently. Verified: a `package.json` with a trailing comma reports nothing at all, and nothing on stderr says the manifest was skipped. |

The false positive is `peerDependencies`, which is in `DEP_BLOCKS`. A peer
dependency is by definition the consumer's to install and is legitimately
absent from this package's lockfile. Verified: `{"peerDependencies":
{"react": "^18"}}` reports `react is in peerDependencies and not in the
lockfile` — correct code, blocking finding, on a library's own manifest.

### `true/missing-timeout`

Two packs and two shapes: Python's `requests.<verb>(…)` where the call opens
and closes on one line with no `timeout=` inside 300 characters, and Go's
empty `http.Client{}` literal or a package-level `http.Get`/`Post`/`PostForm`/
`Head`. Verified reporting: `requests.get(url)`, `c := &http.Client{}` (with
or without inner whitespace) and `http.Get(u)`.

| Missed | The file |
|---|---|
| Four of the six packs | There is no timeout rule in the ts, jvm, dotnet or rust packs. Verified silent: `await fetch(url)` in JS, and `HttpClient.newHttpClient().send(r, h)` in Java. |
| Every Python HTTP client but `requests` | `httpx.get(url)`, `urllib.request.urlopen(url)` and an `aiohttp` session `get` are each silent. |
| A `requests.Session` | `s = requests.Session()` then `s.get(url)` is silent — the receiver is a bound name, not the module. |
| An unqualified import | `from requests import get` then `get(url)` is silent; the literal `requests.` prefix is required. |
| A multi-line call | `requests.get(\n    url,\n)` with no timeout anywhere is silent, and deliberately so — a timeout on a later line would otherwise be reported as missing. |
| A Go client literal with any other field | `&http.Client{Transport: tr}` and no `Timeout` is silent, deliberately, for the same reason. |
| `http.DefaultClient` used directly | `http.DefaultClient.Do(req)` is silent, although the rule's own comment names `http.DefaultClient` as the trap. Only the four package-level helpers and the empty literal match. |

Two false positives, both from the same cause — the rule reads the argument
list as text and cannot follow a name:

| Probe | Reported |
|---|---|
| `requests.get(url, **kw)` | `true/missing-timeout`, whatever `kw` holds. |
| `DEFAULTS = dict(timeout=5)` then `requests.get(url, **DEFAULTS)` | `true/missing-timeout`. The timeout is passed; the word `timeout` is not on the call's line. |

Both block at rung 2 at every level but `pragmatic`.

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

Measured today over **209** tracked files, `procoder check .` reports **0**
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

So **191** files are actually read, and README.md publishes those three
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

- **All three instruments now have an expiry test, and the third one has two
  deliberate gaps.** `unusedPathExclusions` in `hooks/checks/config.js` reports
  a configured
  path exclusion three ways — its path no longer exists, it matches no file in
  the tree, or every file it covers is clean — under plain `verify`, failing
  the build only under `--unused-exclusions`, which is the contract
  `unusedRuleExclusions` has always had. Verified in a throwaway repo:
  `paths = ["keep/", "**/*.gen.ts"]` with `keep/` holding one clean file and no
  `.gen.ts` anywhere now names both, and exits non-zero under the flag. The two
  tree-wide rules are decided only when the run's targets covered the whole
  repository — "this glob matches nothing" is a claim about the tree, and a
  `verify` over one file has not seen it — so a partial run still says nothing,
  deliberately. `unusedIgnorePatterns` judges a `.procoderignore` the same way
  and on the same contract, one pattern at a time, naming file, line and the
  pattern as written: reported when every *tracked* file it covers scans clean
  with that one line lifted, or when a glob matches nothing in the walked tree.
  The two gaps are on purpose. There is no "the path is gone" rule — a literal
  path in an ignore file is a fence around a location, and such a location is
  legitimately absent from a fresh clone, as this repository's own `.claude/`
  and `.superpowers/` lines are in CI. And a pattern covering only untracked
  files is never judged: content the repository does not own cannot go clean in
  any sense procoder can verify. Where `git ls-files` cannot answer, no ignore
  pattern is judged at all. Two more deliberate limits on the same instrument:
  a **negated** pattern (`!keep.js`) is never judged, because "this exception
  stopped mattering" is not a claim about coverage lost; and an ignore file
  that some other ignore file covers is not judged either, since it is not in
  the walked tree to be found. All three of this repository's own ignore files
  are silent under these rules, by rule rather than by exemption.
- **No staleness report of any kind reaches `--format json` or `sarif`.** All
  three instruments — unused rule exclusions, unused path exclusions, stale
  ignore patterns — are `verify` output, `--format` is refused off `check`, so
  the two can never meet. That is deliberate: a suppressed line of config is
  not a code finding, and SARIF has nowhere honest to put one. The consequence
  is that a dashboard-only pipeline sees none of it and has to run a second,
  text-mode `verify` to learn that a whole directory has been quietly
  un-gated.
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

## The parallel scan, and the threshold that is the wrong shape

`hooks/checks/scan.js` splits the file list into contiguous slices and hands
each to a child process running `hooks/checks/worker.js`. The good properties
hold, and were verified rather than assumed: over a 260-file tree, `--jobs 1`
and `--jobs 4` produce byte-identical output, and a run in which **every**
worker is made to exit non-zero still returns all 260 files, in input order,
with the same 260 findings — the parent rescans a dead worker's slice itself.
No file has been observed to go missing, and a short or unparseable result
from a child is discarded wholesale rather than merged, so a truncated
document cannot become a clean file.

Three divergences this page used to carry are closed, and are deleted rather
than hedged:

- **The time budget is one number down both paths.** `SLICE_BUDGET_MS` — the
  600,000 ms a worker used to run each file at, against this process's
  2,000 ms — is **deleted**. The parent puts `budgetMs` in the payload and
  passes the same value on the sequential path, and a test pins it to
  `BUDGET_MS` in `run.js`. Verified: `require('hooks/checks/scan.js')` exports
  `SCAN_BUDGET_MS` at 2,000 and no `SLICE_BUDGET_MS` at all. So
  `true/budget-exhausted` fires, or does not, for the same reason at any
  `--jobs`. What made 600,000 ms reachable was never a large file — the most
  adversarial 1 MB file built here costs well under the budget — it was a
  **non-batchable linter**: `cargo clippy` has no `argvMany` or `parseMany`, so
  it runs once per file, and 1.2 s of sequential clippy became six minutes
  inside a worker that had no deadline to stop it.
- **The worker no longer re-reads `.procoder.toml`.** The parent's config
  object travels in the payload, so a caller-built config governs every file
  rather than only the ones this process scanned, and a parse warning is printed
  once rather than once per worker. Loading from disk survives as the fallback
  for a payload that carries no config.
- **A hung worker is no longer unhandled.** Worker *death* was already handled;
  a live worker that never wrote a byte was not, and there was no timeout
  anywhere in the pool, so the scan simply never returned. A slice now has a
  derived bound — 5 s of startup grace plus two budgets per file — past which
  the worker is `SIGKILL`ed, a line naming the timeout and the slice size goes
  to stderr, and the slice is rescanned in this process, exactly as a dead
  worker's already was.

**`--jobs` is clamped**, on the same terms `max_file_bytes` has always had, and
in `scanFiles` rather than in the CLI, so an API caller gets it too.
`MAX_JOBS` is **8**. Verified, each by running it:

| `--jobs` | What happens |
|---|---|
| `1`, `4`, `8` | honoured, silently |
| `9`, `9999` | `procoder: --jobs 9999 is above the ceiling 8 — using 8. More workers than cores only take turns, and each one costs a process.` |
| `0`, `-3`, `abc`, `NaN`, `Infinity`, empty | refused with `procoder: --jobs … is not a number of worker processes — want a whole number from 1 to 8; using 8`, and the default used |

Two edges on that, both verified and neither fixed:

- **A fractional value is floored, silently.** `--jobs 2.5` prints nothing and
  scans with 2. `Math.floor(2.5)` is a usable count, so it never reaches the
  refusal; only a value that floors below 1 does.
- **The warning does not always name what you typed.** The CLI runs the flag
  through `Number()` before `clampJobs` sees it, so `--jobs abc` warns about
  `--jobs NaN` and `--jobs Infinity` warns about `--jobs null` — `clampJobs`
  has code to show the value as written, and the conversion upstream defeats it.

**The default job count is sized by `os.availableParallelism()`, not
`os.cpus()`.** `os.cpus()` reports the *host's* core count straight through a
cgroup CPU quota, so a container run at `--cpus 1` reported 10 and forked eight
workers: measured on 4,000 files, `--cpus 1` cost 14.9 s sequential and 35.0 s
at four jobs — the same shape of loss `--jobs 9999` used to buy on a laptop,
with nobody typing anything to get it. `os.cpus()` remains the fallback, and the
floor of 1 remains, because it is documented to be able to return nothing.

### `PARALLEL_MIN_FILES` is 250, and a file count is the wrong threshold

This one is **not fixed**, deliberately, and it is the sharpest thing on this
page about the pool. Forking pays for itself when a file costs more than a fork
does, so the crossover is a **work** threshold. `PARALLEL_MIN_FILES` counts
files. Measured here, best of three runs each, through `scanFiles` directly so
that the fork is reached below the threshold:

| Input | Files | `--jobs 1` | 8 workers | Ratio |
|---|---|---|---|---|
| Real source (this repository's own language packs, cycled) | 50 | 334 ms | 391 ms | 0.85 |
| | 75 | 638 ms | 535 ms | **1.19** |
| | 100 | 814 ms | 667 ms | 1.22 |
| | 250 | 2,127 ms | 1,168 ms | 1.82 |
| Trivial one-liners (`const a0 = 0;`) | 250 | 18 ms | 108 ms | **0.17** |
| | 1,000 | 67 ms | 120 ms | 0.56 |
| | 2,500 | 142 ms | 188 ms | 0.75 |
| | 4,000 | 226 ms | 200 ms | 1.13 |

So real third-party source crosses over between **75 and 100** files, and
trivial ones not until somewhere between **2,500 and 4,000** — where a
250-file threshold is **six times slower** than not forking at all (108 ms
against 18 ms). One constant cannot be right for both, and 250 is wrong for both
in opposite directions: it leaves the 75-to-250 band of real source scanning
sequentially when forking would have been up to 1.8× faster, and it forks a
tree of trivial files ten times too early. Sizing it by bytes, or by a sampled
cost per file, is the fix; it has not happened in 0.2.0.

The rest of the pool's edges, each verified:

| Limit | Measured |
|---|---|
| **Below 250 files nothing is forked**, and this repository has 209 tracked files | `procoder check .` over procoder itself is therefore always sequential, and the parallel path is not exercised by the self-scan. Tests reach the pool through a `forceParallel` option that exists for that reason and no other. |
| **Linting is not parallel** | `runToolBatches` runs before the pool, in the parent, one spawn per tool for the whole run. A slow linter is wall clock nobody's cores help with. |
| **Workers inherit stderr** | Anything a worker writes there arrives interleaved with the parent's own notices, unordered between slices. |

## Machine-readable output: what SARIF still loses

`procoder check --format json` and `--format sarif` are built from the same
per-file results the text path prints, so neither can report a finding the
default did not. The gap is the other direction — what the document leaves
out.

**JSON** carries a `version` field (currently `1`), the session level, a
`summary` (`findings`, `blocking`, `advisory`, `pinned`, `level`), every
finding with its rung name and number, id, file, line, message, fix, whether
it blocked, and the ratchet's own fingerprint — and a `skipped` array. Verified
over a tree holding one oversized file, one ignored file and one excluded file:

```
"skipped": [{"file":"big.js","reason":"too-large"},
            {"file":"ign.js","reason":"ignored:.procoderignore"},
            {"file":"sub/x.js","reason":"excluded"}]
```

**SARIF carries the skipped files too, now.** The row that used to head the
table below — a file nobody read arriving at a dashboard identical to a clean
one — is deleted, not hedged. Skips travel as
`invocations[0].toolExecutionNotifications`, with the descriptors they name
declared in `tool.driver.notifications`. Notifications rather than results: a
file nobody opened has no finding in it, and inventing one would report a
problem in an innocent file and fail every build that gates on result count.
Verified on the same tree, plus an unreadable file:

```
"notifications": [{"id": "procoder/skipped/too-large", …}, …]
"invocations": [{"executionSuccessful": false,
                 "toolExecutionNotifications": [
                   {"descriptor": {"id": "procoder/skipped/too-large"},
                    "level": "error",
                    "message": {"text": "big.js was not checked: larger than
                                [limits] max_file_bytes (too-large)."},
                    "locations": [{"physicalLocation":
                                   {"artifactLocation": {"uri": "big.js"}}}]},
                   …]}]
```

Five descriptor ids exist — `procoder/skipped/` plus `excluded`, `ignored`,
`too-large`, `unreadable` and `other`. The level split is the one the CLI
already makes on stderr and `verify` already gates on: deliberate scope loss
(`[exclude] paths`, a `.procoderignore`) is **`warning`**; a file that was in
scope and could not be read (over the size cap, unreadable) is **`error`**; and
`executionSuccessful` is false exactly when an error-level notification exists —
the same predicate `verify` uses when it refuses to claim a ratchet. Anything
unrecognised maps to `other` at `error`, so a skip reason nobody mapped shows up
as a hole rather than as a clean file. A run with nothing skipped emits no
`invocations` and no `notifications` at all: the document is byte-identical to
what it was before this existed.

**`automationDetails` was deliberately not used**, and it is worth saying so,
because it is the kind of thing someone will otherwise "fix". GitHub treats
`runs[].automationDetails.id` as the category that groups one analysis with the
next, so an id that appeared only on runs that skipped something would split a
project's alert history in two the first time a file went unread.

What SARIF still leaves out:

| Lost in SARIF | Why it matters |
|---|---|
| **The run's level** | `blocking` collapses into SARIF's `level: error` / `warning`. The same finding is `error` at `strict` and `warning` at `pragmatic`, and the document does not say which ran, so two dashboards over the same code can disagree with nothing to arbitrate them. |
| **The exit code** | `executionSuccessful` answers "could this run see everything it was asked to", not "did it pass". A run with findings and nothing skipped has no `invocations` block at all and reads as successful; the process's own status is still the only place the gate's verdict lives. |
| **Anything the baseline accepted** | `check` applies the baseline, so an accepted finding is absent from both formats. A dashboard therefore shows fewer findings than the repository has, by design, with no count of the difference. |
| **`note` severity is unused** | Every finding is `error` or `warning`. Deliberate — grading half of them to a level most dashboards hide by default would be a quiet way to lose them — but it means rung 3 and rung 4 arrive as warnings a busy dashboard treats as lint noise. |

Rungs survive: `rungNumber` and `rung` travel on each SARIF rule object, and
`partialFingerprints.procoderFingerprint` is the same fingerprint the ratchet
uses, so a moved line is not a new finding. `--format` is refused on anything
but `check`, so `verify` — the ratchet, the aging report and all three
staleness instruments — has no machine-readable form at all.

## `--since <ref>`: what it does not see

`--since` asks git for three lists — `diff --diff-filter=ACMRT <ref>...HEAD`,
the same against `HEAD` for uncommitted edits, and `ls-files --others` for
untracked ones — and checks the union. Its commit message says it "cannot pass
green on nothing"; run against it, that claim now holds every way it is tested
here.

The filter is `ACMRT`, and the two letters that are not in it are as deliberate
as the ones that are. **`R`** is a rename, and `--name-only` gives the
destination, so a renamed file is checked at the path it lives at now. **`C`**
is a copy and **`T`** a type change: both are content arriving under a path
nothing has checked. **`D`** stays out — a deleted file has no bytes to scan,
and a rename's destination is already selected — and **`U`** (unmerged) with
it. Verified end to end in a throwaway repository: after `git mv dirty.js
renamed.js`, `procoder check --since HEAD~1` reports `renamed.js:1
safe/sql-injection` and exits 1; a copy of the same file and a file turned into
a symlink each report at the new path; a commit that only deletes a file still
prints `no files changed since HEAD~1 — nothing to check.` and exits 0.

Holds, verified:

- A ref git cannot resolve **exits 2**, naming the git command and its error,
  rather than checking nothing quietly: `--since no-such-ref` and a ref that
  resolves to a blob both exit 2.
- Nothing changed is said out loud — `procoder: no files changed since <ref> —
  nothing to check.` — not passed over in silence.
- The dirty working tree is included. A modification to a tracked file and a
  brand-new untracked file are both checked before either is committed.
- A merge commit is handled: `...` is a merge base, so `--since <first
  parent>` over a merge picks up the side branch's files.

`--since` is **not "check only" any more**, and any page still saying so is
stale. `--aging` and `--unused-exclusions` are honoured rather than dropped.
Verified in a throwaway repository, in the same second:
`procoder verify --aging 0 --since HEAD~1` names the accepted finding with its
date, path and rule and exits **1**, exactly as `procoder verify --aging 0 .`
does — the baseline is what `--aging` judges, and no file selection narrows
that.

Does not hold, or holds only partly:

- **A deletion is not checked, and nothing looks at what it left behind.** A
  commit that only removes a file exits 0 with `nothing to check` — correct
  for the deleted file, and the rung-4 question a deletion actually raises
  (who still calls it?) is `procoder rot`'s, which `--since` does not run.
- **`--unused-exclusions` under `--since` makes fewer claims than it looks
  like it does.** It runs, but `wholeTree` is false, exactly as for any other
  partial-scope run: "this glob matches nothing" and "everything this path
  covers is clean" are claims about the tree, and a run over four changed files
  has not seen it. Only the claim about what this run actually read is
  enforced. Verified: `verify --unused-exclusions .` names two dead path
  exclusions and exits 1, while `verify --unused-exclusions --since HEAD~1` on
  the same tree names none and exits 0. Nothing is wrong there — but a CI job
  that runs only the `--since` form will never be told a directory has been
  quietly un-gated.
- **`verify --since` is a partial ratchet against a whole baseline.** It
  compares the findings of the changed files against every accepted entry, so
  it cannot see growth in a file the diff did not touch. That is the point of
  the flag; it is worth knowing that a green `verify --since` is a much
  weaker claim than a green `verify .`.
- `--since` is written up under `check` and is not enforced as such: it runs on
  `baseline` and `verify` too, which is useful, and which the `--since` entry in
  `--help` now half-documents by naming the two `verify` flags it honours.

## `procoder init`, the MCP server, and dated baseline entries

Three smaller surfaces, each with one edge worth knowing.

**`procoder init`** writes `.procoder.toml` and nothing else. The two other
templates that sit beside it in `scripts/templates/` — the pre-commit hook and
the CI workflow — are referenced by no code and are still copy-and-paste. It
branches before argument validation, so every flag is accepted and ignored:
`init --format sarif --aging -5` exits 0 having done nothing with either, where
`--aging -5` on `verify` exits 2. Outside a git repository it writes into the
working directory rather than refusing, with nothing in the output
distinguishing that from writing at a repo root. And it does not validate a
config that already exists: a `.procoder.toml` full of junk is left as it is
and reported as fine, and the junk shows up only as a parser warning on the
next `check`.

The one that matters: **`init --baseline` exits 0 over files it could not
read.** Verified — with an oversized file and an unreadable one in the tree,
`init --baseline` names both on stderr, records the baseline and exits **0**,
then advises running `procoder verify .`, which exits **2** on the same tree
because the ratchet cannot hold over files nothing looked at. Setup passes;
the gate it just set up cannot run.

**The MCP server** (`procoder-mcp/`) exposes four tools — `procoder_doctrine`,
`procoder_check`, `procoder_review`, `procoder_baseline` — and is a strict
subset of the CLI. It has no `verify`, no baseline *writing*, no `rot`, no
`init`, no `--format`, no `--jobs`, no `--aging` and no `--unused-exclusions`;
`--since` exists only inside `procoder_review`. Every answer is a prose blob,
so a client cannot gate on one without parsing English. Three specific edges:

- **`procoder_check` truncates at five findings, silently.** It takes
  `checkFile`'s default `maxFindings`, which is the PostToolUse hook's 5, and
  prints no overflow notice. Verified: a file holding twelve credential
  literals answers with five; `procoder check` on the same file reports
  twelve. `procoder_review` passes `Infinity` and does not truncate, so the
  two tools disagree about completeness and neither says which you got.
- **`procoder_review` leads with the word `clean` over a diff it did not
  read.** Verified with two changed files, both over `max_file_bytes` and both
  carrying a live credential: `clean: 2 changed files (2 files skipped)` — no
  filename, no reason, and the token a model keys on first is `clean`. The CLI
  in that exact state exits 2.
- **Every failure is a successful tool result.** A git error inside
  `procoder_review` comes back as content with no `isError` flag, and a path
  that does not exist comes back from `procoder_check` as `skipped
  (unreadable)`. The prose is honest; the machine-readable signal is uniformly
  success.

**Baseline entries now record what they accepted and when** — `{fp, id, path,
added}`, with `added` a `YYYY-MM-DD` stamp that `writeBaseline` preserves
across re-runs. `--aging <days>` turns that into a CI question. What it does
not give you:

- **Nothing ever prunes the baseline.** `runBaseline` seeds from the existing
  file and only appends, so an entry whose finding was fixed stays forever.
  Verified: fix every finding in the tree, re-run `procoder baseline .`, and
  the file still holds 3 accepted entries for 0 present findings — and
  `verify --aging 1` then fails CI naming files that no longer contain the
  finding. Editing a baselined line mints a second entry rather than replacing
  the first, so the file grows monotonically with every edit to accepted code.
  There is no prune command; deleting the JSON and re-baselining is the only
  remedy.
- **A v3-migrated entry is pinned to `unknown` forever.** `verify --aging`
  over an old baseline prints `unknown  unknown  unknown` and says the next
  `procoder baseline` will stamp it. It does not: `writeBaseline` keeps the
  first entry it has for a fingerprint and treats the string `unknown` as a
  date already present. Such an entry matches every `--aging` value, fails
  `verify --aging` permanently, and names no file, no rule and no date. Since
  the format is now v4 and a stale-version baseline loads as empty (below),
  the reachable case is a hand-edited or hand-migrated file — but the printed
  remedy is false where it is reachable.
- **`added` is data the user owns.** It is hand-editable, nothing signs it,
  nothing cross-checks it against git, and it is read by exactly one thing:
  `--aging`. A date edited to last week resets the clock on a year-old
  suppression, silently.
- `--aging` is `verify`-only and refuses negatives and non-numbers with exit 2;
  `0` and fractional values are legal and report everything.

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
| **A configured linter's thresholds replace procoder's, including where it has none.** `pub fn wide(a0..a5)` reports `[3 OBVIOUS] 6 parameters (limit 4)` with no `[lints.clippy]` in `Cargo.toml`. Add it and the same command reports clippy's findings instead — verified today, `[2 TRUE] writing &Vec instead of &[_] …` and no `6 parameters` line: clippy answered, the Rust pack's `obvious/*` rules are dropped, and clippy's own `too_many_arguments` threshold is 7. That is the design — the project's linter defines OBVIOUS — but the practical effect is that configuring clippy loses every 5- and 6-parameter report. |
| **One whole-crate invocation per Rust file scanned, and cargo is the one linter the batching does not reach.** `runToolBatches` runs eslint, ruff and golangci-lint once for the whole scan; `cargo` has no `argvMany` and is not batchable — verified, `canBatch(resolveFor('src/lib.rs'))` is `false` — so a directory scan still spawns it once per file. Measured today on a 33-file crate with `[lints.clippy]` present: `procoder check .` costs **1.6–1.7 s** warm, ~50 ms per file, since cargo replays a cached build. A crate that takes longer than the per-file timeout to compile gets no clippy findings at all. |
| **A batch that names some files and not others marks the rest clean.** `allClean` in `resolve.js` fills in every file the tool did not mention, on the reasoning that these linters report every file they linted. A tool that silently skips a file — its own ignore list, a config exclusion — therefore counts as having answered for it, and the pack's `obvious/*` rules are dropped for a file nothing linted. ruff's integration guards this explicitly (no `--force-exclude`, `invalid-syntax` declines) and eslint's decline rules are applied per batch entry; **golangci-lint has no such guard in the batch path**. Not reproduced: a `.golangci.yml` excluding the file under test left the pack's `6 parameters (limit 4)` intact here, so the shape is a reasoned risk in the code rather than an observed loss. |

`eslint` and `ruff` were not installed on the machine this was verified on, so
their integrations — including their batch paths, which are the ones the CLI
now uses — are undisclosed rather than cleared.

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

`BASELINE_VERSION` in `hooks/checks/baseline.js` is 4 — it moved from 3 when
entries began recording their rule, path and date; earlier fingerprints
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
| 2s budget, and what it cuts | `hooks/checks/run.js` | Checks after the deadline are skipped. The order was reworked: the language pack now runs **before** the project's linter, and the linter is paid a *share* of what is left on the clock (0.6, floor 100ms) rather than a precomputed split, so the total is under budget by arithmetic on any host rather than by a constant measured on one. The per-MB reserve is deleted. Worst case at a 3× slowdown went from 1797ms to 1448ms of 2000ms, and holds to 6×. What the deadline does cut is now loud on both channels: a `true/budget-exhausted` finding appended **after** the per-file cap, so five SAFE findings cannot push it out, plus a stderr line naming the stage — verified with a zero budget, `procoder: package.json: 0ms budget exhausted — not checked: the dependency manifest rules`. The universal pack (rung 1) runs first of all and can never be the stage that is cut. **This budget is uniform**: the parent puts it in the worker's payload and passes it on the sequential path too, so a scan slice in a worker runs each file at the same 2,000ms — see the parallel-scan section above. What it costs is stated there: on a project whose linter cannot be batched (`cargo clippy`), a slice that used to run clippy to completion now cuts it at its share of 2,000ms and says so. |

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
