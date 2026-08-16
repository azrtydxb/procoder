// tests/baseline.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  fingerprint, loadBaseline, writeBaseline, suppress, growthCheck,
} = require('../hooks/checks/baseline');
const { finding } = require('../hooks/checks/finding');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS };
const tempRepo = () => fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-base-'));
const f = (line, id = 'alone/orphan-todo') =>
  finding({ rung: 'ALONE', id, line, message: 'm', fix: 'x' });

test('fingerprint ignores line numbers and surrounding whitespace', () => {
  const a = fingerprint(f(10), 'src/a.ts', '  // TODO: later');
  const b = fingerprint(f(99), 'src/a.ts', '// TODO: later');
  assert.strictEqual(a, b, 'a reformat must not change the fingerprint');
});

test('fingerprint distinguishes different files, ids, and content', () => {
  const base = fingerprint(f(1), 'src/a.ts', '// TODO: later');
  assert.notStrictEqual(base, fingerprint(f(1), 'src/b.ts', '// TODO: later'));
  assert.notStrictEqual(base, fingerprint(f(1, 'alone/commented-code'), 'src/a.ts', '// TODO: later'));
  assert.notStrictEqual(base, fingerprint(f(1), 'src/a.ts', '// TODO: something else'));
});

test('baseline round-trips through disk', () => {
  const repo = tempRepo();
  writeBaseline(repo, config, ['aaa', 'bbb']);
  const loaded = loadBaseline(repo, config);
  assert.ok(loaded.has('aaa') && loaded.has('bbb'));
  assert.strictEqual(loaded.size, 2);
});

test('an absent or corrupt baseline file loads as empty, never throws', () => {
  const repo = tempRepo();
  assert.strictEqual(loadBaseline(repo, config).size, 0);
  fs.writeFileSync(path.join(repo, '.procoder-baseline.json'), 'not json');
  assert.strictEqual(loadBaseline(repo, config).size, 0);
});

test('suppress removes baselined findings and keeps new ones', () => {
  const lines = ['// TODO: later', 'const x = 1;', '// TODO: other'];
  const known = new Set([fingerprint(f(1), 'a.ts', lines[0])]);
  const out = suppress([f(1), f(3)], { baseline: known, relPath: 'a.ts', lines });
  assert.strictEqual(out.length, 1);
  assert.strictEqual(out[0].line, 3);
});

test('suppression survives the file being reformatted', () => {
  const before = ['// TODO: later'];
  const after = ['    // TODO: later'];
  const known = new Set([fingerprint(f(1), 'a.ts', before[0])]);
  assert.deepStrictEqual(
    suppress([f(1)], { baseline: known, relPath: 'a.ts', lines: after }), []);
});

// One accepted occurrence accepts one occurrence. A second identical line is a
// new violation, even though id, path and normalized content are identical.
test('suppress accepts only as many duplicates as the baseline recorded', () => {
  const lines = ['// TODO: later', '// TODO: later', '// TODO: later'];
  const known = new Set([fingerprint(f(1), 'a.ts', lines[0])]);
  const out = suppress([f(1), f(2), f(3)], { baseline: known, relPath: 'a.ts', lines });
  assert.strictEqual(out.length, 2);
});

test('growthCheck passes when every current finding is baselined', () => {
  const baseline = new Set(['a', 'b', 'c']);
  assert.strictEqual(growthCheck(baseline, new Set(['a', 'b', 'c'])).ok, true);
  assert.strictEqual(growthCheck(baseline, new Set(['a'])).ok, true);
  assert.strictEqual(growthCheck(baseline, new Set()).ok, true);
});

test('growthCheck reports the findings the baseline never accepted', () => {
  const baseline = new Set(['a', 'b', 'c']);
  const result = growthCheck(baseline, new Set(['a', 'd', 'e']));
  assert.strictEqual(result.ok, false);
  assert.strictEqual(result.delta, 2);
  assert.deepStrictEqual(result.added, ['d', 'e']);
});

// The point of the ratchet: fixing one finding must not buy room for a
// different one. A count-based comparison passes this; identity must not.
test('growthCheck fails when a finding is swapped for a new one of equal count', () => {
  assert.strictEqual(growthCheck(new Set(['a']), new Set(['b'])).ok, false);
});
