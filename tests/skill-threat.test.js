// tests/skill-threat.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'threat', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: threat$/m);
  assert.match(m[1], /threat|trust boundar|attack surface|security review/i);
});

test('enumerates entry points and sinks', () => {
  for (const term of [
    'handler', 'queue', 'webhook', 'environment', 'deserializ',
    'sql', 'shell', 'authoriz',
  ]) {
    assert.match(skill.toLowerCase(), new RegExp(term), `missing: ${term}`);
  }
});

test('produces a boundary table, not prose', () => {
  assert.match(skill, /\|.*boundar.*\|/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
