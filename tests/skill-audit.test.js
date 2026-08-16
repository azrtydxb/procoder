// tests/skill-audit.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-audit', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: procoder-audit$/m);
  assert.match(m[1], /audit|whole repo|entire codebase/i);
});

test('offers the baseline as the adoption path', () => {
  assert.match(skill, /procoder\.js baseline|procoder baseline/);
  assert.match(skill, /ratchet|baseline/i);
});

test('ranks findings and caps the report', () => {
  assert.match(skill, /rank|ranked/i);
  assert.match(skill, /\btop\b|\bcap\b|\blimit\b/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
