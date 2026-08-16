// tests/universal.test.js
const test = require('node:test');
const assert = require('node:assert');
const { checkUniversal } = require('../hooks/checks/universal');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const run = (src) => checkUniversal(src, { relPath: 'x.js', config });
const ids = (src) => run(src).map((f) => f.id);

test('flags hardcoded secrets of several shapes', () => {
  assert.ok(ids('const k = "AKIAIOSFODNN7EXAMPLE";').includes('safe/hardcoded-secret'));
  assert.ok(ids('token = "ghp_aBcD1234567890aBcD1234567890aBcD12"').includes('safe/hardcoded-secret'));
  assert.ok(ids('-----BEGIN RSA PRIVATE KEY-----').includes('safe/hardcoded-secret'));
  assert.ok(ids('const password = "hunter2correcthorse";').includes('safe/hardcoded-secret'));
});

test('does not flag secrets read from the environment or a manager', () => {
  assert.deepStrictEqual(ids('const key = process.env.API_KEY;'), []);
  assert.deepStrictEqual(ids('password = os.environ["DB_PASSWORD"]'), []);
  assert.deepStrictEqual(ids('const token = await secrets.get("stripe");'), []);
  assert.deepStrictEqual(ids('password = "" # set at startup'), []);
});

test('flags secrets and PII reaching log calls', () => {
  assert.ok(ids('logger.info(`auth ${token}`)').includes('safe/secret-in-log'));
  assert.ok(ids('console.log("authorization: " + req.headers.authorization)').includes('safe/secret-in-log'));
  assert.ok(ids('log.debug(`session ${sessionId} rotated`)').includes('safe/secret-in-log'));
  assert.ok(ids('log.debug(f"user email {user.email}")').includes('safe/pii-in-log'));
  assert.deepStrictEqual(ids('logger.info(`user ${user.id} logged in`)'), []);
});

test('does not flag a sensitive word in the log message when it is not what is interpolated', () => {
  assert.deepStrictEqual(ids('log.info(`password validation ran for ${user.id}`)'), []);
  assert.deepStrictEqual(ids('log.info(`credential check passed for ${user.id}`)'), []);
  assert.deepStrictEqual(ids('logger.info(`user ${user.id} logged in`)'), []);
  assert.deepStrictEqual(ids('logger.warn("password reset requested for " + user.id)'), []);
});

test('still flags when the sensitive word is part of the interpolated expression itself', () => {
  assert.ok(ids('log.info(`${user.password}`)').includes('safe/secret-in-log'));
  assert.ok(ids('log.info(`${config.apiKey}`)').includes('safe/secret-in-log'));
});

test('flags a block of commented-out code but not prose comments', () => {
  const commented = [
    '// const x = compute(a, b);',
    '// if (x > 3) {',
    '//   send(x);',
    '// }',
  ].join('\n');
  assert.ok(ids(commented).includes('alone/commented-code'));

  const prose = [
    '// This exists because the upstream API returns 200 on failure.',
    '// See INFRA-4821 for the vendor ticket.',
    '// Remove once they ship the fix.',
  ].join('\n');
  assert.ok(!ids(prose).includes('alone/commented-code'));
});

test('still flags commented-out code that has no leading keyword', () => {
  const commented = [
    '// const q = `SELECT * FROM t`;',
    '// db.query(q);',
    '// return q;',
  ].join('\n');
  assert.ok(ids(commented).includes('alone/commented-code'));
});

test('reads measured explanations as prose, not commented-out code', () => {
  // The exact comment procoder's own self-scan used to flag: a why-comment that
  // records the measurements behind a threshold. Rung 3 asks for this.
  const measured = [
    '// Total-size skip. Cost of everything that survives the line guard below is',
    '// linear in file size: measured end to end on many-short-line files, 1MB = 34ms,',
    '// 4MB = 137ms, 8MB = 279ms. 4MB is ~7% of the budget and larger than any file a',
    '// human edits, so that is where the skip sits — not the old 256KB, which was',
    '// inherited from when a long line could blow the budget and threw away real',
    '// findings on ordinary large sources.',
  ].join('\n');
  assert.deepStrictEqual(ids(measured), []);

  const formula = [
    '// The ranking is deliberately linear, so a reviewer can predict it:',
    '// score = weight * signal, normalised so that 1.0 = perfect',
    '// Anything cleverer was impossible to explain in review.',
  ].join('\n');
  assert.deepStrictEqual(ids(formula), []);

  const versions = [
    '// The shim exists only for the broken release window.',
    '// Upstream fixed this in v2.4 = 2025-11-03; remove the shim after',
    '// every consumer has moved past that release.',
  ].join('\n');
  assert.deepStrictEqual(ids(versions), []);
});

test('flags TODOs without an owner or ticket', () => {
  assert.ok(ids('// TODO: fix this later').includes('alone/orphan-todo'));
  assert.ok(!ids('// TODO(pascal): drop the shim').includes('alone/orphan-todo'));
  assert.ok(!ids('// TODO INFRA-4821: drop the shim').includes('alone/orphan-todo'));
});

