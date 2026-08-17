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

const idsFor = (repo, rel) => checkManifest(path.join(repo, rel),
  fs.readFileSync(path.join(repo, rel), 'utf8'));

// A parse failure that reduces coverage while reporting nothing wrong is the
// worst outcome a check has: the manifest reads as clean because nothing was
// read at all. It must say so.
test('a manifest the parser cannot read is reported, not silently skipped', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"left-pad":"1.0.0",',
    'package-lock.json': '{}',
  });
  const findings = idsFor(repo, 'package.json');
  const unread = findings.filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unread.length, 1, 'a truncated manifest was silent');
  assert.match(unread[0].message, /could not be parsed/);
});

// The false positive the limitations audit named: a peer dependency is by
// definition the consumer's to install, and is legitimately absent here.
test('a peer dependency the lockfile does not carry is not a finding', () => {
  const repo = repoWith({
    'package.json': '{"peerDependencies":{"react":"^18.0.0"},"dependencies":{"real":"1.0.0"}}',
    'package-lock.json': '{"packages":{"node_modules/real":{"version":"1.0.0"}}}',
  });
  assert.ok(!idsFor(repo, 'package.json').some((f) => f.id === 'safe/manifest-not-locked'),
    'a peer dependency was reported as unlocked');
});

// optionalDependencies ARE installed by this package, so an entry the lockfile
// has never heard of was hand-written just like a normal one.
test('an optional dependency the lockfile has never heard of is reported', () => {
  const repo = repoWith({
    'package.json': '{"optionalDependencies":{"fsevents":"2.3.3"}}',
    'package-lock.json': '{"packages":{"":{}}}',
  });
  const unlocked = idsFor(repo, 'package.json').filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unlocked.length, 1);
  assert.match(unlocked[0].message, /fsevents/);
});

// A monorepo keeps one lockfile, at the root. Looking only next to the
// manifest reported every workspace package as having no lockfile at all, and
// never checked a single entry.
test('a workspace package is checked against the lockfile at the repository root', () => {
  const repo = repoWith({
    '.git/HEAD': 'ref: refs/heads/main\n',
    'package.json': '{"private":true,"workspaces":["packages/*"]}',
    'package-lock.json': '{"packages":{"node_modules/real":{"version":"1.0.0"}}}',
    'packages/a/package.json': '{"dependencies":{"real":"1.0.0","left-pad":"1.0.0"}}',
  });
  const findings = idsFor(repo, 'packages/a/package.json');
  assert.ok(!findings.some((f) => f.id === 'safe/missing-lockfile'),
    'the root lockfile was not found from a workspace package');
  const unlocked = findings.filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unlocked.length, 1);
  assert.match(unlocked[0].message, /left-pad/);
});

// A lockfile is a resolution, not a bag of names: a direct dependency that
// appears only inside somebody else's subtree was never installed as a direct
// edge, and what CI resolves for it is nobody's decision.
test('a direct dependency the lock knows only as a transitive is reported', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"ms":"2.1.2","debug":"4.3.4"}}',
    'package-lock.json':
      '{"dependencies":{"debug":{"version":"4.3.4","dependencies":{"ms":{"version":"2.1.2"}}}}}',
  });
  const unlocked = idsFor(repo, 'package.json').filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unlocked.length, 1, `reported: ${unlocked.map((f) => f.message).join('; ')}`);
  assert.match(unlocked[0].message, /\bms\b/);
});

// Go: go.sum records every module the build resolves, so a require line with
// no go.sum entry is a hand-edited go.mod.
test('a go.mod requirement missing from go.sum is reported', () => {
  const repo = repoWith({
    'go.mod': 'module example.com/x\n\ngo 1.22\n\nrequire (\n\tgithub.com/pkg/errors v0.9.1\n\tgithub.com/spf13/cobra v1.8.0\n)\n',
    'go.sum': 'github.com/pkg/errors v0.9.1 h1:abc=\ngithub.com/pkg/errors v0.9.1/go.mod h1:def=\n',
  });
  const unlocked = idsFor(repo, 'go.mod').filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unlocked.length, 1, `reported: ${unlocked.map((f) => f.message).join('; ')}`);
  assert.match(unlocked[0].message, /cobra/);
});

// A module replaced by a local directory has no go.sum entry by construction.
test('a replaced module is not expected in go.sum', () => {
  const repo = repoWith({
    'go.mod': 'module example.com/x\n\nrequire github.com/pkg/errors v0.9.1\n\nreplace github.com/pkg/errors => ../errors\n',
    'go.sum': '\n',
  });
  assert.ok(!idsFor(repo, 'go.mod').some((f) => f.id === 'safe/manifest-not-locked'),
    'a locally replaced module was reported as unlocked');
});

