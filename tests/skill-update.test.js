// tests/skill-update.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'update', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: update$/m);
  assert.match(m[1], /update procoder|latest version/i);
});

test('detects the install rather than assuming one', () => {
  assert.match(skill, /CLAUDE_CONFIG_DIR/);
  assert.match(skill, /plugins/);
  assert.match(skill, /local clone|git clone|clone/i);
  assert.match(skill, /cannot find|not found|no install/i);
});

test('reports both versions before changing anything', () => {
  assert.match(skill, /\.claude-plugin\/plugin\.json/);
  assert.match(skill, /git [^\n]*\b(fetch|ls-remote)\b/);
  const announce = skill.search(/before .*(chang|updat|run)|do not .*(run|updat).*before/i);
  assert.ok(announce > -1, 'must state that nothing runs before the plan is shown');
});

test('names the update mechanism it actually runs', () => {
  assert.match(skill, /git -C/);
  assert.match(skill, /claude.*PATH|PATH.*claude|\/plugin/i);
});

test('summarises the changelog rather than pasting it', () => {
  assert.match(skill, /CHANGELOG\.md/);
  assert.match(skill, /summari[sz]e|few lines|do not paste/i);
});

test('warns when the baseline format version changed', () => {
  assert.match(skill, /hooks\/checks\/baseline\.js/);
  assert.match(skill, /BASELINE_VERSION/);
  assert.match(skill, /procoder baseline/);
  assert.match(skill, /verify[^\n]*exits? 2/i);
});

test('warns the platforms that hold copies of the generated files', () => {
  assert.match(skill, /skills\/procoder\/SKILL\.md/);
  for (const target of ['.cursor/rules/procoder.mdc', '.windsurf/rules/procoder.md',
    '.clinerules/procoder.md', '.kiro/steering/procoder.md', '.qoder/rules/procoder.md']) {
    assert.ok(skill.includes(target), `missing re-copy target: ${target}`);
  }
  assert.match(skill, /docs\/install\.md/);
});

test('stops on local modifications and on an already-current install', () => {
  assert.match(skill, /git status --porcelain/);
  assert.match(skill, /already .*(up to date|current)/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*Do not/mi);
  const doNot = skill.slice(skill.search(/^##.*Do not/mi));
  assert.match(doNot, /reset --hard|checkout --|discard|overwrite/i);
});
