// tests/doctrine.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { LEVELS } = require('../hooks/procoder-config');
const { getProcoderInstructions } = require('../hooks/procoder-instructions');

const doctrine = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder', 'SKILL.md'), 'utf8');

test('has valid skill frontmatter with name and description', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(doctrine);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: procoder$/m);
  assert.match(m[1], /^description: .{40,1024}$/m);
});

test('the four rungs appear in order within the first 2000 chars', () => {
  const head = doctrine.slice(0, 2000);
  const order = ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE'].map((r) => head.indexOf(r));
  assert.ok(order.every((i) => i > -1), 'a rung is missing from the first screen');
  assert.deepStrictEqual(order, [...order].sort((a, b) => a - b), 'rungs out of order');
});

test('level-gated blocks are balanced and use known levels', () => {
  const opens = [...doctrine.matchAll(/<!-- level:([a-z]+) -->/g)];
  const closes = [...doctrine.matchAll(/<!-- \/level -->/g)];
  assert.strictEqual(opens.length, closes.length, 'unbalanced level markers');
  for (const o of opens) {
    assert.ok(['pragmatic', 'strict', 'paranoid'].includes(o[1]), `bad level: ${o[1]}`);
  }
});

test('digest-skip blocks are balanced', () => {
  const opens = [...doctrine.matchAll(/<!-- digest:skip -->/g)].length;
  const closes = [...doctrine.matchAll(/<!-- \/digest -->/g)].length;
  assert.strictEqual(opens, closes, 'unbalanced digest markers');
});

test('both level-gated block types are present', () => {
  assert.match(doctrine, /<!-- level:strict -->/, 'no strict-gated block');
  assert.match(doctrine, /<!-- level:paranoid -->/, 'no paranoid-gated block');
});

// Distinctive phrases, not bare topic words: "error" or "test" appear in any
// coding prose, so a rule could be gutted without the check noticing.
test('covers every spec requirement area', () => {
  // Whitespace-flattened: the doctrine is hard-wrapped, so a phrase can
  // straddle a line break.
  const flat = doctrine.toLowerCase().replace(/\s+/g, ' ');
  for (const phrase of [
    'validate at the boundary, not downstream',
    'allowlist not denylist',
    'parameterized queries only',
    'enforced server-side, per-request',
    'fail loudly at startup when absent',
    'correlation id',
    'a new dependency is a new trust boundary',
    'memory-hard kdf (argon2/bcrypt/scrypt)',
    'no swallowed exceptions, no empty `catch`',
    'money is never a float',
    'a test that passes against a stub is not a test',
    'coverage percentage is never a target',
    'names say **what**, never **how**',
    'comment the **why**',
    'removal trigger',
    'the agent is a boundary too',
    'redaction marker',
    'never by hand-editing the manifest',
    'typecheck → lint → tests → build',
    'three attempts at one file',
    'cost is behavior',
    'compare the diff against the ask both ways',
    '(pre-existing)',
    'ponytail chooses **what to write**',
  ]) {
    assert.ok(flat.includes(phrase), `missing rule: ${phrase}`);
  }
});

test('the four level names and their order are consistent everywhere they are documented', () => {
  // Level semantics are hand-maintained in four places; nothing but this test
  // stops one of them from drifting.
  for (const rel of [
    'skills/procoder/SKILL.md',
    'README.md',
    'commands/level.toml',
    'skills/help/SKILL.md',
  ]) {
    const text = fs.readFileSync(path.join(__dirname, '..', rel), 'utf8').toLowerCase();
    for (const level of LEVELS) {
      assert.match(text, new RegExp(`\\b${level}\\b`), `${rel} never names the level ${level}`);
    }
    assert.match(text, /pragmatic[\s\S]{0,400}?strict[\s\S]{0,400}?paranoid/,
      `${rel} does not list pragmatic → strict → paranoid in ascending order`);
  }
});

// The budget is what the doctrine costs every session, so it moves only when a
// rule is added, never to make room for prose — the last two rules (cost is
// behavior, intent) were paid for by trimming, not by moving the number.
//
// Measured on the RENDERED text at the widest level, not on the source file:
// frontmatter, level markers and digest markers are all stripped before the
// model sees any of it, and a budget that counted them would be a budget on
// authoring machinery rather than on context. The digest gets its own, lower
// ceiling — it is what every subagent pays, so the multiplier lives there.
test('doctrine stays under the context budget', () => {
  const rendered = getProcoderInstructions('paranoid');
  assert.ok(rendered.length < 14000,
    `rendered doctrine is ${rendered.length} chars; budget is 14000`);
});

test('the subagent digest stays well under the full text', () => {
  const digest = getProcoderInstructions('paranoid', { digest: true });
  assert.ok(digest.length < 11000, `digest is ${digest.length} chars; budget is 11000`);
});

test('suppression rule requires narrowest scope and a named rule', () => {
  assert.match(doctrine, /narrowest unit the tool allows/,
    'missing narrowest-scope requirement for suppressions');
  assert.match(doctrine, /eslint-disable-next-line/,  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it
    'missing named-rule suppression example');
  assert.match(doctrine, /unnamed.*unexplained.*stale/,
    'missing rule that an unnamed/unexplained/stale suppression is itself a rung-4 violation');
});
