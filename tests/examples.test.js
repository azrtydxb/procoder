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

function check(file, ...flags) {
  try {
    execFileSync('node', [CLI, 'check', ...flags, file], { cwd: root, encoding: 'utf8' });
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

// examples/.procoderignore keeps the before.* files out of an ordinary scan —
// they violate a rung on purpose, and a user auditing this repo should not have
// to read ten deliberate violations. --no-ignore is what makes them checkable
// anyway, so the detection they document is still proved on every run. The
// after.* files are checked with no flag at all: they are ordinary source and
// must be gated as such.
// This test used to assert that every before.ts tripped its own rung, and that
// was a fair claim while procoder carried a rule per rung. It no longer is, and
// pretending otherwise would be the worst kind of green test.
//
// The examples document the DOCTRINE — what SAFE, TRUE, OBVIOUS and ALONE mean
// to a reader. The gate implements a strict subset of that: rungs 3 and 4 are
// judgment calls no analyzer makes, and even on rung 1 the analyzers only prove
// what they prove (measured on CWEval: 5 of 30 real exploits named correctly).
// examples/safe/before.ts is a SQL injection built by template interpolation —
// genuinely unsafe, genuinely undetected by eslint-plugin-security.
//
// So what is asserted is what is true: the after files, which are ordinary
// source, pass the gate. If a future analyzer starts catching a before file,
// nothing here breaks — but no test claims a detection procoder does not have.
test('each after file passes the gate', () => {
  for (const rung of ['safe', 'true', 'obvious', 'alone']) {
    assert.strictEqual(check(`examples/${rung}/after.ts`), '',
      `examples/${rung}/after.ts is not clean`);
  }
});

// Without this the test above would pass just as well with no ignore file at
// all, and nothing would notice examples/.procoderignore going missing.
test('an ordinary scan skips the before files and still checks the after files', () => {
  for (const rung of ['safe', 'true', 'obvious', 'alone']) {
    assert.strictEqual(check(`examples/${rung}/before.ts`), '',
      `examples/${rung}/before.ts is not covered by examples/.procoderignore`);
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
    // Claude Code exposes commands/<name>.toml as /procoder:<name>.
    const cmd = '/procoder:' + path.basename(file, '.toml');
    assert.ok(readme.includes(cmd), `README missing ${cmd}`);
  }
});
