// tests/judgment.test.js
//
// Rungs 5 and 6. Every rule is asserted in both directions, because the whole
// risk of a judgment rung is the correct twin: these findings are `warn` by
// default, and a warn that fires on correct code teaches everyone to scroll
// past the ones that do not.
//
// The twins here are the ones that actually broke a candidate during
// development, each measured against a real corpus rather than imagined:
// a loop over a constant list, a retry loop bounded by a literal, a deliberate
// `spawn_blocking`, a handler registered per item of a list, a `range(n)`
// counter, an in-memory Map that answers to `.get`/`.delete`.
const test = require('node:test');
const assert = require('node:assert');
const { checkJudgment, STYLE } = require('../hooks/checks/judgment');
const { PACKS } = require('../hooks/checks/registry');
const { BUILTIN_RULE_IDS } = require('../hooks/checks/patterns/markers');
const { RUNGS } = require('../hooks/checks/finding');
const { assertLinear, lineOf, bestOf } = require('./perf-guard');

const ids = (src, relPath) => checkJudgment(src, { relPath }).map((f) => f.id);
const has = (src, relPath, id) => ids(src, relPath).includes(id);

// --- fast/query-in-loop ----------------------------------------------------

test('reports a round trip per item, in every language that has the shape', () => {
  const cases = [
    ['a.ts', 'for (const row of rows) {\n  await db.query("select 1", [row.id]);\n}\n'],
    ['a.js', 'for (const row of rows) {\n  const r = await fetch(row.url);\n}\n'],
    ['a.py', 'for row in rows:\n    cur.execute("select 1", (row.id,))\n'],
    ['a.go', 'for _, row := range rows {\n\tdb.QueryRow(q, row.ID)\n}\n'],
    ['a.rs', 'for row in rows {\n    let r = client.get(&row.url).send().await?;\n}\n'],
    ['a.java', 'for (Row row : rows) {\n    stmt.executeQuery(q);\n}\n'],
    ['a.cs', 'foreach (var row in rows)\n{\n    await client.GetAsync(row.Url);\n}\n'],
  ];
  for (const [file, src] of cases) {
    assert.ok(has(src, file, 'fast/query-in-loop'), `not reported in ${file}`);
  }
});

test('says which collection the round trip is per item of', () => {
  const [f] = checkJudgment('for (const row of orderRows) {\n  await db.query(q);\n}\n',
    { relPath: 'a.ts' });
  assert.match(f.message, /orderRows/);
  assert.strictEqual(f.rung, 'FAST');
});

test('the same call outside the loop is silent', () => {
  const src = 'const all = await db.query("select 1");\nfor (const row of rows) {\n  use(row);\n}\n';
  assert.deepStrictEqual(ids(src, 'a.ts'), []);
});

test('a loop over a list this file writes out is silent', () => {
  const constant = 'for (const env of ENVIRONMENTS) {\n  await db.query(q);\n}\n';
  const literal = 'const dataset = [1, 2, 3];\nfor (const row of dataset) {\n  await db.query(q);\n}\n';
  const py = 'dataset = [\n    ("a", 1),\n]\nfor row in dataset:\n    cur.execute(q)\n';
  assert.deepStrictEqual(ids(constant, 'a.ts'), []);
  assert.deepStrictEqual(ids(literal, 'a.ts'), []);
  assert.deepStrictEqual(ids(py, 'a.py'), []);
});

test('a retry loop bounded by a literal is silent', () => {
  const js = 'for (let i = 0; i < 3; i++) {\n  const r = await fetch(url);\n}\n';
  const py = 'for attempt in range(3):\n    requests.get(url)\n';
  assert.deepStrictEqual(ids(js, 'a.ts'), []);
  assert.deepStrictEqual(ids(py, 'a.py'), []);
});

