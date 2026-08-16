// tests/activate.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-activate.js');
const SUBAGENT = path.join(__dirname, '..', 'hooks', 'procoder-subagent.js');
const MODE_TRACKER = path.join(__dirname, '..', 'hooks', 'procoder-mode-tracker.js');

function run(script, env = {}) {
  // The caller inspects dir after this returns, so cleanup is left to the OS
  // temp reaper.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const stdout = execFileSync('node', [script], {
    encoding: 'utf8',
    input: '{}',
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir, ...env },
  });
  return { stdout, dir, levelFile: path.join(dir, '.procoder-active') };
}

test('activate emits the doctrine and persists the level', () => {
  const { stdout, levelFile } = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'strict' });
  assert.match(stdout, /SAFE/);
  assert.match(stdout, /ALONE/);
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'strict');
});

test('paranoid emits strictly more than pragmatic', () => {
  const lean = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'pragmatic' }).stdout;
  const full = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'paranoid' }).stdout;
  assert.ok(full.length > lean.length);
});

test('off emits nothing and writes no level file', () => {
  const { stdout, levelFile } = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'off' });
  assert.ok(!/SAFE/.test(stdout));
  assert.ok(!fs.existsSync(levelFile));
});

test('PROCODER_NO_HOOK disables activation entirely', () => {
  const { stdout } = run(HOOK, { PROCODER_NO_HOOK: '1' });
  assert.ok(!/SAFE/.test(stdout));
});

test('subagent hook wraps context in hookSpecificOutput', () => {
  const { stdout } = run(SUBAGENT, { PROCODER_DEFAULT_LEVEL: 'strict' });
  const parsed = JSON.parse(stdout);
  assert.strictEqual(parsed.hookSpecificOutput.hookEventName, 'SubagentStart');
  assert.match(parsed.hookSpecificOutput.additionalContext, /SAFE/);
});

test('a persisted "off" level suppresses subagent doctrine injection', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  fs.writeFileSync(path.join(dir, '.procoder-active'), 'off\n');
  const stdout = execFileSync('node', [SUBAGENT], {
    encoding: 'utf8',
    input: '{}',
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
  });
  assert.strictEqual(stdout, '');
});

test('saying "stop procoder" then launching a subagent injects no doctrine', () => {
  // End-to-end: the mode tracker (Task 7) must persist the deactivation as
  // the literal level 'off', not delete the level file, or the subagent
  // hook's `readLevel()` falls back to the default and re-injects the
  // doctrine right after the user turned procoder off.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const env = { ...process.env, CLAUDE_CONFIG_DIR: dir };

  execFileSync('node', [MODE_TRACKER], {
    encoding: 'utf8',
    input: JSON.stringify({ prompt: 'stop procoder' }),
    env,
  });

  const stdout = execFileSync('node', [SUBAGENT], {
    encoding: 'utf8',
    input: '{}',
    env,
  });
  assert.strictEqual(stdout, '');
});

test('deactivation survives a session restart, for the session and its subagents', () => {
  // The session-start hook must not treat a persisted 'off' the way it treats
  // PROCODER_DEFAULT_LEVEL=off: deleting the level file there would make the
  // NEXT session read a missing file, fall back to the default, and re-activate.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    const env = { ...process.env, CLAUDE_CONFIG_DIR: dir };
    const levelFile = path.join(dir, '.procoder-active');

    execFileSync('node', [MODE_TRACKER],
      { encoding: 'utf8', input: JSON.stringify({ prompt: 'stop procoder' }), env });
    assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'off');

    // Session 2 starts.
    const start = execFileSync('node', [HOOK], { encoding: 'utf8', input: '{}', env });
    assert.ok(!/SAFE/.test(start), 'doctrine emitted after deactivation');
    assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'off',
      'session start erased the persisted deactivation');

    // A subagent launched in session 2 must stay silent too.
    const sub = execFileSync('node', [SUBAGENT], { encoding: 'utf8', input: '{}', env });
    assert.strictEqual(sub, '');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('PROCODER_DEFAULT_LEVEL=off clears a stale persisted level', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    const levelFile = path.join(dir, '.procoder-active');
    fs.writeFileSync(levelFile, 'paranoid\n');
    execFileSync('node', [HOOK], {
      encoding: 'utf8',
      input: '{}',
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir, PROCODER_DEFAULT_LEVEL: 'off' },
    });
    assert.ok(!fs.existsSync(levelFile), 'env-level off left a stale persisted level behind');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('hooks exit 0 even when the config dir is unwritable', () => {
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: '{}',
    env: { ...process.env, CLAUDE_CONFIG_DIR: '/proc/nope-procoder' },
  }));
});
