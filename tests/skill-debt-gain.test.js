// tests/skill-debt-gain.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const read = (name) => fs.readFileSync(
  path.join(__dirname, '..', 'skills', name, 'SKILL.md'), 'utf8');

test('debt skill finds procoder markers and flags missing removal triggers', () => {
  const skill = read('procoder-debt');
  assert.match(/^---\n([\s\S]*?)\n---\n/.exec(skill)[1], /^name: procoder-debt$/m);
  assert.match(skill, /procoder:/);
  assert.match(skill, /removal trigger/i);
  assert.match(skill, /git log|git blame/);
});

test('gain skill measures against the baseline, not vibes', () => {
  const skill = read('procoder-gain');
  assert.match(/^---\n([\s\S]*?)\n---\n/.exec(skill)[1], /^name: procoder-gain$/m);
  assert.match(skill, /baseline/i);
  assert.match(skill, /git diff --stat|git log/);
  assert.match(skill, /deleted|removed/i);
});

test('neither skill invents a score or grade', () => {
  for (const name of ['procoder-debt', 'procoder-gain']) {
    assert.ok(!/\bgrade\b|\bscore\b|\bA\+|\bB-\b/.test(read(name)),
      `${name} introduces a score, which the spec forbids`);
  }
});

test('both state what NOT to do', () => {
  for (const name of ['procoder-debt', 'procoder-gain']) {
    assert.match(read(name), /^##.*(?:Do not|Never)/mi, `${name} missing a Do not section`);
  }
});
