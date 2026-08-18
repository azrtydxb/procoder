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

// End to end through the hook: the parser unit test is not enough, because the
// break was that the whole chain no-opped while the command looked like it worked.
test('/procoder:level writes the new level to the level file', () => {
  assert.strictEqual(run('/procoder:level paranoid', 'strict').level, 'paranoid');
  assert.strictEqual(run('/procoder:level pragmatic', 'strict').level, 'pragmatic');
  assert.strictEqual(run('/procoder:level off', 'strict').level, 'off');
});

test('/procoder:level mentioned in prose leaves the level file alone', () => {
  assert.strictEqual(
    run('should /procoder:level paranoid be documented in the README?', 'strict').level,
    'strict',
  );
});

test('/procoder:level with no argument leaves the level alone', () => {
  assert.strictEqual(run('/procoder:level', 'strict').level, 'strict');
  assert.strictEqual(run('/procoder:level bogus', 'strict').level, 'strict');
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

// The per-turn rung-1 reminder: present on an ordinary prompt when procoder is
// active, absent when it is off, and never a second copy of the doctrine text —
// it is extracted from SKILL.md so the two cannot drift apart.
test('an ordinary prompt carries the rung-1 imperative while active', () => {
  const out = run('add a login handler', 'strict');
  const ctx = JSON.stringify(out);
  assert.match(ctx, /must be secure/,
    'active session should carry the imperative next to the prompt');
});

test('an off session carries no imperative', () => {
  const out = run('add a login handler', 'off');
  assert.doesNotMatch(JSON.stringify(out), /must be secure/,
    'procoder off must stay silent');
});

test('the imperative is extracted from the doctrine, not duplicated', () => {
  const { getSafeFirstImperative } = require('../hooks/procoder-instructions');
  const doctrine = fs.readFileSync(
    path.join(__dirname, '..', 'skills', 'procoder', 'SKILL.md'), 'utf8');
  const imperative = getSafeFirstImperative();
  assert.ok(imperative.length > 0, 'imperative should not be empty');
  assert.ok(doctrine.includes(imperative),
    'the hook text must come from SKILL.md verbatim, or the two will drift');
});
