// tests/mode-tracker.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-mode-tracker.js');

function run(prompt) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const stdout = execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: JSON.stringify({ prompt }),
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
  });
  const levelFile = path.join(dir, '.procoder-active');
  return { stdout, level: fs.existsSync(levelFile) ? fs.readFileSync(levelFile, 'utf8').trim() : null };
}

test('/procoder paranoid switches the level', () => {
  assert.strictEqual(run('/procoder paranoid').level, 'paranoid');
});

test('/procoder with no argument leaves the level alone', () => {
  assert.strictEqual(run('/procoder').level, null);
});

test('"stop procoder" clears the level', () => {
  assert.strictEqual(run('stop procoder').level, null);
});

test('an ordinary prompt mentioning the phrase does not deactivate', () => {
  assert.strictEqual(run('add a normal mode toggle to the settings page').level, null);
});

test('malformed stdin does not crash the hook', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: 'not json at all',
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
  }));
});
