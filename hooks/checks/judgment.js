#!/usr/bin/env node
// procoder — rungs 5 (FAST) and 6 (MEANT): the judgment rungs, and the small
// part of them a scanner may honestly claim.
//
// Both rungs were doctrine-only until this file existed. Neither can be decided
// from one file in general — FAST needs the size the input really reaches,
// MEANT needs the request the diff answers to — and that is not a reason to
// ship nothing, nor a licence to ship guesses. What is implemented here is the
// subset where the SHAPE alone is the defect, whatever the sizes turn out to
// be, and everything else stays doctrine. docs/known-limitations.md carries the
// list of what was considered and left out; the short version:
//
//   shipped   a database/network call inside a loop over a named collection;
//             a blocking call on an async path; a Rust `todo!()` on a path that
//             ships.
//   rejected  a nested loop over the same collection — 58 hits over CPython's
//             1,852-file stdlib and every one of them correct code at the size
//             it runs; a bare `await` in a loop (a deliberately sequential loop
//             is correct and common); a log line in a "hot" loop (nothing in
//             one file says which loop is hot); a fetch with no page size (the
//             bound is usually elsewhere); string `+=` in a loop (needs the
//             type); a dead parameter (an interface fixes the signature).
//
// Every rule here obeys the same bar: it must be silent on the correct twin of
// the shape it reports. These are `warn` rungs, which makes a false positive
// WORSE and not better — a noisy warn is a warn everyone learns to scroll past.
//
// Cost: two linear passes over the file (one to strip, one to scan) plus O(1)
// work per loop. No rule scans a loop body per loop — see nextMatch below —
// because a file of deeply nested loops would then be quadratic in its own
// right, which is a poor look for the rung that reports quadratics.

const path = require('path');
const { finding } = require('./finding');
const { stripNoise } = require('./shape');

// Comment style per extension, in comments.js's vocabulary. Kept explicit
// rather than derived from the packs, because a pack owns its extensions and
// this file owns whether it can read them: an extension nobody has taught this
// table is skipped in silence, and tests/judgment.test.js fails the day a pack
// adds one, so the silence cannot last a release.
const STYLE = new Map([
  ['.ts', 'js'], ['.tsx', 'js'], ['.js', 'js'], ['.jsx', 'js'],
  ['.mjs', 'js'], ['.cjs', 'js'],
  ['.py', 'py'],
  ['.go', 'c'], ['.rs', 'c'], ['.java', 'c'], ['.kt', 'c'], ['.kts', 'c'],
  ['.cs', 'c'],
]);

// --- loops -----------------------------------------------------------------

