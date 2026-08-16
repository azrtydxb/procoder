// tests/skill-statusline.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'statusline', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: statusline$/m);
  assert.match(m[1], /install the statusline|statusline/i);
});

test('reads the current state before writing anything', () => {
  const status = skill.indexOf('statusline status');
  const install = skill.indexOf('statusline install');
  assert.ok(status > -1 && install > -1, 'missing a CLI subcommand');
  assert.ok(status < install, 'status must be run before install');
});

test('covers uninstall as well as install', () => {
  assert.ok(skill.includes('statusline uninstall'));
});

test('asks before replacing a statusLine that is not procoder\'s', () => {
  assert.match(skill, /--force/);
  assert.match(skill, /ask|confirm/i);
  assert.match(skill, /^##.*Do not/mi);
  const doNot = skill.slice(skill.search(/^##.*Do not/mi));
  assert.match(doNot, /--force/, 'the Do not section must forbid reaching for --force');
});

test('names the backup and when the badge appears', () => {
  assert.match(skill, /\.backup-/);
  assert.match(skill, /next session/i);
});

test('stays short — this is a wrapper, not an essay', () => {
  assert.ok(skill.split('\n').length < 60, 'statusline skill is too long');
});