// Cargo: Cargo.lock names every crate in the graph, direct or not.
test('a Cargo.toml dependency missing from Cargo.lock is reported', () => {
  const repo = repoWith({
    'Cargo.toml': '[package]\nname = "x"\nversion = "0.1.0"\n\n[dependencies]\nserde = "1.0"\nrand = { version = "0.8", features = ["std"] }\n',
    'Cargo.lock': '[[package]]\nname = "x"\nversion = "0.1.0"\n\n[[package]]\nname = "serde"\nversion = "1.0.197"\n',
  });
  const unlocked = idsFor(repo, 'Cargo.toml').filter((f) => f.id === 'safe/manifest-not-locked');
  assert.strictEqual(unlocked.length, 1, `reported: ${unlocked.map((f) => f.message).join('; ')}`);
  assert.match(unlocked[0].message, /rand/);
});

// What the corpus said about walking upward. A lockfile above the manifest
// governs it only where the directory holding it declares a workspace, and
// never across a vendoring boundary: measured over 3,390 third-party
// manifests, those two tests are the difference between 1 finding and 5,961.
test('a project nested under an unrelated one is not measured against its lockfile', () => {
  const repo = repoWith({
    '.git/HEAD': 'ref: refs/heads/main\n',
    'package.json': '{"dependencies":{"real":"1.0.0"}}',
    'package-lock.json': '{"packages":{"node_modules/real":{"version":"1.0.0"}}}',
    'examples/greeter/package.json': '{"dependencies":{"@grpc/grpc-js":"1.9.0"}}',
  });
  const findings = idsFor(repo, 'examples/greeter/package.json');
  assert.ok(!findings.some((f) => f.id === 'safe/manifest-not-locked'),
    "a nested project's dependencies were checked against a lockfile that does not govern them");
});

test('a vendored package is not measured against the application lockfile', () => {
  const repo = repoWith({
    '.git/HEAD': 'ref: refs/heads/main\n',
    'package.json': '{"private":true,"workspaces":["packages/*"]}',
    'package-lock.json': '{"packages":{"node_modules/left-pad":{"version":"1.0.0"}}}',
    'node_modules/left-pad/package.json': '{"dependencies":{"its-own-dep":"1.0.0"}}',
  });
  assert.ok(!idsFor(repo, 'node_modules/left-pad/package.json')
    .some((f) => f.id === 'safe/manifest-not-locked'),
  "a vendored package's own dependencies were checked against the application's lockfile");
});

// pnpm v9 indents its package keys, so an anchor that only accepted a line
// start or a quote answered "not locked" for every dependency of every pnpm
// workspace.
test('an indented pnpm lock entry counts as locked', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"zod":"3.23.8"}}',
    'pnpm-lock.yaml': 'packages:\n\n  zod@3.23.8:\n    resolution: {integrity: sha512-x}\n',
  });
  assert.ok(!idsFor(repo, 'package.json').some((f) => f.id === 'safe/manifest-not-locked'),
    'an indented pnpm key was not recognised as locking the dependency');
});

// A renamed dependency is locked under the crate it renames, never under the
// name this manifest gave it.
test('a renamed Cargo dependency is looked up under the crate it renames', () => {
  const repo = repoWith({
    'Cargo.toml': '[package]\nname = "x"\n\n[dependencies]\njson = { package = "serde_json", version = "1.0" }\n',
    'Cargo.lock': '[[package]]\nname = "serde_json"\nversion = "1.0.100"\n',
  });
  assert.ok(!idsFor(repo, 'Cargo.toml').some((f) => f.id === 'safe/manifest-not-locked'),
    'a renamed dependency was reported under its local name');
});

// The table form of the same rename, which is how `alloc`, `core` and
// `libzstd` are declared across the crates.io corpus — 217 findings until it
// was read.
test('a rename written as its own dependency table is followed', () => {
  const repo = repoWith({
    'Cargo.toml': '[package]\nname = "x"\n\n[dependencies.alloc]\nversion = "1.0.0"\noptional = true\npackage = "rustc-std-workspace-alloc"\n',
    'Cargo.lock': '[[package]]\nname = "rustc-std-workspace-alloc"\nversion = "1.0.0"\n',
  });
  assert.ok(!idsFor(repo, 'Cargo.toml').some((f) => f.id === 'safe/manifest-not-locked'),
    'a dependency table rename was reported under its local name');
});

// Nothing may throw into a hook: a manifest that is empty, truncated, or not
// the format its name implies has to come back as findings or as nothing.
test('a manifest that is truncated, empty or the wrong format never throws', () => {
  for (const [name, body] of [
    ['package.json', ''],
    ['package.json', '{"dependencies":'],
    ['go.mod', ''],
    ['go.mod', 'require (\n\tgithub.com/a/b v1.0.0\n'],
    ['Cargo.toml', '[dependencies\nserde = "1.0"'],
    ['Cargo.toml', ''],
  ]) {
    const repo = repoWith({ [name]: body, 'package-lock.json': '{}', 'go.sum': '', 'Cargo.lock': '' });
    assert.doesNotThrow(() => checkManifest(path.join(repo, name), body), `${name}: ${JSON.stringify(body)}`);
  }
});
