// tests/deps.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { detectEcosystems, checkManifest, AUDIT_COMMANDS } = require('../hooks/checks/deps');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-deps-'));
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('detects the npm ecosystem and its lockfile', () => {
  const repo = repoWith({ 'package.json': '{}', 'package-lock.json': '{}' });
  const found = detectEcosystems(repo);
  assert.strictEqual(found.length, 1);
  assert.strictEqual(found[0].name, 'npm');
  assert.strictEqual(found[0].hasLockfile, true);
});

test('detects several ecosystems in one repo', () => {
  const repo = repoWith({ 'package.json': '{}', 'pyproject.toml': '', 'go.mod': '' });
  assert.deepStrictEqual(
    detectEcosystems(repo).map((e) => e.name).sort(), ['go', 'npm', 'python']);
});

test('flags a missing lockfile', () => {
  const repo = repoWith({ 'package.json': '{"dependencies":{"a":"1.0.0"}}' });
  const ids = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8')).map((f) => f.id);
  assert.ok(ids.includes('safe/missing-lockfile'));
});

test('flags floating version ranges', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"a":"*","b":"latest","c":"^1.2.3","d":"1.2.3"}}',
    'package-lock.json': '{}',
  });
  const findings = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
  const messages = findings.map((f) => f.message).join(' ');
  assert.match(messages, /\ba\b/);
  assert.match(messages, /\bb\b/);
  assert.ok(!/"d"/.test(messages), 'a pinned version must not be flagged');
});

test('every ecosystem declares an audit command', () => {
  for (const name of ['npm', 'python', 'go', 'rust', 'dotnet']) {
    assert.ok(AUDIT_COMMANDS[name], `no audit command for ${name}`);
    assert.ok(Array.isArray(AUDIT_COMMANDS[name].argv));
  }
});
