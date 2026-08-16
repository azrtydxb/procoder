// procoder — tests/e2e.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execHook } = require('./hook-stdin');

const root = path.join(__dirname, '..');
const hook = (name) => path.join(root, 'hooks', name);

test('a full session lifecycle: activate, switch level, deactivate', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-e2e-'));
  const env = { ...process.env, CLAUDE_CONFIG_DIR: dir };
  const levelFile = path.join(dir, '.procoder-active');

  // execHook, not execFileSync's `input:`: procoder-activate.js never reads
  // stdin, so a piped payload races its exit — see tests/hook-stdin.js.
  const start = execHook(hook('procoder-activate.js'), { env });
  assert.match(start, /SAFE/);
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'strict');

  execFileSync('node', [hook('procoder-mode-tracker.js')],
    { encoding: 'utf8', input: JSON.stringify({ prompt: '/procoder paranoid' }), env });
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'paranoid');

  const badge = execFileSync('bash', [hook('procoder-statusline.sh')], { encoding: 'utf8', env });
  assert.strictEqual(badge.trim(), '[PROCODER:PARANOID]');

  execFileSync('node', [hook('procoder-mode-tracker.js')],
    { encoding: 'utf8', input: JSON.stringify({ prompt: 'stop procoder' }), env });
  // Deactivation persists the literal level 'off' rather than deleting the
  // file: readLevel() treats a missing file as "use the default", so
  // deleting it here would silently re-activate procoder at the default
  // level on the next read (e.g. from a subagent hook).
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'off');
  const offBadge = execFileSync('bash', [hook('procoder-statusline.sh')], { encoding: 'utf8', env });
  assert.strictEqual(offBadge.trim(), '');

  fs.rmSync(dir, { recursive: true, force: true });
});

test('README documents every level and command that exists', () => {
  const readme = fs.readFileSync(path.join(root, 'README.md'), 'utf8');
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    assert.match(readme, new RegExp(level), `README missing level: ${level}`);
  }
  for (const file of fs.readdirSync(path.join(root, 'commands'))) {
    // Claude Code exposes commands/<name>.toml as /procoder:<name>.
    const cmd = '/procoder:' + path.basename(file, '.toml');
    assert.ok(readme.includes(cmd), `README missing command: ${cmd}`);
  }
});
