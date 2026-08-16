// tests/sync-rules.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const { render, TARGETS } = require('../scripts/sync-rules');

const root = path.join(__dirname, '..');

test('renders a file for every declared target', () => {
  const out = render();
  assert.strictEqual(out.size, TARGETS.length);
  for (const target of TARGETS) {
    assert.ok(out.has(target.path), `missing ${target.path}`);
  }
});

test('every rendered file carries the generated-file warning and the ladder', () => {
  for (const [file, content] of render()) {
    assert.match(content, /DO NOT EDIT/, `${file} missing warning`);
    assert.match(content, /skills\/procoder\/SKILL\.md/, `${file} missing source pointer`);
    assert.match(content, /SAFE/, `${file} missing the ladder`);
  }
});

test('cursor target gets .mdc frontmatter, others do not', () => {
  const out = render();
  assert.match(out.get('.cursor/rules/procoder.mdc'), /^---\nalwaysApply: true\n/);
  assert.ok(!out.get('.clinerules/procoder.md').startsWith('---\nalwaysApply'));
});

test('--check exits 0 when files are in sync', () => {
  execFileSync('node', [path.join(root, 'scripts/sync-rules.js')], { cwd: root });
  assert.doesNotThrow(() => execFileSync(
    'node', [path.join(root, 'scripts/sync-rules.js'), '--check'], { cwd: root }));
});

test('--check exits non-zero after a generated file drifts', () => {
  const victim = path.join(root, '.clinerules', 'procoder.md');
  const saved = fs.readFileSync(victim, 'utf8');
  try {
    fs.writeFileSync(victim, saved + '\nhand-edited drift\n');
    assert.throws(() => execFileSync(
      'node', [path.join(root, 'scripts/sync-rules.js'), '--check'], { cwd: root, stdio: 'pipe' }));
  } finally {
    fs.writeFileSync(victim, saved);
  }
});
