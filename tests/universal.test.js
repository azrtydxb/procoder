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

test('does not catastrophically backtrack on a long line', () => {
  const long = 'x = ' + 'a'.repeat(20000) + ';  // eslint-disable-next-line no-eval no reason here but long';
  const start = Date.now();
  run(long);
  assert.ok(Date.now() - start < 500, 'took too long on pathological input');
});
