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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    const stdout = execFileSync('node', [script], {
      encoding: 'utf8',
      input: '{}',
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir, ...env },
    });
    return { stdout, dir, levelFile: path.join(dir, '.procoder-active') };
  } finally {
    // dir is inspected by the caller before this runs only for sync assertions,
    // so clean up lazily via the OS temp reaper instead of removing it here.
  }
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

test('hooks exit 0 even when the config dir is unwritable', () => {
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: '{}',
    env: { ...process.env, CLAUDE_CONFIG_DIR: '/proc/nope-procoder' },
  }));
});