test('flags blanket suppressions but not narrow, named, explained ones', () => {
  // File-wide or unnamed: silences everything at that location, including future findings.
  assert.ok(ids('/* eslint-disable */').includes('alone/blanket-suppression'));
  assert.ok(ids('// eslint-disable-next-line').includes('alone/blanket-suppression'));
  assert.ok(ids('x = compute()  # noqa').includes('alone/blanket-suppression'));
  assert.ok(ids('y = cast(v)  # type: ignore').includes('alone/blanket-suppression'));
  assert.ok(ids('//nolint').includes('alone/blanket-suppression'));
  assert.ok(ids('@SuppressWarnings("all")').includes('alone/blanket-suppression'));

  // Named + scoped + explained: the sanctioned form, must stay silent.
  assert.deepStrictEqual(
    ids('// eslint-disable-next-line no-eval -- sandboxed evaluator, input is a literal'), []);
  assert.deepStrictEqual(ids('x = compute()  # noqa: E501 - URL cannot be wrapped'), []);
  assert.deepStrictEqual(
    ids('//nolint:errcheck // Close() on a read-only handle cannot fail'), []);
});

test('flags a named suppression that gives no reason', () => {
  assert.ok(ids('// eslint-disable-next-line no-eval').includes('alone/unexplained-suppression'));
  assert.ok(ids('#pragma warning disable CS0618').includes('alone/unexplained-suppression'));
});

test('flags a suppression with a reason expressed as trailing prose, no separator', () => {
  assert.deepStrictEqual(
    ids('# noqa: E501 this line intentionally exceeds the limit for readability'), []);
});

test('flags deprecations with no removal trigger', () => {
  assert.ok(ids('// @deprecated use createUser instead').includes('alone/deprecated-no-trigger'));
  assert.ok(!ids('// @deprecated remove after v3.0').includes('alone/deprecated-no-trigger'));
  assert.ok(!ids('// procoder: remove after the 2026-10 migration').includes('alone/deprecated-no-trigger'));
});

test('the clean fixture produces no findings and the dirty one produces several', () => {
  const fs = require('fs');
  const path = require('path');
  const dir = path.join(__dirname, 'fixtures', 'universal');
  assert.strictEqual(
    checkUniversal(fs.readFileSync(path.join(dir, 'clean.txt'), 'utf8'),
      { relPath: 'clean.txt', config }).length, 0);
  assert.ok(
    checkUniversal(fs.readFileSync(path.join(dir, 'dirty.txt'), 'utf8'),
      { relPath: 'dirty.txt', config }).length >= 5);
});

test('every finding carries a line number and a fix', () => {
  for (const f of run('const k = "AKIAIOSFODNN7EXAMPLE";\n// TODO: later\n')) {
    assert.ok(f.line > 0, `${f.id} has no line`);
    assert.ok(f.fix.length > 0, `${f.id} has no fix`);
  }
});

test('stays cheap on a long comment run of long near-code lines', () => {
  // Worst case for the assignment arm: every line starts with something that
  // walks the member-expression loop before failing to reach an `=`.
  const line = '// ' + 'a.b[0].c'.repeat(500) + ' measured at 1MB = 34ms';
  const start = Date.now();
  run(Array.from({ length: 2000 }, () => line).join('\n'));
  const ms = Date.now() - start;
  assert.ok(ms < 500, `took ${ms}ms on pathological comment run`);
});

test('does not catastrophically backtrack on a long line', () => {
  const long = 'x = ' + 'a'.repeat(20000) + ';  // eslint-disable-next-line no-eval no reason here but long';
  const start = Date.now();
  run(long);
  assert.ok(Date.now() - start < 500, 'took too long on pathological input');
});

// A 300KB minified bundle on one line — the file the pack is exempted from the
// line-length guard in order to keep scanning for credentials. Each shape below
// used to drive an unbounded `[^x]*` span to end-of-line from every start
// position: 300KB cost ~36s, against a 2s hook budget.
//
// Bound: 500ms for all three, measured at ~2ms each. The margin is deliberately
// enormous — a loaded CI runner can lose a factor of fifty and still pass, while
// any return of the quadratic behaviour costs seconds to tens of seconds and
// cannot slip under it. The point of the assertion is the growth rate, not the
// constant, so it is not tightened to the measurement.
test('stays linear on a single 300KB line', () => {
  const rep = (unit) => unit.repeat(Math.ceil((300 * 1024) / unit.length));
  const shapes = {
    'word run in a comment': '// ' + rep('a'),
    'unclosed parens in a comment': '// ' + rep('a('),
    'unclosed interpolation in a log call': 'console.log("' + rep('${') + '")',
  };
  for (const [what, source] of Object.entries(shapes)) {
    const start = Date.now();
    run(source);
    const ms = Date.now() - start;
    assert.ok(ms < 500, `${what}: took ${ms}ms on a 300KB line`);
  }
});

