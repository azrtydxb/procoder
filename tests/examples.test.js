// procoder — the examples tree is executable documentation: each before.ts
// must trip its rung through the real check engine, and each after.ts must be
// clean, or the example proves nothing.
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');
const CLI = path.join(root, 'bin', 'procoder.js');

function check(file) {
  try {
    execFileSync('node', [CLI, 'check', file], { cwd: root, encoding: 'utf8' });
    return '';
  } catch (e) {
    return String(e.stdout || '');
  }
}

test('there is one example per rung', () => {
  for (const rung of ['safe', 'true', 'obvious', 'alone']) {
    const dir = path.join(root, 'examples', rung);
    assert.ok(fs.existsSync(path.join(dir, 'before.ts')), `${rung}/before.ts missing`);
    assert.ok(fs.existsSync(path.join(dir, 'after.ts')), `${rung}/after.ts missing`);
  }
});

test('each before file trips its rung and each after file is clean', () => {
  const expected = { safe: 'SAFE', true: 'TRUE', obvious: 'OBVIOUS', alone: 'ALONE' };
  for (const [rung, label] of Object.entries(expected)) {
    const before = check(`examples/${rung}/before.ts`);
    assert.match(before, new RegExp(label), `examples/${rung}/before.ts does not trip ${label}`);
    assert.strictEqual(check(`examples/${rung}/after.ts`), '',
      `examples/${rung}/after.ts is not clean`);
  }
});

test('install docs cover every supported host', () => {
  const docs = fs.readFileSync(path.join(root, 'docs', 'install.md'), 'utf8');
  for (const host of ['Claude Code', 'Cursor', 'Windsurf', 'Cline', 'opencode', 'Kiro', 'MCP']) {
    assert.ok(docs.includes(host), `install docs missing ${host}`);
  }
});

test('README lists every command that exists', () => {
  const readme = fs.readFileSync(path.join(root, 'README.md'), 'utf8');
  for (const file of fs.readdirSync(path.join(root, 'commands'))) {
    const cmd = '/' + path.basename(file, '.toml');
    assert.ok(readme.includes(cmd), `README missing ${cmd}`);
  }
});
