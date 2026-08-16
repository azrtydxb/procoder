// tests/sync-rules.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
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

// The writing CLI must never run against the tracked tree: `npm test` runs
// before `npm run sync:check` on some paths, and a test that regenerates the
// rule files would repair the very drift the CI gate exists to catch.
test('every generated file on disk matches render()', () => {
  for (const [rel, content] of render()) {
    // assert.ok, not strictEqual: a mismatch here would dump both full
    // doctrine renderings into the test output.
    assert.ok(fs.readFileSync(path.join(root, rel), 'utf8') === content,
      `${rel} is out of sync with the doctrine — run: npm run sync`);
  }
});

test('--check exits 0 when in sync and non-zero on real drift', () => {
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-sync-'));
  try {
    for (const dir of ['scripts', 'hooks', 'skills']) {
      fs.cpSync(path.join(root, dir), path.join(scratch, dir), { recursive: true });
    }
    const cli = path.join(scratch, 'scripts/sync-rules.js');

    execFileSync('node', [cli], { cwd: scratch });
    assert.doesNotThrow(() => execFileSync('node', [cli, '--check'], { cwd: scratch }));

    const victim = path.join(scratch, '.clinerules', 'procoder.md');
    fs.appendFileSync(victim, '\nhand-edited drift\n');
    assert.throws(() => execFileSync(
      'node', [cli, '--check'], { cwd: scratch, stdio: 'pipe' }));
  } finally {
    fs.rmSync(scratch, { recursive: true, force: true });
  }
});