// A credential on such a line is exactly what the exemption exists to catch, so
// speed must not have been bought by skipping the line.
test('finds a credential on a 300KB minified line', () => {
  const filler = 'function e(t){return t.x?f(t,1):g(t)};var n=[1,2,3];'.repeat(3000);
  const findings = run(`${filler};var q="AKIAIOSFODNN7EXAMPLE";${filler}`);
  assert.ok(findings.some((f) => f.id === 'safe/hardcoded-secret'),
    'a hardcoded AWS key on a 300KB line went unreported');
});

// ---------------------------------------------------------------------------
// The literal marker: text that describes a pattern, marked as such.
//
// MARK is assembled at runtime rather than written out, so that the lines of
// this section are not themselves markers — the tests below feed marker text
// to the pack as *data*, and a real marker in the test source would silence
// the very findings these tests assert on. Likewise the AWS key: split, so it
// is a credential only once the pack sees it.
const MARK = 'procoder' + ': literal ';
const KEY = 'AKIA' + 'IOSFODNN7EXAMPLE';

test('a trailing literal marker silences the rules it names, on its line only', () => {
  assert.deepStrictEqual(
    ids(`const k = "${KEY}"; // ${MARK}safe/hardcoded-secret sample key in a doc`), []);

  // Trailing, so it reaches no further than the line it sits on.
  assert.ok(ids(`const a = 1; // ${MARK}safe/hardcoded-secret sample key in a doc\nconst k = "${KEY}";`)
    .includes('safe/hardcoded-secret'));
});

test('a standalone literal marker reaches the following line, and no further', () => {
  assert.deepStrictEqual(
    ids(`// ${MARK}safe/hardcoded-secret the key below is documentation\nconst k = "${KEY}";`), []);

  assert.ok(ids(`// ${MARK}safe/hardcoded-secret the key below is documentation\nconst a = 1;\nconst k = "${KEY}";`)
    .includes('safe/hardcoded-secret'));
});

test('a literal marker silences nothing it did not name', () => {
  assert.ok(ids(`const k = "${KEY}"; // ${MARK}alone/orphan-todo wrong rule named here`)
    .includes('safe/hardcoded-secret'));
  assert.deepStrictEqual(
    ids(`// TODO: later ${MARK}alone/orphan-todo describes an unowned marker`), []);
});

test('a literal marker may name several rules', () => {
  assert.deepStrictEqual(
    ids(`// TODO: later, deprecated too // ${MARK}alone/orphan-todo, alone/deprecated-no-trigger both described here`),
    []);
});

test('a bare literal marker is a blanket suppression and suppresses nothing', () => {
  const found = ids(`const k = "${KEY}"; // procoder` + ': literal');
  assert.ok(found.includes('alone/blanket-suppression'), 'a marker naming no rule must be reported');
  assert.ok(found.includes('safe/hardcoded-secret'), 'a marker naming no rule must silence nothing');
});

test('a literal marker with no reason is unexplained and suppresses nothing', () => {
  const found = ids(`const k = "${KEY}"; // ${MARK}safe/hardcoded-secret`);
  assert.ok(found.includes('alone/unexplained-suppression'), 'a marker with no reason must be reported');
  assert.ok(found.includes('safe/hardcoded-secret'), 'a marker with no reason must silence nothing');
});

test('a well-formed literal marker is not itself reported as a suppression', () => {
  assert.deepStrictEqual(
    ids(`// ${MARK}alone/orphan-todo this line is a marker and nothing else`), []);
});

// The marker is only useful if it reaches the language packs too: most of what
// a test file or a doctrine page describes is an injection sink or a debug
// statement, not a credential. checkFile applies this to every finding it has.
test('filterMarkedLiterals applies the same marker to findings from any pack', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `const q = "SELECT 1"; // ${MARK}safe/sql-injection the query above is a doc example\nconst r = "SELECT 2";`;
  const findings = [
    { id: 'safe/sql-injection', line: 1 },
    { id: 'safe/sql-injection', line: 2 },
    { id: 'safe/xss-sink', line: 1 },
  ];
  assert.deepStrictEqual(
    filterMarkedLiterals(source, findings).map((f) => `${f.id}:${f.line}`),
    ['safe/sql-injection:2', 'safe/xss-sink:1']);
});

test('marker scanning is cheap on a large marker-free file', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = Array.from({ length: 20000 }, (_, i) => `const a${i} = compute(${i});`).join('\n');
  const findings = [{ id: 'safe/hardcoded-secret', line: 1 }];
  const start = Date.now();
  for (let i = 0; i < 20; i += 1) filterMarkedLiterals(source, findings);
  const ms = Date.now() - start;
  assert.ok(ms < 200, `marker scan cost ${ms}ms over 20 passes of a 20k-line file`);
});