test('a while loop is silent — pagination is the fix, not the defect', () => {
  const src = 'let cursor = null;\nwhile (hasMore) {\n  const page = await db.query(q, [cursor]);\n}\n';
  assert.deepStrictEqual(ids(src, 'a.ts'), []);
});

test('a C-style loop bounded by a length is reported', () => {
  const src = 'for (let i = 0; i < rows.length; i++) {\n  await fetch(rows[i].url);\n}\n';
  assert.ok(has(src, 'a.ts', 'fast/query-in-loop'));
});

test('an in-memory container that answers to the same verbs is silent', () => {
  const map = 'for (const key of keys) {\n  this.pool.delete(key);\n}\n';
  const go = 'for _, mod := range mods {\n\tfh, err = txn.Get(mod.URI)\n}\n';
  assert.deepStrictEqual(ids(map, 'a.ts'), []);
  assert.deepStrictEqual(ids(go, 'a.go'), []);
});

test('a handler registered per item is silent — its body runs per request', () => {
  const src = 'for (const t of tools) {\n  server.tool(t.name, async (args) => {\n'
    + '    const r = await fetch(t.url);\n  });\n}\n';
  assert.deepStrictEqual(ids(src, 'a.ts'), []);
});

test('a python comprehension that queries per item is reported', () => {
  assert.ok(has('rows = [db.query(i) for i in ids]\n', 'a.py', 'fast/query-in-loop'));
});

test('one finding per loop, not one per statement in it', () => {
  const src = 'for (const row of rows) {\n  await db.query(a);\n  await db.query(b);\n  await db.query(c);\n}\n';
  assert.strictEqual(ids(src, 'a.ts').length, 1);
});

// --- fast/blocking-in-async ------------------------------------------------

test('reports a blocking call on an async path', () => {
  const cases = [
    ['a.py', 'async def handle(req):\n    time.sleep(1)\n'],
    ['a.py', 'async def handle(req):\n    r = requests.get(url)\n'],
    ['a.rs', 'async fn handle() {\n    thread::sleep(d);\n}\n'],
    ['a.cs', 'async Task Handle()\n{\n    Thread.Sleep(1000);\n}\n'],
    ['a.kt', 'suspend fun handle() {\n    Thread.sleep(1000)\n}\n'],
  ];
  for (const [file, src] of cases) {
    assert.ok(has(src, file, 'fast/blocking-in-async'), `not reported in ${file}: ${src}`);
  }
});

test('the async form of the same call is silent', () => {
  assert.deepStrictEqual(ids('async def handle(req):\n    await asyncio.sleep(1)\n', 'a.py'), []);
  assert.deepStrictEqual(ids('async fn handle() {\n    tokio::time::sleep(d).await;\n}\n', 'a.rs'), []);
});

test('the same blocking call outside an async function is silent', () => {
  assert.deepStrictEqual(ids('def handle(req):\n    time.sleep(1)\n', 'a.py'), []);
  assert.deepStrictEqual(ids('fn handle() {\n    thread::sleep(d);\n}\n', 'a.rs'), []);
});

test('work handed to a thread is silent, on its own line or across a block', () => {
  const inline = 'async def handle(req):\n    await loop.run_in_executor(None, lambda: requests.get(u))\n';
  const block = 'async fn handle() {\n    task::spawn_blocking(|| {\n        thread::sleep(d);\n    });\n}\n';
  const nested = 'async def handle(req):\n    def worker():\n        time.sleep(1)\n    await go(worker)\n';
  assert.deepStrictEqual(ids(inline, 'a.py'), []);
  assert.deepStrictEqual(ids(block, 'a.rs'), []);
  assert.deepStrictEqual(ids(nested, 'a.py'), []);
});

test('node sync I/O in an async function is not reported', () => {
  // Deliberate, and measured: `async` in JS marks a function that awaits, not a
  // request path — CLIs, build scripts and tests block on purpose. This test is
  // the one that fails if that decision is ever quietly reversed.
  const src = 'async function main() {\n  const out = execSync("git status");\n}\n';
  assert.deepStrictEqual(ids(src, 'a.js'), []);
});

