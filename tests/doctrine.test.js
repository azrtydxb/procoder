// tests/doctrine.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

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

test('both level-gated block types are present', () => {
  assert.match(doctrine, /<!-- level:strict -->/, 'no strict-gated block');
  assert.match(doctrine, /<!-- level:paranoid -->/, 'no paranoid-gated block');
});

test('covers every spec requirement area', () => {
  for (const topic of [
    'trust boundar', 'parameterized', 'authorization', 'secret',
    'PII', 'dependenc', 'error', 'test', 'naming', 'why',
    'removal trigger', 'ponytail',
  ]) {
    assert.match(doctrine.toLowerCase(), new RegExp(topic.toLowerCase()), `missing: ${topic}`);
  }
});

test('doctrine stays under the context budget', () => {
  assert.ok(doctrine.length < 12000, `doctrine is ${doctrine.length} chars; budget is 12000`);
});
