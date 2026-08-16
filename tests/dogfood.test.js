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

function selfScan() {
  try {
    return { code: 0, out: execFileSync('node', [CLI, 'check', ...TARGETS], { cwd: root, encoding: 'utf8' }) };
  } catch (e) {
    return { code: e.status, out: String(e.stdout || '') };
  }
}

test('procoder reports no findings against its own source', () => {
  const { code, out } = selfScan();
  assert.strictEqual(code, 0,
    `procoder fails its own rungs:\n${out}\nFix the source, do not baseline it.`);
});

test('the self-scan is a real gate: a planted violation is reported', () => {
  const fs = require('fs');
  const planted = path.join(root, 'hooks', 'checks', 'dogfood-canary.js');
  fs.writeFileSync(planted, '// TODO: no owner, no ticket\nmodule.exports = {};\n');
  try {
    const { code, out } = selfScan();
    assert.strictEqual(code, 1, 'a planted orphan TODO must fail the self-scan');
    assert.match(out, /dogfood-canary\.js:1 TODO with no owner or ticket/);
  } finally {
    fs.unlinkSync(planted);
  }
});
