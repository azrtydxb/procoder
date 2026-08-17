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

// A manifest entry with no lockfile entry was hand-written, not installed:
// nothing resolved the version and nothing recorded the tree it pulls in, so
// what CI installs is not what anybody reviewed. It is also the shape an agent
// produces when it edits package.json directly instead of running the manager.
test('a dependency the lockfile has never heard of is reported', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"left-pad":"1.0.0","real":"1.0.0"}}',
    'package-lock.json': '{"packages":{"node_modules/real":{"version":"1.0.0"}}}',
  });
  const findings = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
  const unlocked = findings.filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unlocked.length, 1, 'expected exactly the unlocked dependency');
  assert.match(unlocked[0].message, /left-pad/);
});

// npm v1 keys the name, npm v2/v3 key a path, yarn and pnpm key name@range.
// A rule that only understood one of them would report every dependency in the
// other three as hand-written.
test('every lockfile spelling counts as locked', () => {
  for (const [file, content] of [
    ['package-lock.json', '{"dependencies":{"real":{"version":"1.0.0"}}}'],
    ['package-lock.json', '{"packages":{"node_modules/real":{"version":"1.0.0"}}}'],
    ['yarn.lock', 'real@^1.0.0:\n  version "1.0.0"\n'],
    ['pnpm-lock.yaml', 'packages:\n  /real@1.0.0:\n    resolution: {}\n'],
  ]) {
    const repo = repoWith({ 'package.json': '{"dependencies":{"real":"1.0.0"}}', [file]: content });
    const findings = checkManifest(path.join(repo, 'package.json'),
      fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
    assert.ok(!findings.some((f) => f.id === 'safe/manifest-not-locked'),
      `${file} spelling was not recognised as locking the dependency`);
  }
});

// The false positive that matters: a shorter name must not be answered by a
// longer one that contains it.
test('a name is not locked by another name that contains it', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"pad":"1.0.0"}}',
    'package-lock.json': '{"packages":{"node_modules/left-pad":{"version":"1.0.0"}}}',
  });
  const findings = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
  assert.ok(findings.some((f) => f.id === 'safe/manifest-not-locked'));
});

// No lockfile at all is safe/missing-lockfile's finding. Reporting every
// dependency as unlocked on top of it would be one cause, many findings.
test('a repo with no lockfile reports the missing lockfile, not every dependency', () => {
  const repo = repoWith({ 'package.json': '{"dependencies":{"a":"1.0.0","b":"1.0.0"}}' });
  const findings = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
  assert.ok(findings.some((f) => f.id === 'safe/missing-lockfile'));
  assert.ok(!findings.some((f) => f.id === 'safe/manifest-not-locked'));
});
