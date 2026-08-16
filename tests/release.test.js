// procoder — release readiness: skill/command parity, version alignment
// across every manifest, packaged directories, and a changelog naming the
// shipped version.
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');
const readJson = (p) => JSON.parse(fs.readFileSync(path.join(root, p), 'utf8'));

test('every skill has a matching command, and vice versa', () => {
  const skills = fs.readdirSync(path.join(root, 'skills'))
    .filter((s) => s !== 'procoder');
  const commands = fs.readdirSync(path.join(root, 'commands'))
    .map((f) => path.basename(f, '.toml'))
    .filter((c) => c !== 'procoder');
  assert.deepStrictEqual(skills.sort(), commands.sort());
});

test('all ten spec commands exist', () => {
  const commands = fs.readdirSync(path.join(root, 'commands'))
    .map((f) => path.basename(f, '.toml')).sort();
  assert.deepStrictEqual(commands, [
    'procoder', 'procoder-audit', 'procoder-debt', 'procoder-deps',
    'procoder-gain', 'procoder-guard', 'procoder-help', 'procoder-review',
    'procoder-rot', 'procoder-threat',
  ]);
});

test('versions agree across every manifest', () => {
  const version = readJson('package.json').version;
  assert.strictEqual(readJson('.claude-plugin/plugin.json').version, version);
  assert.strictEqual(readJson('procoder-mcp/package.json').version, version);
  assert.strictEqual(readJson('gemini-extension.json').version, version);
});

test('the package ships every directory a host needs', () => {
  const files = readJson('package.json').files;
  for (const dir of ['hooks/', 'skills/', 'commands/', 'bin/', 'procoder-mcp/']) {
    assert.ok(files.some((f) => f === dir || f === dir.replace(/\/$/, '')),
      `package.json files missing ${dir}`);
  }
});

test('the changelog names the current version', () => {
  const changelog = fs.readFileSync(path.join(root, 'CHANGELOG.md'), 'utf8');
  assert.ok(changelog.includes(readJson('package.json').version));
});
