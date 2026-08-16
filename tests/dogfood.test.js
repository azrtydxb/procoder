// tests/dogfood.test.js
//
// procoder run against procoder. A tool that exempts itself from its own
// doctrine has already lost the argument, so this is a hard gate: when it
// fails, fix the source. Do not baseline it, and do not widen an exclusion.
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const path = require('path');

const root = path.join(__dirname, '..');
const CLI = path.join(root, 'bin', 'procoder.js');
const TARGETS = ['hooks', 'bin', 'scripts'];

function selfScan(extraTargets = []) {
  try {
    return {
      code: 0,
      out: execFileSync('node', [CLI, 'check', ...TARGETS, ...extraTargets], { cwd: root, encoding: 'utf8' }),
    };
  } catch (e) {
    return { code: e.status, out: String(e.stdout || '') };
  }
}

test('procoder reports no findings against its own source', () => {
  const { code, out } = selfScan();
  assert.strictEqual(code, 0,
    `procoder fails its own rungs:\n${out}\nFix the source, do not baseline it.`);
});

// The canary must prove the self-scan actually fails on a planted violation,
// without ever landing inside the tracked tree — an interrupted run must not
// be able to leave a stray file for `git add` to pick up. It's written under
// the OS temp dir instead and passed to `procoder check` as an extra target
// (checkFile accepts any path, not just ones under the repo), so the scanner
// genuinely evaluates it — this is not a weakened test, just a relocated file.
test('the self-scan is a real gate: a planted violation is reported', () => {
  const fs = require('fs');
  const os = require('os');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-dogfood-'));
  const planted = path.join(dir, 'dogfood-canary.js');
  fs.writeFileSync(planted, '// TODO: no owner, no ticket\nmodule.exports = {};\n');
  try {
    const { code, out } = selfScan([planted]);
    assert.strictEqual(code, 1, 'a planted orphan TODO must fail the self-scan');
    assert.match(out, /dogfood-canary\.js:1 TODO with no owner or ticket/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
