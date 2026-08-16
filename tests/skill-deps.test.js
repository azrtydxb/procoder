// tests/skill-deps.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-deps', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: procoder-deps$/m);
  assert.match(m[1], /dependenc|supply chain|vulnerab|package/i);
});

test('uses the project audit tools rather than guessing', () => {
  for (const tool of ['npm audit', 'pip-audit', 'govulncheck', 'cargo audit']) {
    assert.ok(skill.includes(tool), `missing: ${tool}`);
  }
});

test('covers abandonment, pinning, and unused dependencies', () => {
  assert.match(skill, /abandon|unmaintained|last release/i);
  assert.match(skill, /pin|lockfile/i);
  assert.match(skill, /unused/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
