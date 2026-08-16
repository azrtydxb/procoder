// procoder — release readiness: skill/command parity, version alignment
// across every manifest, packaged directories, and a changelog naming the
// shipped version.
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const { TARGETS, renderCommands } = require('../scripts/sync-rules');

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

test('the MCP server reports the version from its own manifest, not a hardcoded literal', () => {
  const server = fs.readFileSync(path.join(root, 'procoder-mcp/server.js'), 'utf8');
  assert.ok(!/version:\s*['"]\d+\.\d+\.\d+['"]/.test(server),
    'server.js hardcodes a version literal instead of reading procoder-mcp/package.json');
});

test('the package ships every directory and manifest a host needs', () => {
  const files = readJson('package.json').files;

  // A generated path is "shipped" if its top-level segment (a directory for
  // nested paths, the file itself for a bare one like AGENTS.md) is listed.
  const shipsPath = (rel) => {
    const top = rel.split('/')[0];
    return rel.includes('/')
      ? files.some((f) => f === top || f === `${top}/`)
      : files.includes(rel);
  };

  for (const dir of ['hooks/', 'skills/', 'commands/', 'bin/', 'procoder-mcp/']) {
    assert.ok(shipsPath(`${dir.replace(/\/$/, '')}/x`), `package.json files missing ${dir}`);
  }

  // Derived from scripts/sync-rules.js's own target lists — every platform it
  // renders a doctrine or command file for must ship, so a platform added
  // there later cannot be silently left out of the npm package.
  const generatedPaths = [...TARGETS.map((t) => t.path), ...renderCommands().keys()];
  for (const rel of generatedPaths) {
    assert.ok(shipsPath(rel), `package.json files missing the directory for generated file ${rel}`);
  }

  // Static manifests docs/install.md tells users to copy from directly.
  for (const manifest of ['gemini-extension.json', 'opencode.json']) {
    assert.ok(files.includes(manifest), `package.json files missing manifest ${manifest}`);
  }
});

test('the changelog names the current version', () => {
  const changelog = fs.readFileSync(path.join(root, 'CHANGELOG.md'), 'utf8');
  assert.ok(changelog.includes(readJson('package.json').version));
});
