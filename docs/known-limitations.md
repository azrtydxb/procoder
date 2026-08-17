# Known limitations

procoder gates other people's code. That obligates candour about where the
gate is weak. Every entry below was verified by running the tool against a
throwaway file, at 0.2.0, and quotes the number that run produced. Where a
previous edition of this page disclosed something that has since been fixed,
the entry is gone rather than kept as false modesty — a document that
overstates weakness is as untrustworthy as one that hides it.

**What this page is now, and what it stopped being.** For five editions it
mixed three different kinds of entry, and the repository owner's instruction is
the one that sorts them: *a decision we made deliberately is not a limitation.*
So a decision that does not cost detection — the ratchet's fingerprint
semantics, the TOML subset, the baseline's version wall, the ceilings the
performance work left behind, the self-scan's own 18 skips — has moved to
[**Why it works this way**](https://azrtydxb.github.io/procoder/design.html),
which is rationale and reads as such. A decision that *does* cost detection
stays here, gathered under [Accepted trade-offs](#accepted-trade-offs), where
each one says what it buys and not only what it costs. Everything else is a
genuine gap and is written up as one.

This pass covers four engine changes. Rungs 5 and 6 have an engine, three rules
wide; `true/missing-timeout`, `safe/manifest-not-locked` and
`safe/redaction-marker` were each widened past the third of their promise they
used to keep; the parallel scan's threshold is measured milliseconds rather
than a file count; and `init`, the MCP server and the baseline stopped
reporting success over work they had not done. **Twenty-three entries are
deleted rather than hedged** — ten misses and four false positives across the
three widened rules, five across `init`, MCP and the baseline, two on the
parallel scan, one on Kotlin, and one whole section on a threshold that no
longer exists.

## Rungs 5 and 6: what the engine claims, and what it does not

FAST and MEANT were doctrine and nothing else until `hooks/checks/judgment.js`
existed. They now have **three rules between them**, out of 47 —
`fast/query-in-loop`, `fast/blocking-in-async` and `meant/unimplemented-stub` —
and the ratio is the honest headline. `BUILTIN_RULE_IDS` in
`hooks/checks/patterns/markers.js` holds 47 ids; `RUNGS` in
`hooks/checks/finding.js` lists six names, and `[rungs] fast` and `[rungs]
meant` are no longer inert. Verified, each by running it:

| Claim | Verified |
|---|---|
| `procoder check` reports a FAST finding | `for (const row of rows) { await db.query(…) }` reports `[5 FAST] a database or network round trip per item of rows` and exits 1. Same shape in Python and Go. |
| `procoder check` reports a MEANT finding | A Rust `todo!()` reports `[6 MEANT] todo!()/unimplemented!() on a path that ships`. |
| `[rungs] fast = "error"` promotes rung 5 to blocking | It does. With `[levels] pragmatic = ["**"]` — where a `warn` rung does not block — `fast = "error"` on the loop above still exits 1. |
| A line marker naming a `fast/*` id suppresses something | It does. The three ids are in the known set; a *misspelled* one still warns `unknown rule id … — it suppresses nothing`. |

**What the three rules cost in noise.** Measured on 123,639 third-party files
when the rules landed: `fast/query-in-loop` fires on 0.12% of files,
`fast/blocking-in-async` on 0.017%, and `meant/unimplemented-stub` on 0.28% of
Rust files. Re-run for this pass over the 53,927 JS/TS/Python files reachable
here — three unrelated projects' `node_modules` — `fast/query-in-loop` fired 36
times across 34 files (**0.063%**) and `fast/blocking-in-async` not once; no
Rust corpus was reachable, so the Rust figure is the landing measurement rather
than a re-run. These are `warn` rungs, which makes a false positive worse rather
than better — a noisy warning is a warning everyone learns to scroll past.

### What was built for these rungs and deliberately dropped

Each of the following was written, measured and deleted. Every one of them is a
defect procoder does not report, so the list belongs here rather than in the
rationale:

| Rejected rule | Why |
|---|---|
| A nested scan over the same collection | It fired **58 times** over CPython's 1,852-file standard library, and every one of the 58 was correct code at the size it actually runs. A quadratic is a claim about `n`, and one file does not state `n`. |
| `await` in a loop | A deliberately sequential loop is correct and common — rate limits, ordering, back-pressure. Indistinguishable from the defect in one file. |
| A log line in a hot loop | Nothing in one file says which loop is hot. |
| A fetch with no page size | The bound is usually elsewhere: the caller, the API, a `while (hasMore)` that is itself the correct answer. |
| String `+=` in a loop | Needs the type. In Python and Java it is a quadratic; in Go with a builder, or in Rust with a `String`, it is not. |
| A dead parameter | An interface fixes the signature; the parameter is not the author's to remove. |
| Rust's `unimplemented!()` | Rejected on evidence — see [Accepted trade-offs](#accepted-trade-offs). |

**Almost all of MEANT stays doctrine-only, and always will.** Scope creep is a
relationship between a change and a request. A line scanner has the change and
has never seen the request, so the only shape of rung 6 a single file carries
the whole evidence for is a stub that compiles, reads as done, and panics in
front of whoever called it. That is `meant/unimplemented-stub`, and it is
one rule because there is one such shape, not because nobody has written the
others yet. The structural claim the earlier edition of this page made still
holds in weakened form: the doctrine is what the model reads, the engine is
what CI runs, and for rungs 5 and 6 the overlap is three rules wide.

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
| A parameter arriving already tainted | `function f(q) { db.query(q); }` reports nothing, verified in all six packs. This one is a deliberate trade-off rather than a gap — see [Accepted trade-offs](#accepted-trade-offs). |
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

Two mechanisms decide when the scan stops following a value, and the shape of
each is what the remaining limitations are made of. What they bought, and the
five false negatives they cost, are under
[Accepted trade-offs](#accepted-trade-offs).

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

Both are allow-lists, and an allow-list is a decision to trust a name.

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
| Any language outside the six packs | `.rb`, `.php`, `.swift`, `.scala` and everything else resolve to no pack, so no shape rule and no SAFE rule of any pack runs over them. Verified: a Ruby, PHP, Swift or Scala six-parameter function reports nothing, and a Ruby file concatenating input into `db.query` reports nothing either. Only the universal pack (rung 1, credentials and suppressions) reads them. **Kotlin is no longer one of them** — `.kt` and `.kts` are in the jvm pack's `EXTENSIONS`, verified: `fun matcher(a0…a5)` reports `6 parameters (limit 4)` and a concatenated query into `stmt.executeQuery` reports `safe/sql-injection`. |

Swapping in a real per-language parser is the fix; it has not happened in
0.2.0.

## The three widened rules, and what each of them still misses

`safe/redaction-marker`, `safe/manifest-not-locked` and `true/missing-timeout`
were the three the doctrine had and the engine barely did: one pattern, one
ecosystem, two packs between them. All three were widened, and **fourteen rows
this section used to carry are gone** — ten misses and four false positives.
What is below is what a fresh round of probing found, smallest file that should
fire against smallest file that should not.

### `safe/redaction-marker`

`REDACTION_MARKER` in `hooks/checks/universal.js` is now
case-**in**sensitive, accepts the angle-bracket and suffixed spellings, and
accepts an unterminated one — and it fires **only where a value belongs**:
after an `=` or a `=>`, or under a config key in JSON, YAML, dotenv, a TOML
pair or an object literal. Seven files were verified reporting, one spelling
each, each a single assignment or config pair:

| Spelling | Where it was verified |
|---|---|
| lower-case, bracketed | a JavaScript string assignment |
| angle-bracketed | the same |
| unterminated — no closing bracket, the shape a truncated write leaves | the same |
| suffixed with a word, as a scanner emits | the same |
| bracketed, upper-case | a JSON credential key, a YAML credential key, and a dotenv line |

The wide false positive that used to make every page explaining the mechanism
carry a hand-written marker is **gone**, verified: the Markdown sentence *"The
scanner replaces the value with [REDACTED] before the model sees it."* now
reports nothing, and neither does a sentence opening `Note:` nor a JavaScript
comment describing the mechanism. The gate is the position, not a word list —
prose puts the word after a verb or a preposition; a destroyed value sits where
the value went.

| Still missed | The file |
|---|---|
| A marker in a table cell | A Markdown row `\| secret \| [REDACTED] \|` is silent, verified. A pipe is not a position a value occupies, and teaching the rule one would re-open the prose false positive across every documentation table in existence. |
| A spelling with no bracket at all | `const g = "***REMOVED***"` is silent, verified. The pattern is anchored on `[REDACTED` or `<REDACTED`; a redaction layer emitting anything else is not recognised. |
| Anything the position gate cannot see | A marker returned from a function, built by concatenation, or written on a continuation line is not "where a value belongs" as this rule reads it, and is silent. |

### `safe/manifest-not-locked`

Three ecosystems now, not one: `PER_ENTRY` in `hooks/checks/deps.js` maps
`package.json` to `checkNpmLocked`, `go.mod` to `checkGoLocked` and
`Cargo.toml` to `checkCargoLocked`. Verified reporting, one throwaway tree
each: `left-pad` in `dependencies` against a lockfile that does not name it;
`github.com/pkg/errors` required in `go.mod` with an empty `go.sum`; `serde`
under `[dependencies]` with a `Cargo.lock` that never names it. Each verified
silent once the lockfile names it.

Four rows this section used to carry are struck as fixed, each re-verified:

- **`optionalDependencies` is checked.** `{"optionalDependencies":
  {"fsevents": "2.3.3"}}` against an empty lockfile now reports.
- **A direct dependency the lock knows only as somebody else's transitive is
  caught.** `ms` in `dependencies`, with a lockfile mentioning it only inside
  `debug`'s own dependency map, reports — the lock is resolved by entry now,
  not text-matched as one string.
- **A workspace package finds the repository's lockfile.** `packages/a/package.json`
  under a root manifest declaring `"workspaces"` resolves against the root
  `package-lock.json`: unlocked reports the entry, locked is silent, and the
  false `safe/missing-lockfile` is gone.
- **An unreadable manifest says so.** A `package.json` with a trailing comma
  reports `package.json could not be parsed (…) — nothing in it was checked
  against the lockfile` rather than returning in silence.

The `peerDependencies` false positive is **gone**: `LOCK_BLOCKS` is
`dependencies`, `devDependencies` and `optionalDependencies`, and a peer is by
definition the consumer's to install. Verified: `{"peerDependencies": {"react":
"^18.0.0"}}` against an empty lockfile reports nothing.

| Still missed | The file |
|---|---|
| python, dotnet, maven and gradle manifests | `pyproject.toml`, `requirements.txt` and `Directory.Packages.props` get `safe/missing-lockfile` and nothing else — verified, each reports exactly that one finding. `pom.xml` and a Gradle build file report nothing at all. Deliberate, and each for its own reason: `requirements.txt` *is* the pinned artefact in most repositories; a pyproject name maps to a poetry or uv lock entry only through PEP 503 normalization, extras and environment markers; `packages.lock.json` is opt-in and rarely committed; and maven and gradle have no lockfile in the default toolchain at all, so there is nothing to compare a declaration against. |
| A dependency declared somewhere the section scan does not read | The Cargo reader understands `[dependencies]`, `[dev-dependencies]`, `[build-dependencies]`, their workspace and target-cfg spellings, and a `[dependencies.x]` table with its own `package =` rename. A declaration reached any other way is not seen. |

The **unreadable-manifest finding still carries `safe/manifest-not-locked`'s
own id**, which overloads one id with two different claims — "this dependency
is not locked" and "I could not read this file". A dedicated rule id for it had
not landed on `main` when this page was written.

### `true/missing-timeout`

**Five of the six packs**, not two. Verified reporting, one file each:

| Pack | Shapes verified reporting |
|---|---|
| py | `requests.get(url)`; `urllib.request.urlopen(url)`; a bound `s = requests.Session()` then `s.get(url)`, and the `self.s` form; `from requests import get` then `get(url)`, and the aliased `import get as fetch` then `fetch(url)` |
| go | `c := &http.Client{}`; `http.Get(u)`; **`http.DefaultClient.Do(req)`**, which this page used to list as missed |
| ts | `await fetch(url)`; `axios.get(url)` |
| rust | `reqwest::get(u)`; `reqwest::Client::new()` |
| jvm | `HttpClient.newHttpClient()` |

The `**kwargs` false positive is **gone**: `requests.get(url, **DEFAULTS)` is
silent, verified, because the rule cannot follow a name and a miss costs less
than a blocking finding on correct code. `httpx` and `aiohttp` are deliberately
out — both ship a default timeout, so reporting a bare call there was a finding
against correct code. A `file:` or `data:` URL buys silence for a plainer
reason: there is no peer to wait for. Verified silent: `httpx.get(url)`,
`urlopen("file:%s" % path)`, `axios(config)` (whose single argument is where
axios's own `timeout` lives), and `fetch(url, { signal: AbortSignal.timeout(1000) })`.

| Still missed | The file |
|---|---|
| A multi-line call | `requests.get(\n    url,\n)` is silent, verified, and deliberately: the call must open and close on one line, because a timeout on a later line would otherwise be reported as missing, and this rung blocks. |
| A builder chain | `reqwest::Client::builder()` … `.build()` and `HttpClient.newBuilder()` … `.build()` are both silent, verified, for the same reason — the `.timeout(…)` sits on a line of its own. Go's multi-field literal `&http.Client{Transport: tr}` is silent on the same argument. |
| An unbound receiver | `session.get(url)` in a file that never tied `session` to `requests` is silent, verified. Which receivers count is read off the file's own bindings and imports; a name this file did not bind means nothing. |
| The dotnet pack | There is no timeout rule in it. `new HttpClient()` in C# reports nothing, verified. |

Where a timeout *is* discharged by a deadline rather than a field, the rule
sees it: `http.DefaultClient.Do(req)` ten lines below a
`context.WithTimeout` and a `http.NewRequestWithContext` is silent, verified.

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

## The self-scan: what it does not prove

Measured today over **212** tracked files, `procoder check .` reports **0**
findings and exits 0, having skipped **18** of them and said so on stderr for
every one. The accounting of those 18 — what holds each one back, and why each
argument still stands — is a set of decisions rather than a set of weaknesses,
and has moved to
[Why it works this way](https://azrtydxb.github.io/procoder/design.html). What
stays here is what a green self-scan does *not* establish.

- **A clean self-scan means procoder finds nothing wrong with procoder**, which
  is exactly the claim a heuristic tool is worst placed to make about itself.
  It is evidence that the tool is not exempting itself. It is not evidence that
  the code is correct.
- **`docs/superpowers/` is ignored because procoder still cannot tell a
  document quoting a violation from a file committing one**, at the scale of a
  1,500-line planning document. Verified: 56 findings under `--no-ignore`, every
  one of them meta-text. The line marker solves the case one line at a time; it
  was never applied to historical documents, and at that density it would be a
  document made of markers. This is the same gap `safe/redaction-marker`'s
  position gate narrowed for one rule and one rule only.
- **No staleness report of any kind reaches `--format json` or `sarif`.** All
  three instruments — unused rule exclusions, unused path exclusions, stale
  ignore patterns — are `verify` output, and `--format` is refused off `check`,
  so the two can never meet. That is deliberate — a suppressed line of config is
  not a code finding, and SARIF has nowhere honest to put one — but it costs a
  dashboard-only pipeline the ability to learn that a whole directory has been
  quietly un-gated without running a second, text-mode `verify`.
- **The parallel path is not exercised by the self-scan.** Measured through
  `scanFiles` today, the 212 files of this repository cost **737 ms**
  sequentially, against a fork threshold of 900 ms, so `procoder check .` over
  procoder itself is always sequential. Tests reach the pool through a
  `forceParallel` option that exists for that reason and no other.

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

One edge remains, verified and not fixed:

- **A fractional value is floored, silently.** `--jobs 2.5` prints nothing and
  scans with 2. `Math.floor(2.5)` is a usable count, so it never reaches the
  refusal; only a value that floors below 1 does.

The second edge this page carried — the warning naming `NaN` or `null` rather
than what you typed — is **fixed**, and the row is deleted rather than hedged.
Verified: `--jobs abc` now warns `--jobs "abc" is not a number of worker
processes`, and `--jobs Infinity` warns about `"Infinity"`.

**The default job count is sized by `os.availableParallelism()`, not
`os.cpus()`.** `os.cpus()` reports the *host's* core count straight through a
cgroup CPU quota, so a container run at `--cpus 1` reported 10 and forked eight
workers: measured on 4,000 files, `--cpus 1` cost 14.9 s sequential and 35.0 s
at four jobs — the same shape of loss `--jobs 9999` used to buy on a laptop,
with nobody typing anything to get it. `os.cpus()` remains the fallback, and the
floor of 1 remains, because it is documented to be able to return nothing.

### The threshold is measured work now, and what that still leaves

`PARALLEL_MIN_FILES` is **gone**, and the section this page carried about it —
including the line *"Sizing it by bytes, or by a sampled cost per file, is the
fix; it has not happened in 0.2.0"* — is struck. `PARALLEL_MIN_WORK_MS` is
**900**, and it is milliseconds of *sequential* work, not files. Verified:
`require('hooks/checks/scan.js')` exports `PARALLEL_MIN_WORK_MS` at 900 and no
`PARALLEL_MIN_FILES` at all.

The number is measured rather than modelled. `probeCost` walks the file list on
a stride, scans a handful of files **for real** — 4 at minimum, 16 at most,
100 ms at most — drops the single most expensive sample so one outlier cannot
decide the run, and extrapolates a per-file cost over what is left. The sample
is not a rehearsal: those files are scanned once, with the options a worker
would get, and their results are the ones reported, so the measurement costs the
parallelism of at most 16 files and nothing else.

**Total bytes was measured and rejected.** Across the shapes the pool was
measured on, bytes as a predictor spread the crossover by **30×** — worse than
the file count's 23× — while sequential milliseconds held it inside a 3.8×
band, 230 ms to 870 ms. The row that kills every list-derived predictor is a
Rust crate: five files, two hundred *bytes*, 1.7× faster forked, because each
file costs a `cargo` subprocess that neither a byte count nor a file count can
see. 900 is the worst observed crossover rounded up, because being wrong above
the threshold costs a fraction of a scan and being wrong below it costs a
multiple of one.

What that still leaves, each verified:

| Limit | Measured |
|---|---|
| **Slices are balanced by file count, not by bytes** | `sliceInto` cuts the list into equal-length contiguous runs, so one slice holding the tree's three large files finishes long after the others and the pool waits for it. This is the known follow-up, and it is why the measured speed-up at eight workers is 2.2–2.5× rather than 8×. |
| **A shape whose own crossover is below 900 ms scans sequentially anyway** | Stated rather than hidden: a clippy crate of 16 files measures 897 ms sequential against 318 ms forked and just misses the threshold. The alternative is a threshold that forks a tree of one-liners six times slower than not forking, which is the defect that was fixed. |
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

The one this page led with is **fixed**: `init --baseline` no longer exits 0
over files it could not read. Verified — with an oversized file and an
unreadable one in the tree, it names both on stderr, records what it could, and
exits **2** with `a baseline cannot accept what nothing looked at`, which is
exactly what `procoder verify .` on the same tree then says and exits. Setup and
the gate it sets up now agree.

**The MCP server** (`procoder-mcp/`) exposes four tools — `procoder_doctrine`,
`procoder_check`, `procoder_review`, `procoder_baseline` — and is a strict
subset of the CLI. It has no `verify`, no baseline *writing*, no `rot`, no
`init`, no `--format`, no `--jobs`, no `--aging` and no `--unused-exclusions`;
`--since` exists only inside `procoder_review`. That is what remains. **All
three of the edges this page carried are fixed**, and are struck rather than
hedged — each re-verified by driving the server over stdio:

- **`procoder_check` no longer truncates.** It passes `maxFindings: Infinity`,
  as the CLI does. Verified: a file holding twelve credential literals answers
  with twelve, the same number `procoder check` reports.
- **`procoder_review` never says `clean` over a diff it did not read.** Verified
  with two changed files, both over `max_file_bytes`: the answer is `no findings
  in the 0 changed files that were checked` followed by `2 of 2 changed files
  NOT checked — nothing in them was looked at, so this is not a clean review`,
  naming each file and its reason. The word `clean` is spent only on a run where
  every changed file was read.
- **A failure is an `isError` result.** Verified: a bad ref comes back with
  `isError: true` and `cannot review: git diff … failed: … No file was checked;
  this is not a clean result.`, and a path that does not exist comes back from
  `procoder_check` with `isError: true` too.
- **MCP review follows a rename.** `changedFiles` uses
  `--diff-filter=ACMRT`, the same filter the CLI's `--since` uses. Verified:
  after `git mv many.js renamed.js`, the review reports at `renamed.js`.

What remains true of the server: every answer is a prose blob, so a client
cannot gate on one without parsing English. `isError` is the only structured
signal it emits.

**Baseline entries record what they accepted and when** — `{fp, id, path,
added}`, with `added` a `YYYY-MM-DD` stamp. Both entries this page carried about
them are **fixed and struck**: the baseline **prunes** (verified: three accepted
entries, fix every finding, re-run, and it reports `0 accepted findings, 3
pruned — fixed or gone`), and a dated-`unknown` entry **is** stamped on the next
write, exactly as `verify --aging` promises (verified: a hand-edited `unknown`
entry came back as a real rule id, path and today's date). What stays is one
property of the format, and it is a design decision rather than a defect — see
[Why it works this way](https://azrtydxb.github.io/procoder/design.html):
`added` is data the user owns, hand-editable and unsigned.

`--aging` is `verify`-only and refuses negatives and non-numbers with exit 2;
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

## Two silent value losses

Everything else about the config parser and the file-size ceilings is a
decision, and lives on
[Why it works this way](https://azrtydxb.github.io/procoder/design.html). These
two are not, because in both cases the tool proceeds as though it had read
something it did not, and says nothing.

| Input | What happens | Warned |
|---|---|---|
| A scalar and a table of the same name in `.procoder.toml` — `a = 1`<br>`[a]`<br>`b = 2` (or `a.b = 2`) | `{a: {b: 2}}` loads; the scalar is dropped. Verified in `parseToml` directly, both spellings. Every other construct the parser cannot read exactly is warned on stderr with file and line; this one is not. | **no** |
| A file over `[limits] max_file_bytes` under `check` | `procoder check` prints `skipped <file> (too-large) — not checked.` on stderr and exits **0**. Verified at 1,048,577 bytes. Only `verify` treats an unread file as a hole in its own claim and exits 2, so a `check`-only pipeline reads a file nobody opened as a file with nothing in it. | on stderr, but the exit code does not say so |

## The PostToolUse hook reports near the edit

`hooks/procoder-check.js` narrows the language packs' findings to the region
the tool call touched, ±3 lines (`CONTEXT_MARGIN`). The universal pack is
exempt — a credential is a leak wherever it sits — but a file-level finding
reported at line 1 (`obvious/nesting-depth`) will not surface from an edit made
further down. Verified against a 56-line file: an edit at line 56 reports only
the secret, an edit at line 1 reports the secret and the nesting depth.

The narrowing is off entirely when the hook cannot locate the tool call's text
in the saved file — a whole-file write, or an edit whose string no longer
matches — in which case every finding surfaces. What survives the narrowing is
then truncated to five, which is a deliberate trade-off rather than a gap and is
written up below.

## Accepted trade-offs

Each of these was chosen, not overlooked, and each one still costs the reader
something. That is why they are on this page rather than on
[Why it works this way](https://azrtydxb.github.io/procoder/design.html): a
decision with a price is a thing a user needs to know the price of. Every row
says what it buys as well as what it costs, and every row was re-verified by
running the tool.

### The escaper allow-list: five false negatives

Taint dies at a call whose callee is *named* as sanitising, or is a known
per-ecosystem driver escape, and the sanitiser's whole argument list is skipped
with it. **What it buys**: four rung-1 false positives on correct code, all
four verified silent today — `q = sanitizeSql(q)` before the sink,
`mysql.escape(id)` concatenated into a query, `escapeHtml(x)` assigned to
`innerHTML`, and `redis.execute(cmd)` in a module that also runs SQL. Before
this, correct escaping blocked the build. **What it costs**:

| Trusted, and wrong | The file |
|---|---|
| **The wrong escaper for the sink** | `db.query(escapeHtml("SELECT * FROM t WHERE id=" + id));` is silent. The name says escaping; nothing asks escaping *for what*, and HTML escaping does not make a query safe. Every cross-context pairing goes the same way — `escapeSql` into `innerHTML`, `htmlEscape` into `exec`. |
| **Escaping the assembled query rather than the value** | `q = "SELECT * FROM t WHERE id=" + id;` then `q = sanitizeSql(q);` then `db.query(q);` is silent, and it is not a fix: the injection is already in the string by the time the escaper sees it. Verified in JS, and in Python (`sanitize_sql(q)`) and Go (`escapeSql(q)`) alike. |
| **Anything merely *named* `sanitizeX` / `escapeX` / `quoteX`** | `q = sanitizeNothingAtAll(q); db.query(q);` is silent. The allow-list is a naming convention, not a contract, and a function that only looks like an escaper is trusted exactly as far as one that is. |
| **A sink nested inside a sanitiser's arguments** | `const out = escapeHtml(db.query("SELECT * FROM t WHERE id=" + id));` is silent. The sanitiser's whole argument list is skipped — which is what stops `q = sanitizeSql(q)` reporting — and the inner sink goes with it. The taint path still reaches it when the value was bound on an earlier line: with `const q = "SELECT id=" + id;` above, the same statement reports at the sink, built at line 1. |
| **A real database handle named `cache`, `api` or `queue`** | `const cache = new Pool();` in a file importing `pg`, then `const q = "SELECT * FROM t WHERE id=" + id; cache.query(q);` — silent. So is the same file with the handle named `api` or `queue`. The receiver list is a guess about intent from a name, and a name is the one thing a project is free to choose badly. |

### The taint scan does not follow a tainted parameter

`function f(q) { db.query(q); }` reports nothing, verified in all six packs.
**What it buys**: silence on every data-access helper ever written. Nothing
inside the file separates the untrusted caller from the one passing a constant,
so reporting it would fire on the correct code far more often than on the
defect — it is the single largest false positive available in this rule.
**What it costs**: an injection that arrives through a parameter is invisible,
which is the commonest shape a real one takes once a codebase has helpers.

### The hook prints at most five findings

`MAX_FINDINGS` in `hooks/checks/run.js` is **5**, and the PostToolUse hook takes
that default. **What it buys**: an in-session notice that fits in a turn. The
hook fires on every file write; a wall of findings after each edit is a wall
everybody learns to scroll past, and the value of the hook is that a finding
lands while the change is still cheap to undo. **What it costs**: the count in
the message is the truncated one, not the real one, and the hook does not say
so — verified on a file of 8 `eval(` calls written whole,
`procoder [strict] — 5 findings in many.js`, while `procoder check many.js`
reports 8. The CLI is unbounded, and so is the MCP server's `procoder_check`
since it stopped sharing this default. Run `procoder check <file>` or
`/procoder:review` for the whole-file picture.

### `unimplemented!()` is excluded from `meant/unimplemented-stub`

Only `todo!()` is reported. **What it buys**: the rule's whole signal. Measured
over 29,365 crates.io files: **1,122 `unimplemented!` against 129 `todo!`**, and
the sample is what the standard library's own definitions promise — `todo!` is
"not yet implemented, intended to be", `unimplemented!` is "not supported / not
required". `unimplemented!` in the wild is a deliberate `_ =>` arm, a platform
stub, a dummy impl in a test double: correct code. Reporting it would have made
the rule 90% noise, and this is a `warn` rung. **What it costs**: a genuine
"I have not written this yet" spelled `unimplemented!()` ships in silence.
Verified: a file containing only `unimplemented!()` reports nothing, where
`todo!()` reports.

### Node's `*Sync` calls are excluded from `fast/blocking-in-async`

`execSync`, `spawnSync` and `readFileSync` inside an `async function` are not
reported. **What it buys**: silence on CLIs, build scripts, tests and
migrations. Measured on this repository first, where the rule reported
`spawnSync('git', …)` inside an async test — correct code, twice. `async` in
JavaScript marks any function that awaits something, not a request path, which
is not true of Python's `async def` or Rust's `async fn`, where an executor is
driving and the blocking calls in those two *are* reported. **What it costs**:
a Node server handler that blocks the event loop on `readFileSync` is not
caught by this rule. Verified silent. Node keeps this rung through
`fast/query-in-loop` instead.

### Three performance ceilings that cut without announcing it

The rest of the performance narrowings announce themselves — a
`true/budget-exhausted` finding, a `true/findings-suppressed` finding, a stderr
line naming the stage — and are on
[Why it works this way](https://azrtydxb.github.io/procoder/design.html). These
three do not.

| Narrowing | Where | What it buys, and what it costs |
|---|---|---|
| 500-character span, nested ternary | `hooks/checks/lang/ts.js` | Buys a linear scan on a minified line. Costs: `obvious/nested-ternary` is reported with 400 characters between the two `?` and **not** with 499. Verified both ways. |
| 500-character span, signature head and tail | `go.js` (receiver, and the text between `)` and `{`), `rust.js` (generic list and the same tail), `ts.js` (return type) | Buys the same. Costs: a 6-parameter function is measured with a 200-character tail or generic list and **not** with a 700-character one — verified in Go and TypeScript today. Java and C# have no such span; their tails are anchored to end of line, and C#'s expression-bodied `void M(…) => a + b;` reports `8 parameters (limit 4)`, verified. |
| 4 KB per-line guard, span-derived shape rules only | `hooks/checks/run.js` | Buys a bounded cost on a bundled or minified file. Costs: function length, nesting depth and complexity are not measured on a line over 4 KB — verified, a 4,847-character single-line function with four levels of nesting reports nothing. `obvious/too-many-params` is **not** guarded and is exact on a minified line (a 1,000-parameter one-line signature reports, verified), and the SAFE rules read the line unguarded, so a credential 1,500 characters into a log call is still reported. |
