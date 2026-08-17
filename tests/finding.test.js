// tests/finding.test.js
const test = require('node:test');
const assert = require('node:assert');
const { finding, sortFindings, capFindings, formatFindings, RUNGS, rungIndex } =
  require('../hooks/checks/finding');

const f = (rung, line, id = 'x/y') =>
  finding({ rung, id, line, message: 'msg', fix: 'do the thing' });

test('sorts by rung order first, then by line', () => {
  const sorted = sortFindings([f('ALONE', 1), f('SAFE', 9), f('OBVIOUS', 2), f('SAFE', 3)]);
  assert.deepStrictEqual(
    sorted.map((x) => [x.rung, x.line]),
    [['SAFE', 3], ['SAFE', 9], ['OBVIOUS', 2], ['ALONE', 1]]);
});

test('rungIndex follows the doctrine order', () => {
  assert.deepStrictEqual(RUNGS, ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE', 'FAST', 'MEANT']);
  assert.ok(rungIndex('SAFE') < rungIndex('TRUE'));
  assert.ok(rungIndex('OBVIOUS') < rungIndex('ALONE'));
  assert.ok(rungIndex('ALONE') < rungIndex('FAST'));
  assert.ok(rungIndex('FAST') < rungIndex('MEANT'));
});

test('caps to the limit after sorting, keeping the most severe', () => {
  const capped = capFindings(sortFindings([
    f('ALONE', 1), f('SAFE', 2), f('OBVIOUS', 3), f('TRUE', 4), f('ALONE', 5), f('SAFE', 6),
  ]), 3);
  assert.strictEqual(capped.length, 3);
  assert.deepStrictEqual(capped.map((x) => x.rung), ['SAFE', 'SAFE', 'TRUE']);
});

test('formats one line per finding in the doctrine shape', () => {
  const out = formatFindings([
    finding({ rung: 'SAFE', id: 'safe/raw-input', line: 42, message: 'raw req.body.role into authz check', fix: 'validate + server-side role lookup' }),
  ], 'api/users.ts');
  assert.strictEqual(
    out.trim(),
    '[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup');
});

test('formatting an empty list yields an empty string', () => {
  assert.strictEqual(formatFindings([], 'x.ts'), '');
});

test('finding rejects an unknown rung', () => {
  assert.throws(() => finding({ rung: 'QUICK', id: 'a/b', line: 1, message: 'm', fix: 'f' }));
});
