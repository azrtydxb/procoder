// tests/lang-ts.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/ts');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.ts', config }).map((f) => f.id);

test('declares the extensions it owns', () => {
  assert.ok(EXTENSIONS.includes('.ts') && EXTENSIONS.includes('.jsx'));
});

test('flags SQL built by string concatenation or template', () => {
  assert.ok(ids('db.query(`SELECT * FROM users WHERE id = ${id}`)').includes('safe/sql-injection'));
  assert.ok(ids('db.query("SELECT * FROM t WHERE a = " + a)').includes('safe/sql-injection'));
  assert.ok(!ids('db.query("SELECT * FROM users WHERE id = ?", [id])').includes('safe/sql-injection'));
});

test('flags XSS sinks and dynamic evaluation', () => {
  assert.ok(ids('el.innerHTML = userInput;').includes('safe/xss-sink'));
  assert.ok(ids('<div dangerouslySetInnerHTML={{ __html: body }} />').includes('safe/xss-sink'));
  assert.ok(ids('eval(payload)').includes('safe/dynamic-eval'));
  assert.ok(!ids('el.textContent = userInput;').includes('safe/xss-sink'));
});

test('flags disabled TLS verification and weak randomness for tokens', () => {
  assert.ok(ids('rejectUnauthorized: false').includes('safe/tls-disabled'));
  assert.ok(ids('const token = Math.random().toString(36);').includes('safe/weak-random'));
  assert.ok(!ids('const jitter = Math.random() * 100;').includes('safe/weak-random'));
});

test('flags swallowed errors and floating promises', () => {
  assert.ok(ids('try { go(); } catch (e) {}').includes('true/swallowed-error'));
  assert.ok(ids('try { go(); } catch (e) { /* ignore */ }').includes('true/swallowed-error'));
  assert.ok(!ids('try { go(); } catch (e) { logger.error(e); }').includes('true/swallowed-error'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('console.log("here")').includes('alone/debug-leftover'));
  assert.ok(ids('debugger;').includes('alone/debug-leftover'));
  assert.ok(!ids('logger.info("started")').includes('alone/debug-leftover'));
});

test('flags shape violations via the shared metrics', () => {
  const long = 'function big(a, b, c, d, e) {\n' + '  work();\n'.repeat(60) + '}\n';
  const found = ids(long);
  assert.ok(found.includes('obvious/function-too-long'));
  assert.ok(found.includes('obvious/too-many-params'));
});

test('flags nested ternaries', () => {
  assert.ok(ids('const x = a ? b ? 1 : 2 : 3;').includes('obvious/nested-ternary'));
  assert.ok(!ids('const x = a ? 1 : 2;').includes('obvious/nested-ternary'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'ts');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.ts'), 'utf8'),
    { relPath: 'clean.ts', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.ts'), 'utf8'),
    { relPath: 'dirty.ts', config }).length >= 6);
});