// --- meant/unimplemented-stub ----------------------------------------------

test('reports a Rust todo!() on a path that ships', () => {
  assert.ok(has('fn price() -> u8 {\n    todo!()\n}\n', 'a.rs', 'meant/unimplemented-stub'));
  assert.ok(has('fn price() -> u8 {\n    todo!("needs the tax table");\n}\n', 'a.rs', 'meant/unimplemented-stub'));
});

test('unimplemented!() is not the same claim and is silent', () => {
  assert.deepStrictEqual(ids('match x {\n    _ => unimplemented!(),\n}\n', 'a.rs'), []);
});

test('the abstract-method idiom of other languages is silent', () => {
  assert.deepStrictEqual(ids('class Base:\n    def price(self):\n        raise NotImplementedError\n', 'a.py'), []);
  assert.deepStrictEqual(ids('price() {\n  throw new Error("not implemented");\n}\n', 'a.ts'), []);
});

test('a todo!() in a comment is not code', () => {
  assert.deepStrictEqual(ids('// todo!() one day\nfn price() -> u8 { 1 }\n', 'a.rs'), []);
});

test('the finding lands on rung MEANT', () => {
  const [f] = checkJudgment('fn price() -> u8 {\n    todo!()\n}\n', { relPath: 'a.rs' });
  assert.strictEqual(f.rung, 'MEANT');
  assert.strictEqual(f.line, 2);
});

// --- wiring ----------------------------------------------------------------

test('every id this file can produce is registered and under a real rung', () => {
  const known = new Set(BUILTIN_RULE_IDS);
  const rungs = new Set(RUNGS.map((r) => r.toLowerCase()));
  const produced = [
    ...ids('for (const row of rows) {\n  await db.query(q);\n}\n', 'a.ts'),
    ...ids('async def h():\n    time.sleep(1)\n', 'a.py'),
    ...ids('fn f() {\n    todo!()\n}\n', 'a.rs'),
  ];
  assert.strictEqual(produced.length, 3);
  for (const id of produced) {
    assert.ok(known.has(id), `${id} is not in BUILTIN_RULE_IDS`);
    assert.ok(rungs.has(id.split('/')[0]), `${id} names no rung`);
  }
});

// A pack that grows an extension this table has never heard of checks nothing
// at rungs 5 and 6 for it, silently. This is what makes that loud.
test('every extension the packs own has a comment style here', () => {
  for (const pack of PACKS) {
    for (const ext of pack.EXTENSIONS) {
      assert.ok(STYLE.has(ext), `${ext} has no entry in judgment.js STYLE`);
    }
  }
});

test('a file type with no pack is left alone', () => {
  assert.deepStrictEqual(ids('for row in rows:\n    db.query(q)\n', 'a.txt'), []);
  assert.deepStrictEqual(ids('for row in rows:\n    db.query(q)\n', 'README.md'), []);
});

test('malformed input does not throw', () => {
  for (const src of ['', '{{{{', 'for (const x of y) {', 'async def', ' ']) {
    assert.doesNotThrow(() => checkJudgment(src, { relPath: 'a.ts' }));
    assert.doesNotThrow(() => checkJudgment(src, { relPath: 'a.py' }));
  }
  assert.doesNotThrow(() => checkJudgment(null, {}));
});

// --- cost ------------------------------------------------------------------

// The same linearity bar the six packs are held to.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check: checkJudgment,
    relPath: 'a.ts',
    config: {},
    baseline: 'const value = compute(input); ',
    units: [
      'for (const row of rows) { await db.query(q); } ',
      'async function f() { ',
      'for (let i = 0; i < rows.length; i++) { ',
      '} { } { ',
      'const dataset = [1], ',
    ],
  });
});

