// procoder — command-porting coverage: every commands/*.toml gets rendered
// to the markdown-command platforms, carries the generated warning, and is
// covered by the sync --check drift gate.
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
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

// sync-rules.js resolves its output root from __dirname, not cwd, so running
// the tracked copy writes into the tracked tree — which would repair the very
// drift `npm run sync:check` exists to catch, and would hand every other test
// file a working tree that mutates under it (this test deliberately corrupts a
// tracked file and restores it in a finally, a window any concurrently running
// file could read, and a crash would leave dirty). Copying first is what
// tests/sync-rules.test.js already does, and for the same reason.
test('sync --check covers commands as well as rules', () => {
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-sync-cmd-'));
  try {
    for (const dir of ['scripts', 'hooks', 'skills', 'commands']) {
      fs.cpSync(path.join(root, dir), path.join(scratch, dir), { recursive: true });
    }
    const cli = path.join(scratch, 'scripts/sync-rules.js');
    execFileSync('node', [cli]);

    const victim = path.join(scratch, '.opencode/command/review.md');
    assert.ok(fs.existsSync(victim), 'sync did not render the command port under test');
    fs.appendFileSync(victim, '\ndrift\n');
    assert.throws(() => execFileSync('node', [cli, '--check'], { stdio: 'pipe' }));
  } finally {
    fs.rmSync(scratch, { recursive: true, force: true });
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
