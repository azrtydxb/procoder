// tests/resolve.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { hasTool, isConfigured, resolveFor, runTool, runToolResult } = require('../hooks/checks/resolve');
const { TOOLS } = require('../hooks/checks/registry');

// Tracked centrally and swept in one `after` hook, rather than a try/finally
// per test, so a test that adds a new tempRepo() call later can't forget it.
const tempDirs = [];
test.after(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true });
});

function tempRepo(files = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-res-'));
  tempDirs.push(dir);
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('hasTool finds node and does not find a nonsense binary', () => {
  assert.strictEqual(hasTool('node'), true);
  assert.strictEqual(hasTool('procoder-definitely-not-a-real-binary'), false);
});

test('isConfigured requires one of the tool config files', () => {
  assert.strictEqual(isConfigured(tempRepo({ '.eslintrc.json': '{}' }), TOOLS.ts), true);
  assert.strictEqual(isConfigured(tempRepo({ 'ruff.toml': '' }), TOOLS.py), true);
  assert.strictEqual(isConfigured(tempRepo(), TOOLS.ts), false);
});

test('a shared manifest is not evidence unless it names the tool', () => {
  assert.strictEqual(isConfigured(tempRepo({ 'pyproject.toml': '[project]\nname="x"\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'pyproject.toml': '[tool.ruff]\n' }), TOOLS.py), true);
  assert.strictEqual(isConfigured(tempRepo({ 'setup.cfg': '[metadata]\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'Cargo.toml': '[package]\nname="x"\n' }), TOOLS.rust), false);
  assert.strictEqual(isConfigured(tempRepo({ 'Cargo.toml': '[lints.clippy]\n' }), TOOLS.rust), true);
  assert.strictEqual(isConfigured(tempRepo({ 'clippy.toml': '' }), TOOLS.rust), true);
});

test('resolveFor yields null when the tool is unconfigured', () => {
  assert.strictEqual(resolveFor('a.py', { repoRoot: tempRepo() }), null);
});

test('resolveFor yields null for a file type with no tool', () => {
  assert.strictEqual(resolveFor('a.cs', { repoRoot: tempRepo({ '.eslintrc': '{}' }) }), null);
});

test('runTool returns an empty array when the binary is missing', () => {
  const fake = { name: 'procoder-missing-binary', argv: () => ['x'], parse: () => [{}] };
  assert.deepStrictEqual(runTool(fake, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 500 }), []);
});

test('runTool honours the timeout and returns an empty array', () => {
  const slow = { name: 'node', argv: () => ['-e', 'setTimeout(()=>{}, 10000)'], parse: () => [{}] };
  const started = Date.now();
  const out = runTool(slow, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 400 });
  assert.deepStrictEqual(out, []);
  assert.ok(Date.now() - started < 3000, 'runTool did not abandon the slow process');
});

test('runToolResult reports an abnormal exit as not ok', () => {
  const missing = { name: 'procoder-missing-binary', argv: () => ['x'], parse: () => [] };
  assert.strictEqual(runToolResult(missing, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 500 }).ok, false);

  const slow = { name: 'node', argv: () => ['-e', 'setTimeout(()=>{}, 10000)'], parse: () => [] };
  assert.strictEqual(runToolResult(slow, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 400 }).ok, false);
});

test('runToolResult reports a clean exit with no output as ok', () => {
  const clean = { name: 'node', argv: () => ['-e', 'process.stdout.write("[]")'], parse: () => [] };
  const out = runToolResult(clean, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.deepStrictEqual(out, { findings: [], ok: true });
});

test('runTool parses stdout through the tool parser', () => {
  const echo = {
    name: 'node',
    argv: () => ['-e', 'process.stdout.write(JSON.stringify([{filename:"a.py",location:{row:3},code:"E1",message:"boom"}]))'],
    parse: TOOLS.py.parse,
  };
  const out = runTool(echo, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.strictEqual(out.length, 1);
  assert.strictEqual(out[0].line, 3);
});
