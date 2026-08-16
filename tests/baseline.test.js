// tests/baseline.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  fingerprint, fingerprintsFor, loadBaseline, writeBaseline, suppress, growthCheck,
  BASELINE_VERSION,
} = require('../hooks/checks/baseline');
const { finding } = require('../hooks/checks/finding');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS };
const tempRepo = () => fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-base-'));
const f = (line, id = 'alone/orphan-todo') =>
  finding({ rung: 'ALONE', id, line, message: 'm', fix: 'x' });

test('fingerprint ignores line numbers and surrounding whitespace', () => {
  const a = fingerprint(f(10), 'src/a.ts', '  // TODO: later');  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  const b = fingerprint(f(99), 'src/a.ts', '// TODO: later');  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  assert.strictEqual(a, b, 'a reformat must not change the fingerprint');
});

test('fingerprint distinguishes different files, ids, and content', () => {
  const base = fingerprint(f(1), 'src/a.ts', '// TODO: later');  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  assert.notStrictEqual(base, fingerprint(f(1), 'src/b.ts', '// TODO: later'));  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  assert.notStrictEqual(base, fingerprint(f(1, 'alone/commented-code'), 'src/a.ts', '// TODO: later'));  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  assert.notStrictEqual(base, fingerprint(f(1), 'src/a.ts', '// TODO: something else'));  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
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

// The ordinal changed every fingerprint, so v1 entries cannot be migrated —
// they never recorded which occurrence they were. Loading them anyway would
// suppress nothing and light up the user's whole backlog as if it were new.
test('a baseline written in an older format loads empty and says so', () => {
  const repo = tempRepo();
  fs.writeFileSync(path.join(repo, '.procoder-baseline.json'),
    JSON.stringify({ version: 1, fingerprints: ['aaa', 'bbb'] }));
  const loaded = loadBaseline(repo, config);
  assert.strictEqual(loaded.size, 0);
  assert.strictEqual(loaded.staleVersion, 1);
});

// Every fingerprint format change is a hard break, including the most recent
// one: silently loading v-previous entries would suppress nothing and dump a
// legacy repo's whole backlog on the user as if it were new.
test('a baseline in the immediately previous format loads empty and says so', () => {
  const repo = tempRepo();
  const previous = BASELINE_VERSION - 1;
  fs.writeFileSync(path.join(repo, '.procoder-baseline.json'),
    JSON.stringify({ version: previous, fingerprints: ['aaa', 'bbb'] }));
  const loaded = loadBaseline(repo, config);
  assert.strictEqual(loaded.size, 0);
  assert.strictEqual(loaded.staleVersion, previous);
});

test('a current-format baseline loads without a stale marker', () => {
  const repo = tempRepo();
  writeBaseline(repo, config, ['aaa']);
  const loaded = loadBaseline(repo, config);
  assert.strictEqual(loaded.staleVersion, undefined);
  assert.strictEqual(
    JSON.parse(fs.readFileSync(path.join(repo, '.procoder-baseline.json'), 'utf8')).version,
    BASELINE_VERSION);
});

test('suppress removes baselined findings and keeps new ones', () => {
  const lines = ['// TODO: later', 'const x = 1;', '// TODO: other'];  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  const known = new Set([fingerprint(f(1), 'a.ts', lines[0])]);
  const out = suppress([f(1), f(3)], { baseline: known, relPath: 'a.ts', lines });
  assert.strictEqual(out.length, 1);
  assert.strictEqual(out[0].line, 3);
});

test('suppression survives the file being reformatted', () => {
  const before = ['// TODO: later'];  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  const after = ['    // TODO: later'];  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  const known = new Set([fingerprint(f(1), 'a.ts', before[0])]);
  assert.deepStrictEqual(
    suppress([f(1)], { baseline: known, relPath: 'a.ts', lines: after }), []);
});

// One accepted occurrence accepts one occurrence. A second identical line is a
// new violation, even though id, path and normalized content are identical.
test('suppress accepts only as many duplicates as the baseline recorded', () => {
  const lines = ['// TODO: later', '// TODO: later', '// TODO: later'];  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  const known = new Set([fingerprint(f(1), 'a.ts', lines[0])]);
  const out = suppress([f(1), f(2), f(3)], { baseline: known, relPath: 'a.ts', lines });
  assert.strictEqual(out.length, 2);
});

// The limitation this closes: a fingerprint used to hash one line's text, so
// `prettier --write .` over a freshly baselined legacy repo resurrected every
// wrapped statement as a new finding and turned CI red on code nobody wrote.
// Identity is now the token sequence of the whole logical statement.
const FLAT = ['const r = doThing(userInput, options);'];
const WRAPPED = ['const r = doThing(', '  userInput,', '  options,', ');'];
const fpOf = (findings, relPath, lines) => fingerprintsFor(findings, relPath, lines);

test('a statement re-wrapped across lines keeps its fingerprint', () => {
  assert.strictEqual(fpOf([f(1)], 'a.ts', WRAPPED)[0], fpOf([f(1)], 'a.ts', FLAT)[0]);
});

test('suppression survives a re-wrapping formatter', () => {
  const known = new Set(fpOf([f(1)], 'a.ts', FLAT));
  assert.deepStrictEqual(suppress([f(1)], { baseline: known, relPath: 'a.ts', lines: WRAPPED }), []);
});

// Prettier and black both add a trailing comma when they move a closer onto its
// own line. It is punctuation the author never wrote, so it cannot be identity.
test('a trailing comma added by the wrap does not change identity', () => {
  const noComma = ['const r = doThing(', '  userInput,', '  options', ');'];
  assert.strictEqual(fpOf([f(1)], 'a.ts', noComma)[0], fpOf([f(1)], 'a.ts', FLAT)[0]);
});

// Prettier breaks a long assignment or concatenation after the operator, with
// no bracket open to signal the continuation.
test('a statement wrapped after a trailing operator keeps its fingerprint', () => {
  const flat = ["const msg = greeting + ' ' + name;"];
  const wrapped = ['const msg = greeting +', "  ' ' +", '  name;'];
  assert.strictEqual(fpOf([f(1)], 'a.ts', wrapped)[0], fpOf([f(1)], 'a.ts', flat)[0]);
});

test('re-wrapping does not merge a finding with a genuinely different one', () => {
  const other = ['const r = doThing(', '  otherInput,', '  options,', ');'];
  assert.notStrictEqual(fpOf([f(1)], 'a.ts', WRAPPED)[0], fpOf([f(1)], 'a.ts', other)[0]);
  assert.notStrictEqual(
    fpOf([f(1)], 'a.ts', WRAPPED)[0],
    fpOf([f(1, 'alone/commented-code')], 'a.ts', WRAPPED)[0]);
});

// Whitespace is not identity, but token boundaries are: collapsing the window
// to a bare character string would make `const x` and `constx` the same code.
test('removing the space between two tokens is a different construct', () => {
  assert.notStrictEqual(
    fpOf([f(1)], 'a.ts', ['return x'])[0], fpOf([f(1)], 'a.ts', ['returnx'])[0]);
});

test('two identical wrapped statements are still told apart', () => {
  const lines = [...WRAPPED, ...WRAPPED];
  const fps = fpOf([f(1), f(5)], 'a.ts', lines);
  assert.notStrictEqual(fps[0], fps[1]);
  const known = new Set([fps[0]]);
  assert.strictEqual(
    suppress([f(1), f(5)], { baseline: known, relPath: 'a.ts', lines }).length, 1);
});

// A line ending in `{` opens a block far more often than it wraps an
// expression. Following it would tie a finding on a function's signature to
// every line of its body, so any edit inside would resurrect it.
test('a finding on a signature does not depend on the body below it', () => {
  const a = ['function foo(name) {', '  return one(name);', '}'];
  const b = ['function foo(name) {', '  return two(name);', '}'];
  assert.strictEqual(fpOf([f(1)], 'a.ts', a)[0], fpOf([f(1)], 'a.ts', b)[0]);
});

// The window is bounded so a pathological open bracket cannot make one
// finding's identity depend on half the file.
test('the statement window is bounded', () => {
  const long = (tail) => ['const r = doThing('].concat(
    Array.from({ length: 40 }, (_, i) => `  arg${i},`), [tail, ');']);
  assert.strictEqual(
    fpOf([f(1)], 'a.ts', long('  last,'))[0], fpOf([f(1)], 'a.ts', long('  other,'))[0]);
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