// The collection a loop header iterates, per language, capture last. Only the
// for-each shapes are here, plus the C-style header whose bound is a name.
//
// `while` is deliberately absent, and that is the single largest false-positive
// saving in this file: `while (hasMore)` around a fetch is PAGINATION, the
// correct answer to the very rung this file implements, and `while True:`
// around a request is a retry loop. Both hold exactly the shape rule 1 reports.
const ITERABLES = [
  // JS/TS: for (const row of rows), for await (const row of rows), for (k in o)
  /\bfor\s*(?:await\s+)?\(\s*(?:const|let|var)\s+[\w$[\]{},\s]{1,80}\s+(?:of|in)\s+([A-Za-z_$][\w$]*(?:\.[\w$]+)*)/,
  // C#: foreach (var row in rows). Kotlin: for (row in rows).
  /\bfor(?:each)?\s*\(\s*(?:[\w<>,.[\]?]{1,60}\s+)?[A-Za-z_]\w*\s+in\s+([A-Za-z_][\w]*(?:\.[\w]+)*)/,
  // Java: for (Row row : rows)
  /\bfor\s*\(\s*(?:final\s+)?[\w<>,.[\]?]{1,60}\s+[A-Za-z_]\w*\s*:\s*([A-Za-z_][\w]*(?:\.[\w]+)*)/,
  // Go: for i, row := range rows
  /\bfor\s+[\w,\s]{0,60}:?=?\s*range\s+([A-Za-z_][\w]*(?:\.[\w]+)*)/,
  // Rust, Python, Kotlin: for row in rows — and Python's comprehension form.
  /\bfor\s+[\w,\s()&*]{0,60}?\s*\bin\s+&?\s*([A-Za-z_][\w]*(?:\.[\w]+)*)/,
  // C-style, bound by a name: for (let i = 0; i < rows.length; i++). A bound
  // that is a numeric literal matches nothing here, which is the point — a
  // retry loop of three attempts is not a loop over request-sized input.
  /\bfor\s*\([^;\n]{0,200};[^;\n]{0,120}?[<!]=?\s*([A-Za-z_$][\w$]*(?:\.[\w$]+)*)/,
];

// Cheap gate before any of the six patterns runs.
const FOR_WORD = /\bfor(?:each)?\b/;

// A name the file itself builds out of a literal — `dataset = [...]`,
// `cases = {...}`. Rung 5 is about the size production arrives at, and a
// collection whose contents are written out in the file has already stated its
// size: it is the fixture, not the request. Measured on CPython's 1,852-file
// stdlib, this is what separates the three N+1 candidates found there — all of
// them a query per item of a five-row test table — from the shape the rule
// exists for, where the collection arrives from a caller, a parser or a query.
//
// Only `[` and `{` open a literal here. `= (` is a parenthesised expression far
// more often than a tuple, and `= call(` is a call, whose size is exactly what
// the file does NOT state.
const LITERAL_BINDING =
  /^\s*(?:(?:const|let|var|final|static|readonly)\s+)?(?:[\w$<>,.[\]?]+\s+)?([A-Za-z_$][\w$]*)\s*(?::[^=\n]{0,60})?=\s*[[{]/;

function literalNames(lines) {
  const names = new Set();
  for (const line of lines) {
    const m = LITERAL_BINDING.exec(line);
    if (m) names.add(m[1]);
  }
  return names;
}

// The collection's size is stated in this file: a literal binding, or a
// SCREAMING_CASE constant.
function sizeIsStated(name, literals) {
  const parts = name.split('.');
  const last = parts[parts.length - 1];
  return /^[A-Z0-9_]+$/.test(last) || literals.has(parts[0]) || literals.has(last);
}

// The collection this header iterates, or null for a header whose subject is
// not a named collection — which is where two more false-positive families are
// dropped rather than reported:
//
//   a call     `for i in range(3)`, `for x in enumerate(rows)`, `for k in
//              d.items()`. A call may be a counter, a bound of any size, or a
//              generator; the name it is called on is not the collection.
//              `range(n)` and a retry loop are indistinguishable here, so
//              neither is claimed.
// The collection whose size this file states is dropped by the caller, through
// sizeIsStated above — `for (const env of ENVIRONMENTS)` and `for row in
// dataset` where `dataset = [...]` is the "loop over a small constant list"
// twin, and a query per item of one is three queries.
function iterableOf(line) {
  for (const re of ITERABLES) {
    const m = re.exec(line);
    if (!m) continue;
    return line[m.index + m[0].length] === '(' ? null : m[1];
  }
  return null;
}

// Where each `{` on a line closes. Last write wins per line, so a line opening
// more than one block answers with the OUTERMOST of them — the widest body,
// which is the one a loop header on that line opened.
function braceCloses(text) {
  const closes = new Map();
  const stack = [];
  let line = 1;
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (ch === '\n') line += 1;
    else if (ch === '{') stack.push(line);
    else if (ch === '}' && stack.length) closes.set(stack.pop(), line);
  }
  return closes;
}

// The last line of the block a header on `n` opens. A header with no brace of
// its own takes the next line's only when that line is a lone `{` — the Allman
// style Java and C# are written in. Anything else is a single-statement body,
// and reading the NEXT block as the loop's would attribute the following
// `if` block's contents to a loop that never contained them.
function blockEnd(n, lines, closes) {
  if (closes.has(n)) return closes.get(n);
  if ((lines[n] || '').trim() === '{' && closes.has(n + 1)) return closes.get(n + 1);
  return n;
}

function braceLoops(lines, closes, literals) {
  const loops = [];
  for (let i = 0; i < lines.length; i += 1) {
    if (!FOR_WORD.test(lines[i])) continue;
    const iterable = iterableOf(lines[i]);
    if (!iterable || sizeIsStated(iterable, literals)) continue;
    loops.push({ line: i + 1, endLine: blockEnd(i + 1, lines, closes), iterable });
  }
  return loops;
}

// Indentation column per line, tabs expanded, blank lines left out (null).
function columns(lines) {
  return lines.map((l) => (l.trim() ? /^[ \t]*/.exec(l)[0].replace(/\t/g, '    ').length : null));
}

// The last line indented deeper than `at` — a Python suite.
function suiteEnd(cols, at) {
  const base = cols[at];
  let end = at;
  for (let i = at + 1; i < cols.length; i += 1) {
    if (cols[i] === null) continue;
    if (cols[i] <= base) break;
    end = i;
  }
  return end;
}

// Python's two loop forms. A `for` statement owns its suite; a comprehension
// owns exactly its own line, which is enough — the call it repeats is on it.
const PY_FOR_STATEMENT = /^\s*(?:async\s+)?for\b[^\n]*:\s*$/;

function indentLoops(lines, literals) {
  const cols = columns(lines);
  const loops = [];
  for (let i = 0; i < lines.length; i += 1) {
    if (!FOR_WORD.test(lines[i])) continue;
    const iterable = iterableOf(lines[i]);
    if (!iterable || sizeIsStated(iterable, literals)) continue;
    const end = PY_FOR_STATEMENT.test(lines[i]) ? suiteEnd(cols, i) : i;
    loops.push({ line: i + 1, endLine: end + 1, iterable });
  }
  return loops;
}

// --- what counts as a round trip -------------------------------------------

// A call that leaves the process: the database, the cache, the network. Every
// entry is anchored on something that names the store or the protocol, never on
// a verb alone — `.get(`, `.find(`, `.save(` and `.delete(` on their own are
// Map, Array, React and DOM APIs, and a rule built on the verb reports every
// one of them.

// Go, Java and C# capitalise their methods and Python and JS do not, so every
// verb below is accepted in both spellings — `db.Query(` and `db.query(` are
// the same round trip. Only the first letter varies, which is the difference
// between the two conventions and is not the same thing as an `i` flag: a
// case-insensitive rule would also accept `DB.QUERY(` out of a SQL string, and
// would make the receiver list — the whole reason this rule is quiet — porous.
const anyCase = (verb) => `[${verb[0].toUpperCase()}${verb[0]}]${verb.slice(1)}`;

// Verbs no in-memory container answers to. These are safe on any receiver that
// names a connection.
const QUERY_VERBS = [
  'query', 'queryRow', 'queryRowContext', 'queryContext', 'queryOne', 'execute',
  'executeQuery', 'executeUpdate', 'exec', 'execContext', 'fetch', 'fetchone',
  'fetchall', 'fetchrow', 'findOne', 'findFirst', 'findUnique', 'findMany',
  'insert', 'upsert', 'scalar', 'select',
].map(anyCase).join('|');

// Verbs a Map, a Set and a cache object answer to just as readily. Measured:
// `this.pool.delete(key)` inside a loop, in an SSH connection pool, is a Map
// deletion; `txn.Get(uri)` in Go's module cache is a file-cache read. Both were
// reported until these verbs were scoped to a receiver that can only be a
// store — which still keeps SQLAlchemy's `session.get(pk)` and drizzle's
// `db.delete(users)`, the N+1s this rule exists for.
const STORE_VERBS = ['get', 'find', 'delete', 'update', 'save', 'create', 'add'].map(anyCase).join('|');

const CONNECTIONS = 'db|DB|database|conn|connection|cursor|cur|session|sess|tx|trx|txn|pool|repo|repository|dao|em|entityManager|prisma|knex|sequelize|stmt|preparedStatement';
const STORES = 'db|DB|database|conn|connection|cursor|session|sess|prisma|repo|repository|dao|em|entityManager';

const IO_CALL = new RegExp([
  // A connection, a cursor, a session, a transaction, a repository.
  String.raw`\b(?:${CONNECTIONS})\s*\.\s*(?:${QUERY_VERBS})\s*\(`,
  String.raw`\b(?:${STORES})\s*\.\s*(?:${STORE_VERBS})\s*\(`,
  // Django, SQLAlchemy, Mongo: the call shape names the store.
  String.raw`\.objects\s*\.\s*(?:get|filter|create|update|delete|count)\s*\(`,
  String.raw`\.query\s*\.\s*(?:filter|filter_by|get|all|one|first)\s*\(`,
  String.raw`\.(?:findOne|findById|insertOne|insertMany|updateOne|updateMany|deleteOne|deleteMany|aggregate)\s*\(`,
  // HTTP. The lookbehind keeps `.fetch(` and `prefetch(` out; a bare `fetch(`
  // is the platform's.
  String.raw`(?<![.\w$])fetch\s*\(`,
  String.raw`\baxios\s*(?:\.\s*(?:get|post|put|patch|delete|head|request))?\s*\(`,
  String.raw`\b(?:requests|httpx|urllib\.request)\s*\.\s*(?:get|post|put|patch|delete|head|request|urlopen)\s*\(`,
  String.raw`\burlopen\s*\(`,
  String.raw`\bhttp\s*\.\s*(?:Get|Post|Head|PostForm|Do)\s*\(`,
  String.raw`\b(?:httpClient|_httpClient|restTemplate|webClient)\s*\.\s*\w+\s*\(`,
  String.raw`\bclient\s*\.\s*(?:Get|Post|Do|GetAsync|PostAsync|PutAsync|PatchAsync|DeleteAsync|SendAsync|GetStringAsync|GetFromJsonAsync|getForObject|postForObject|exchange)\s*\(`,
  // reqwest's builder: the request happens at `.send()`, and only there. An
  // empty argument list is what separates it from a channel's `tx.send(value)`.
  String.raw`\.send\s*\(\s*\)`,
  // Cache.
  String.raw`\b(?:redis|memcache|memcached)\s*\.\s*\w+\s*\(`,
].join('|'));

// --- blocking on an async path ---------------------------------------------

// A function that may not block: `async def`, `async fn`, `async function`,
// `async Task<T> Name(`, Kotlin's `suspend fun`.
const ASYNC_HEAD = /\basync\s*[(\w]|\bsuspend\s+fun\b/;
const PY_ASYNC_DEF = /^\s*async\s+def\b/;

// Calls that stop the thread rather than yielding it. Each has an async twin in
// its own ecosystem — asyncio.sleep, tokio::time::sleep, Task.Delay, an async
// HTTP client — so the finding always has somewhere to go.
const BLOCKING_CALL = new RegExp([
  String.raw`\btime\.sleep\s*\(`,
  String.raw`\b(?:std::)?thread::sleep\s*\(`,
  String.raw`\bThread\.[Ss]leep\s*\(`,
  String.raw`\breqwest::blocking\b`,
  String.raw`\b(?:requests|httpx)\s*\.\s*(?:get|post|put|patch|delete|head|request)\s*\(`,
  String.raw`\burlopen\s*\(`,
  String.raw`\bsubprocess\.(?:run|call|check_call|check_output|Popen)\s*\(`,
].join('|'));

// What is deliberately NOT in that list: Node's `execSync`/`spawnSync`/
// `readFileSync` inside an `async function`. Measured on this repository first,
// where it reported `spawnSync('git', …)` inside an async TEST — correct code,
// twice. `async` in JS marks any function that awaits something, not a request
// path: CLIs, build scripts, tests and migrations are written async and block
// on purpose. Python's `async def` and Rust's `async fn` mean an executor is
// driving, which is why the blocking calls in those two are still reported.
// Node keeps this rung through fast/query-in-loop instead.

// Handing blocking work to a thread is the FIX, and it is written on the same
// line as the blocking call it wraps. Reporting `await loop.run_in_executor(
// None, requests.get, url)` would report the correct code and nothing else.
const OFF_THREAD = /\brun_in_executor\b|\bto_thread\b|\bspawn_blocking\b|\bblock_in_place\b|\bTask\.Run\b/;

// --- MEANT -----------------------------------------------------------------

// Rust's "not written yet" macro. It compiles, it reads as done, and it panics
// on the path that reaches it — "scope reduced silently", which is the one
// shape of rung 6 a single file carries the whole evidence for.
//
// `todo!` and not `unimplemented!`, which is the neighbouring macro and is NOT
// the same claim: the standard library defines todo! as "not yet implemented,
// intended to be" and unimplemented! as "not supported / not required".
// Measured over 29,365 crates.io files: 1,122 `unimplemented!` against 129
// `todo!`, and the sample is what the definitions promise — unimplemented! is a
// deliberate `_ =>` arm, a platform stub, a dummy impl in a test double, all of
// them correct code. Reporting it would have made this rule 90% noise.
//
// Rust only, and that is a decision rather than an omission. Every other
// language spells "not written yet" exactly the way it spells an ABSTRACT
// method — Python's `raise NotImplementedError`, Java's `throw new
// UnsupportedOperationException`, a JS base class throwing "not implemented" —
// and those are correct code doing their job. Rust's abstract form is a trait
// method with no body at all, so nothing correct is spelled `todo!()`.
const RUST_STUB = /\btodo!\s*[([{]/;

// --- the scan ---------------------------------------------------------------

// For each line, the next line at or after it that matches — one backward pass.
// Loops then cost O(1) each instead of O(body), so a file of nested loops stays
// linear however deep the nesting goes.
function nextMatch(lines, re) {
  const next = new Array(lines.length + 1).fill(Infinity);
  for (let i = lines.length - 1; i >= 0; i -= 1) {
    next[i] = re.test(lines[i]) ? i : next[i + 1];
  }
  return next;
}

// Lines inside at least one async function, as a difference array summed once.
// Overlapping and nested regions cost nothing extra, where marking each region
// line by line would be quadratic on a file of nested closures.
//
// A region handed to a thread is subtracted from it, and the subtraction has to
// cover the whole BLOCK rather than the line: tokio's own tests are
//
//     task::spawn_blocking(|| {
//         thread::sleep(Duration::from_millis(5));
//     })
//
// which is the fix for this rule written across three lines, and reporting the
// middle one reports the correct answer as the defect. Measured on 29,365
// crates.io files, where it was the single largest source of hits. A plain
// `thread::spawn` counts too — the code inside it is not on the executor. The
// nested `def` does the same job in Python, where a sync function defined
// inside a coroutine is what gets handed to an executor.
const OFF_THREAD_BLOCK = /\brun_in_executor\b|\bto_thread\b|\bspawn_blocking\b|\bblock_in_place\b|\bTask\.Run\b|\bthread::spawn\b|\bThread\s*\(/;
const PY_DEF = /^\s*def\b/;

const isAsyncHead = (line, style) => (style === 'py' ? PY_ASYNC_DEF : ASYNC_HEAD).test(line);
const isOffThread = (line, style) => (style === 'py' ? PY_DEF : OFF_THREAD_BLOCK).test(line);

function asyncLines(lines, closes, style) {
  const cols = style === 'py' ? columns(lines) : null;
  const extent = (i) => (style === 'py' ? suiteEnd(cols, i) + 1 : blockEnd(i + 1, lines, closes));
  const delta = new Array(lines.length + 2).fill(0);
  const mark = (from, to, by) => { delta[from] += by; delta[to + 1] -= by; };

  for (let i = 0; i < lines.length; i += 1) {
    if (isAsyncHead(lines[i], style)) mark(i + 1, extent(i), 1);
    // A one-line `spawn_blocking(|| foo())` has no block to subtract, and the
    // same-line OFF_THREAD test in blockingInAsync already covers it.
    else if (isOffThread(lines[i], style) && extent(i) > i + 1) mark(i + 1, extent(i), -1);
  }

  const inside = new Array(lines.length + 1).fill(false);
  let depth = 0;
  for (let n = 1; n <= lines.length; n += 1) {
    depth += delta[n];
    inside[n] = depth > 0;
  }
  return inside;
}

// A function DEFINED inside a loop does not run inside it. Registering one
// handler per route, per tool, per subscription is what that shape almost
// always is, and the call in the handler's body happens once per REQUEST later,
// not once per item now. Measured: an MCP server registering a tool per entry
// of `dynamicTools`, whose handler calls `fetch` — reported as an N+1 that
// cannot happen.
//
// Counted as a prefix sum and applied to the whole loop rather than searched
// past: if any function opens between the loop's header and the call, this loop
// does not answer for that call. That is the conservative direction — a loop
// that also happens to define a callback before its own query goes unreported,
// which costs a finding and invents none.
const FUNC_HEAD = /=>\s*\{|\bfunction\s*\w*\s*\(|\bfunc\s*\(|\bfn\s+\w+\s*\(|^\s*(?:async\s+)?def\b/;

function functionHeadCounts(lines) {
  const counts = new Array(lines.length + 1).fill(0);
  for (let i = 0; i < lines.length; i += 1) {
    counts[i + 1] = counts[i] + (FUNC_HEAD.test(lines[i]) ? 1 : 0);
  }
  return counts;
}

function queryInLoop(loops, { ioAt, funcs }, out) {
  const seen = new Set();
  for (const loop of loops) {
    const at = ioAt[loop.line - 1];
    if (at === Infinity || at + 1 > loop.endLine) continue;
    const line = at + 1;
    if (funcs[line - 1] > funcs[loop.line]) continue;
    if (seen.has(line)) continue;
    seen.add(line);
    out.push(finding({
      rung: 'FAST',
      id: 'fast/query-in-loop',
      line,
      message: `a database or network round trip per item of ${loop.iterable}`,
      fix: 'fetch the set in one call before the loop, or batch by key',
    }));
  }
}

function blockingInAsync(lines, inside, out) {
  for (let i = 0; i < lines.length; i += 1) {
    if (!inside[i + 1]) continue;
    if (!BLOCKING_CALL.test(lines[i]) || OFF_THREAD.test(lines[i])) continue;
    out.push(finding({
      rung: 'FAST',
      id: 'fast/blocking-in-async',
      line: i + 1,
      message: 'a blocking call on an async path — it stops the thread instead of yielding it',
      fix: 'await the async form (asyncio.sleep, tokio::time::sleep, Task.Delay, an async client), or hand it to a thread',
    }));
  }
}

function unimplementedStub(lines, out) {
  for (let i = 0; i < lines.length; i += 1) {
    if (!RUST_STUB.test(lines[i])) continue;
    out.push(finding({
      rung: 'MEANT',
      id: 'meant/unimplemented-stub',
      line: i + 1,
      message: 'todo!()/unimplemented!() on a path that ships — it compiles and reads as done',
      fix: 'implement it, or say in the change itself which part of the ask is not delivered',
    }));
  }
}

// Never throws: every stage is a regex over strings, and an extension this
// table does not know returns nothing at all.
function checkJudgment(source, { relPath } = {}) {
  const style = STYLE.get(path.extname(String(relPath || '')).toLowerCase());
  if (!style) return [];

  const text = stripNoise(String(source || ''), style);
  const lines = text.split(/\r?\n/);
  const closes = style === 'py' ? new Map() : braceCloses(text);
  const literals = literalNames(lines);
  const loops = style === 'py'
    ? indentLoops(lines, literals)
    : braceLoops(lines, closes, literals);

  const out = [];
  queryInLoop(loops, { ioAt: nextMatch(lines, IO_CALL), funcs: functionHeadCounts(lines) }, out);
  blockingInAsync(lines, asyncLines(lines, closes, style), out);
  if (style === 'c' && path.extname(String(relPath)).toLowerCase() === '.rs') {
    unimplementedStub(lines, out);
  }
  return out;
}

module.exports = { checkJudgment, STYLE };
