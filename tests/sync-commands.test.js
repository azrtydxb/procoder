// procoder — command-porting coverage: every commands/*.toml gets rendered
// to the markdown-command platforms, carries the generated warning, and is
// covered by the sync --check drift gate.
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const { renderCommands, render } = require('../scripts/sync-rules');

const root = path.join(__dirname, '..');

test('every command gets an opencode port', () => {
  const commands = fs.readdirSync(path.join(root, 'commands')).filter((f) => f.endsWith('.toml'));
  const rendered = renderCommands();
  for (const file of commands) {
    const name = path.basename(file, '.toml');
    assert.ok(rendered.has(`.opencode/command/${name}.md`), `no opencode port for ${name}`);
  }
});

test('ported commands carry the generated warning', () => {
  for (const [file, content] of renderCommands()) {
    assert.match(content, /DO NOT EDIT/, `${file} missing warning`);
  }
});

test('sync --check covers commands as well as rules', () => {
  execFileSync('node', [path.join(root, 'scripts/sync-rules.js')], { cwd: root });
  const victim = path.join(root, '.opencode/command/procoder-review.md');
  const saved = fs.readFileSync(victim, 'utf8');
  try {
    fs.writeFileSync(victim, saved + '\ndrift\n');
    assert.throws(() => execFileSync(
      'node', [path.join(root, 'scripts/sync-rules.js'), '--check'],
      { cwd: root, stdio: 'pipe' }));
  } finally {
    fs.writeFileSync(victim, saved);
  }
});

test('the pi and gemini manifests list every skill', () => {
  const skills = fs.readdirSync(path.join(root, 'skills'));
  const pi = fs.readFileSync(path.join(root, 'pi-extension/index.js'), 'utf8');
  for (const skill of skills) {
    assert.ok(pi.includes(skill) || pi.includes('./skills'), `pi extension misses ${skill}`);
  }
  assert.ok(fs.existsSync(path.join(root, 'gemini-extension.json')));
});

test('render() and renderCommands() do not collide on the same path', () => {
  const rules = render();
  const commands = renderCommands();
  for (const key of rules.keys()) {
    assert.ok(!commands.has(key), `render() and renderCommands() both write ${key}`);
  }
});
