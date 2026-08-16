// tests/skill-rot.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-rot', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: procoder-rot$/m);
  assert.match(m[1], /dead code|stale|deprecated|unused/i);
});

test('covers every rot category from the spec', () => {
  for (const category of [
    'export', 'commented', 'feature flag', 'deprecat', 'removal trigger',
    'documentation', 'dependenc', 'fixture',
  ]) {
    assert.match(skill.toLowerCase(), new RegExp(category), `missing: ${category}`);
  }
});

test('requires verification before recommending deletion', () => {
  assert.match(skill, /git grep|rg |ripgrep|search/i);
  assert.match(skill, /dynamic|reflection|string|entry ?point/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
