// tests/skill-review.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'review', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: review$/m);
  assert.match(m[1], /review/i);
  assert.match(m[1], /diff|changes|staged/i);
});

test('runs the deterministic engine rather than eyeballing', () => {
  assert.match(skill, /bin\/procoder\.js check|procoder check/);
  assert.match(skill, /git diff/);
});

test('covers all four rungs by name', () => {
  for (const rung of ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE']) {
    assert.match(skill, new RegExp(`\\b${rung}\\b`), `missing rung ${rung}`);
  }
});

test('fixes the output format and forbids essays', () => {
  assert.match(skill, /\[1 SAFE\]/);
  assert.match(skill, /one line per finding/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
