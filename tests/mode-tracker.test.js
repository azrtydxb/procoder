// tests/mode-tracker.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-mode-tracker.js');

function run(prompt, seedLevel, env = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    const levelFile = path.join(dir, '.procoder-active');
    if (seedLevel) fs.writeFileSync(levelFile, seedLevel + '\n');
    const stdout = execFileSync('node', [HOOK], {
      encoding: 'utf8',
      input: JSON.stringify({ prompt }),
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir, ...env },
    });
    return {
      stdout,
      level: fs.existsSync(levelFile) ? fs.readFileSync(levelFile, 'utf8').trim() : null,
    };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test('/procoder paranoid switches the level', () => {
  assert.strictEqual(run('/procoder paranoid').level, 'paranoid');
});

test('/procoder with no argument leaves the level alone', () => {
  assert.strictEqual(run('/procoder').level, null);
});

test('"stop procoder" persists the level as off', () => {
  assert.strictEqual(run('stop procoder').level, 'off');
});

test('an ordinary prompt mentioning the phrase does not deactivate', () => {
  // Seed a known level first: an empty level file is indistinguishable
  // between "correctly left alone" and "wrongly deleted", so this must
  // start from a non-empty, known state to be able to fail.
  assert.strictEqual(
    run('add a normal mode toggle to the settings page', 'paranoid').level,
    'paranoid',
  );
});

test('an ordinary prompt reports the level that is actually active', () => {
  // Codex renders the hook's level in its UI, so a hardcoded 'strict' here
  // would contradict the statusline for every non-strict user.
  const { stdout } = run('add a login form', 'paranoid', { PROCODER_HOST: 'codex' });
  assert.strictEqual(JSON.parse(stdout).systemMessage, 'PROCODER:PARANOID');
});

test('malformed stdin does not crash the hook', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    assert.doesNotThrow(() => execFileSync('node', [HOOK], {
      encoding: 'utf8',
      input: 'not json at all',
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
    }));
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
