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

// Claude Code derives a command's invocation from its commands/<name>.toml
// filename and a skill's from its skills/<name>/ directory name, prefixing both
// with the plugin name: /procoder:<name>. So the two sets must pair up — with
// exactly two exemptions, one on each side:
//
//   skills/procoder/  — the doctrine. Injected by the SessionStart hook
//     (hooks/procoder-instructions.js) and rendered into the platform rule
//     files by scripts/sync-rules.js. It is never invoked, so it has no
//     command, and its path is hardcoded in both of those files.
//   commands/level.toml — /procoder:level sets the persisted level. The
//     UserPromptSubmit hook has already done the work by the time the prompt
//     runs; there is no procedure to carry, so it has no skill.
//
// Anything else unpaired is a half-applied rename: a command that resolves to
// no skill, or a skill no command can reach.
const DOCTRINE_SKILL = 'procoder';
const SKILLLESS_COMMAND = 'level';

const skillNames = () => fs.readdirSync(path.join(root, 'skills'))
  .filter((s) => s !== DOCTRINE_SKILL).sort();
const commandNames = () => fs.readdirSync(path.join(root, 'commands'))
  .map((f) => path.basename(f, '.toml'))
  .filter((c) => c !== SKILLLESS_COMMAND).sort();

test('every skill has a matching command, and vice versa', () => {
  assert.deepStrictEqual(skillNames(), commandNames());
});

test('all twelve spec commands exist', () => {
  const commands = fs.readdirSync(path.join(root, 'commands'))
    .map((f) => path.basename(f, '.toml')).sort();
  assert.deepStrictEqual(commands, [
    'audit', 'debt', 'deps', 'gain', 'guard', 'help', 'level',
    'review', 'rot', 'statusline', 'threat', 'update',
  ]);
});

// The check that catches a half-applied rename. Renaming the files is not the
// same as the commands resolving: Claude Code matches a skill by its directory
// name, and the frontmatter `name:` has to agree with it, so a directory
// renamed without its frontmatter (or the reverse) leaves the command pointing
// at nothing. Asserted for the doctrine skill too — it is exempt from needing a
// command, not from having a name that matches its directory.
test('every command resolves to a skill whose frontmatter name matches its directory', () => {
  const frontmatterName = (dir) => {
    const file = path.join(root, 'skills', dir, 'SKILL.md');
    assert.ok(fs.existsSync(file), `skills/${dir}/SKILL.md is missing`);
    const m = /^---\n([\s\S]*?)\n---\n/.exec(fs.readFileSync(file, 'utf8'));
    assert.ok(m, `skills/${dir}/SKILL.md has no frontmatter`);
    const name = /^name:[ \t]*(\S+)[ \t]*$/m.exec(m[1]);
    assert.ok(name, `skills/${dir}/SKILL.md frontmatter declares no name`);
    return name[1];
  };

  for (const dir of [...commandNames(), DOCTRINE_SKILL]) {
    assert.strictEqual(frontmatterName(dir), dir,
      `skills/${dir}/SKILL.md frontmatter name does not match its directory — `
      + `/procoder:${dir} will not resolve`);
  }
});

// The manifests once said MIT while LICENSE said Apache 2.0, so anyone reading
// package.json would have believed the wrong licence and acted on it. Same
// drift class as the version mismatch below, which is why it gets the same
// guard: the LICENSE file is authoritative and the manifests must agree with it.
test('the declared licence agrees with the LICENSE file', () => {
  const license = fs.readFileSync(path.join(root, 'LICENSE'), 'utf8');
  const expected = /Apache License/.test(license) ? 'Apache-2.0'
    : /MIT License/.test(license) ? 'MIT'
      : null;
  assert.ok(expected, 'LICENSE is neither Apache 2.0 nor MIT — teach this test the new one');

  for (const manifest of ['package.json', 'procoder-mcp/package.json', '.claude-plugin/plugin.json']) {
    assert.strictEqual(readJson(manifest).license, expected,
      `${manifest} declares a licence the LICENSE file does not grant`);
  }
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