// Nesting is the shape a per-loop body scan would go quadratic on: 400 loops,
// each enclosing the next, is 400 bodies each nearly the whole file.
test('nested loops do not cost a scan of the body per loop', () => {
  const depth = 400;
  const nest = (n) => 'for (const row of rows) {\n'.repeat(n)
    + 'await db.query(q);\n' + '}\n'.repeat(n);
  const small = bestOf(3, () => checkJudgment(nest(depth / 4), { relPath: 'a.ts' }));
  const large = bestOf(3, () => checkJudgment(nest(depth), { relPath: 'a.ts' }));
  // Four times the input, well under sixteen times the cost — the quadratic
  // shape this replaces ran at the square.
  assert.ok(large < Math.max(small, 5) * 8,
    `${depth} nested loops took ${large}ms against ${small}ms for ${depth / 4}`);
});

// --- through the engine ----------------------------------------------------
//
// The rules above are a module; what a user gets is what run.js returns. These
// are the wiring: the finding arrives, a marker naming it silences it, an
// `[exclude] rules` entry silences it, and the `[rungs] fast` severity that has
// existed and been inert since the rung was written now decides something.

const fs = require('fs');
const os = require('os');
const path = require('path');
const { checkFile } = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');

const N_PLUS_ONE = 'for (const row of rows) {\n  await db.query("select 1", [row.id]);\n}\n';

function repoWith(files, toml) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-judgment-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.writeFileSync(path.join(dir, rel), content);
  }
  if (toml) fs.writeFileSync(path.join(dir, '.procoder.toml'), toml);
  return dir;
}

function reportFor(files, name, toml) {
  const dir = repoWith(files, toml);
  return checkFile(path.join(dir, name), {
    repoRoot: dir, config: loadConfig(dir), applyBaseline: false,
  });
}

test('a rung-5 finding reaches the report through checkFile', () => {
  const report = reportFor({ 'n1.ts': N_PLUS_ONE }, 'n1.ts');
  const found = report.findings.find((f) => f.id === 'fast/query-in-loop');
  assert.ok(found, `no rung-5 finding: ${JSON.stringify(report.findings)}`);
  assert.strictEqual(found.rung, 'FAST');
});

test('a rung-6 finding reaches the report through checkFile', () => {
  const report = reportFor({ 'stub.rs': 'fn price() -> u8 {\n    todo!()\n}\n' }, 'stub.rs');
  assert.ok(report.findings.some((f) => f.id === 'meant/unimplemented-stub'),
    JSON.stringify(report.findings));
});

test('a literal marker naming a rung-5 id silences it', () => {
  const marked = 'for (const row of rows) {\n'
    + '  await db.query("select 1", [row.id]);  // procoder: literal fast/query-in-loop three rows, measured\n}\n';
  const report = reportFor({ 'marked.ts': marked }, 'marked.ts');
  assert.deepStrictEqual(report.findings.filter((f) => f.rung === 'FAST'), []);
});

test('[exclude] rules silences a rung-5 id by name', () => {
  const report = reportFor({ 'n1.ts': N_PLUS_ONE }, 'n1.ts',
    '[exclude]\nrules = ["n1.ts:fast/query-in-loop"]\n');
  assert.deepStrictEqual(report.findings.filter((f) => f.rung === 'FAST'), []);
});

test('[rungs] fast is no longer inert', () => {
  const dir = repoWith({ 'n1.ts': N_PLUS_ONE }, '[rungs]\nfast = "error"\n');
  const config = loadConfig(dir);
  const report = checkFile(path.join(dir, 'n1.ts'), { repoRoot: dir, config, applyBaseline: false });
  const found = report.findings.find((f) => f.rung === 'FAST');
  assert.ok(found, JSON.stringify(report.findings));
  // The same question the hook and the CLI ask of a finding to decide whether
  // it blocks — see isBlocking in hooks/procoder-check.js.
  assert.strictEqual(config.rungs[found.rung.toLowerCase()], 'error');
});
