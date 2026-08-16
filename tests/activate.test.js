// tests/activate.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
// Neither procoder-activate.js nor procoder-subagent.js reads stdin, so every
// call below hands them a file-backed one rather than a piped `input:` the
// parent has to win a race to write. See tests/hook-stdin.js.
const { execHook } = require('./hook-stdin');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-activate.js');
const SUBAGENT = path.join(__dirname, '..', 'hooks', 'procoder-subagent.js');
const MODE_TRACKER = path.join(__dirname, '..', 'hooks', 'procoder-mode-tracker.js');

function run(script, env = {}) {
  // The caller inspects dir after this returns, so cleanup is left to the OS
  // temp reaper.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const stdout = execHook(script, {
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
  const { stdout, levelFile } = run(HOOK, { PROCODER_NO_HOOK: '1' });
  assert.ok(!/SAFE/.test(stdout));
  assert.ok(!fs.existsSync(levelFile), 'PROCODER_NO_HOOK must not write a level file');
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
  const stdout = execHook(SUBAGENT, {
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

  const stdout = execHook(SUBAGENT, { env });
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
    const start = execHook(HOOK, { env });
    assert.ok(!/SAFE/.test(start), 'doctrine emitted after deactivation');
    assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'off',
      'session start erased the persisted deactivation');

    // A subagent launched in session 2 must stay silent too.
    const sub = execHook(SUBAGENT, { env });
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
    execHook(HOOK, {
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir, PROCODER_DEFAULT_LEVEL: 'off' },
    });
    assert.ok(!fs.existsSync(levelFile), 'env-level off left a stale persisted level behind');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('hooks exit 0 even when the config dir is unwritable', () => {
  assert.doesNotThrow(() => execHook(HOOK, {
    env: { ...process.env, CLAUDE_CONFIG_DIR: '/proc/nope-procoder' },
  }));
});

// ---------------------------------------------------------------------------
// Update notice. Nothing below may touch the network: the fetch takes an
// injected `get`, and the one test that lets the real hook spawn its refresh
// child points PROCODER_UPDATE_URL at a closed loopback port.
// ---------------------------------------------------------------------------

const {
  compareVersions, updateNotice, fetchLatestVersion, CACHE_TTL_MS,
} = require('../hooks/procoder-update-check');

const INSTALLED = require('../.claude-plugin/plugin.json').version;
const CACHE_NAME = '.procoder-update-check.json';
const DEAD_URL = 'https://127.0.0.1:1/plugin.json';

// updateNotice() resolves the cache dir from the env on every call, so a
// per-test temp dir is all the isolation these need — there is no module
// state to reset.
function withCache(contents, fn, env = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-upd-'));
  const saved = { ...process.env };
  Object.assign(process.env, { CLAUDE_CONFIG_DIR: dir, CLAUDE_PLUGIN_ROOT: dir, ...env });
  if (contents !== null) {
    fs.writeFileSync(path.join(dir, CACHE_NAME),
      typeof contents === 'string' ? contents : JSON.stringify(contents));
  }
  try {
    return fn(dir);
  } finally {
    for (const key of Object.keys(process.env)) delete process.env[key];
    Object.assign(process.env, saved);
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const counter = () => {
  const calls = [];
  const fn = () => calls.push(1);
  fn.calls = calls;
  return fn;
};

test('compareVersions orders by number, not by string', () => {
  assert.strictEqual(compareVersions('0.9.0', '0.10.0'), -1);
  assert.strictEqual(compareVersions('0.10.0', '0.9.0'), 1);
  assert.strictEqual(compareVersions('1.2.3', '1.2.3'), 0);
  assert.strictEqual(compareVersions('v1.2.3', '1.2.3'), 0);
  assert.strictEqual(compareVersions('1.2.3', '1.2.4'), -1);
  assert.strictEqual(compareVersions('2.0.0', '1.99.99'), 1);
});

test('compareVersions sorts a pre-release below its release', () => {
  assert.strictEqual(compareVersions('1.0.0-rc.1', '1.0.0'), -1);
  assert.strictEqual(compareVersions('1.0.0', '1.0.0-rc.1'), 1);
  assert.strictEqual(compareVersions('1.0.0-rc.1', '1.0.0-rc.1'), 0);
  assert.strictEqual(compareVersions('1.0.0-rc.1', '1.0.0-rc.2'), -1);
  assert.strictEqual(compareVersions('1.0.0+build.5', '1.0.0'), 0);
});

test('compareVersions returns null when either side is malformed', () => {
  assert.strictEqual(compareVersions('1.2', '1.2.3'), null);
  assert.strictEqual(compareVersions('1.2.3', 'banana'), null);
  assert.strictEqual(compareVersions('', '1.2.3'), null);
  assert.strictEqual(compareVersions('1.2.3', null), null);
  assert.strictEqual(compareVersions(undefined, undefined), null);
});

test('an up-to-date install says nothing', () => {
  withCache({ checkedAt: Date.now(), latest: INSTALLED }, () => {
    assert.strictEqual(updateNotice({ spawnRefresh: counter() }), '');
  });
});

test('an older published version says nothing — no downgrade nagging', () => {
  withCache({ checkedAt: Date.now(), latest: '0.0.1' }, () => {
    assert.strictEqual(updateNotice({ spawnRefresh: counter() }), '');
  });
});

test('a newer version names both versions and the update command', () => {
  withCache({ checkedAt: Date.now(), latest: '9.9.9' }, () => {
    const notice = updateNotice({ spawnRefresh: counter() });
    assert.match(notice, /9\.9\.9/);
    assert.match(notice, new RegExp(INSTALLED.replace(/\./g, '\\.')));
    assert.match(notice, /\/procoder:update/);
  });
});

test('a fresh cache spawns no refresh', () => {
  withCache({ checkedAt: Date.now(), latest: INSTALLED }, () => {
    const spawnRefresh = counter();
    updateNotice({ spawnRefresh });
    assert.strictEqual(spawnRefresh.calls.length, 0);
  });
});

test('a stale cache spawns exactly one refresh, and still reports what it has', () => {
  const stale = Date.now() - CACHE_TTL_MS - 1;
  withCache({ checkedAt: stale, latest: '9.9.9' }, () => {
    const spawnRefresh = counter();
    const notice = updateNotice({ spawnRefresh });
    assert.strictEqual(spawnRefresh.calls.length, 1);
    assert.match(notice, /9\.9\.9/);
  });
});

test('a missing or unusable cache is silent, and triggers the refresh that heals it', () => {
  for (const contents of [null, 'not json', '{"checkedAt":"soon"}', '{}']) {
    withCache(contents, () => {
      const spawnRefresh = counter();
      assert.strictEqual(updateNotice({ spawnRefresh }), '',
        `cache ${JSON.stringify(contents)} should be silent`);
      assert.strictEqual(spawnRefresh.calls.length, 1);
    });
  }
});

test('PROCODER_NO_UPDATE_CHECK suppresses the notice and the spawn', () => {
  withCache({ checkedAt: 0, latest: '9.9.9' }, () => {
    const spawnRefresh = counter();
    assert.strictEqual(updateNotice({ spawnRefresh }), '');
    assert.strictEqual(spawnRefresh.calls.length, 0);
  }, { PROCODER_NO_UPDATE_CHECK: '1' });
});

test('a source checkout — no CLAUDE_PLUGIN_ROOT — never checks or spawns', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-upd-'));
  const savedDir = process.env.CLAUDE_CONFIG_DIR;
  const savedRoot = process.env.CLAUDE_PLUGIN_ROOT;
  try {
    process.env.CLAUDE_CONFIG_DIR = dir;
    delete process.env.CLAUDE_PLUGIN_ROOT;
    fs.writeFileSync(path.join(dir, CACHE_NAME),
      JSON.stringify({ checkedAt: 0, latest: '9.9.9' }));
    const spawnRefresh = counter();
    assert.strictEqual(updateNotice({ spawnRefresh }), '');
    assert.strictEqual(spawnRefresh.calls.length, 0);
  } finally {
    if (savedDir === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = savedDir;
    if (savedRoot !== undefined) process.env.CLAUDE_PLUGIN_ROOT = savedRoot;
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

const exploding = () => { throw new Error('boom'); };

test('a throw inside the check is swallowed, not propagated to the hook', () => {
  withCache({ checkedAt: 0, latest: '9.9.9' }, () => {
    assert.doesNotThrow(
      () => assert.strictEqual(updateNotice({ spawnRefresh: exploding }), ''));
  });
});

// Fake `get` implementations, one per way a request can go wrong. None of them
// opens a socket.
const { EventEmitter } = require('events');

function fakeRequest(timesOut) {
  const req = new EventEmitter();
  req.destroy = () => {};
  req.setTimeout = (ms, onTimeout) => (timesOut ? setImmediate(onTimeout) : null);
  return req;
}

function deliver(callback, res, body) {
  callback(res);
  if (body) res.emit('data', body);
  res.emit('end');
}

function responding(statusCode, body) {
  return (url, options, callback) => {
    const res = new EventEmitter();
    res.statusCode = statusCode;
    res.setEncoding = () => {};
    res.resume = () => {};
    setImmediate(deliver, callback, res, body);
    return fakeRequest(false);
  };
}

const emitError = (req) => req.emit('error', new Error('ENOTFOUND'));

function failing(how) {
  if (how === 'throw') return () => { throw new Error('unsupported protocol'); };
  return () => {
    const req = fakeRequest(how === 'timeout');
    if (how === 'error') setImmediate(emitError, req);
    return req;
  };
}

test('fetchLatestVersion resolves null for every failure mode, and never throws', async () => {
  assert.strictEqual(await fetchLatestVersion(responding(200, '{"version":"1.2.3"}')), '1.2.3');
  assert.strictEqual(await fetchLatestVersion(responding(200, '<html>rate limited')), null);
  assert.strictEqual(await fetchLatestVersion(responding(200, '{"nope":1}')), null);
  assert.strictEqual(await fetchLatestVersion(responding(403, '')), null);
  assert.strictEqual(await fetchLatestVersion(responding(500, '')), null);
  assert.strictEqual(await fetchLatestVersion(failing('error')), null);
  assert.strictEqual(await fetchLatestVersion(failing('timeout')), null);
  assert.strictEqual(await fetchLatestVersion(failing('throw')), null);
});

// --- the hook itself -------------------------------------------------------

function runHook(dir, env = {}) {
  const started = Date.now();
  const stdout = execHook(HOOK, {
    env: {
      ...process.env,
      CLAUDE_CONFIG_DIR: dir,
      CLAUDE_PLUGIN_ROOT: dir,
      PROCODER_UPDATE_URL: DEAD_URL,
      ...env,
    },
  });
  return { stdout, ms: Date.now() - started };
}

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// Polls rather than sleeping a fixed span: the detached child is not something
// the test can await, and a fixed sleep is either flaky or slow.
async function waitForRefresh(file, since) {
  for (let i = 0; i < 60; i++) {
    await wait(50);
    if (JSON.parse(fs.readFileSync(file, 'utf8')).checkedAt > since) return true;
  }
  return false;
}

function seedCache(cache) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-upd-'));
  if (cache) fs.writeFileSync(path.join(dir, CACHE_NAME), JSON.stringify(cache));
  return dir;
}

test('the hook carries the notice alongside the doctrine, not instead of it', () => {
  const dir = seedCache({ checkedAt: Date.now(), latest: '9.9.9' });
  try {
    const { stdout } = runHook(dir);
    assert.match(stdout, /9\.9\.9/);
    assert.match(stdout, /\/procoder:update/);
    assert.match(stdout, /SAFE/);
    assert.match(stdout, /ALONE/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('the hook stays silent when up to date, and still injects the doctrine', () => {
  const dir = seedCache({ checkedAt: Date.now(), latest: INSTALLED });
  try {
    const { stdout } = runHook(dir);
    assert.ok(!/procoder:update/.test(stdout), 'notified while up to date');
    assert.match(stdout, /SAFE/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('a broken check cannot stop the doctrine reaching the session', () => {
  // The cache path is a directory, so every read and write against it throws.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-upd-'));
  fs.mkdirSync(path.join(dir, CACHE_NAME));
  try {
    const { stdout } = runHook(dir);
    assert.match(stdout, /SAFE/);
    assert.match(stdout, /ALONE/);
    assert.ok(!/\n\s+at .*:\d+:\d+/.test(stdout), 'a stack trace leaked into the session');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('PROCODER_NO_UPDATE_CHECK leaves no cache behind and still injects the doctrine',
  async () => {
    const dir = seedCache(null);
    try {
      const { stdout } = runHook(dir, { PROCODER_NO_UPDATE_CHECK: '1' });
      assert.match(stdout, /SAFE/);
      await wait(300);
      assert.ok(!fs.existsSync(path.join(dir, CACHE_NAME)),
        'the opt-out did not suppress the refresh spawn');
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

test('a stale cache refreshes detached: the hook returns at once, the child writes later',
  async () => {
    const stale = Date.now() - CACHE_TTL_MS - 1;
    const dir = seedCache({ checkedAt: stale, latest: '9.9.9' });
    const cache = path.join(dir, CACHE_NAME);
    try {
      const { stdout, ms } = runHook(dir);
      // The hook did not wait on the refresh: it reports the cached 9.9.9 and
      // exits. 2s is a generous ceiling for node startup alone — the point is
      // that it is nowhere near a network round trip or the 5s hook timeout.
      assert.match(stdout, /9\.9\.9/);
      assert.ok(ms < 2000, `session start took ${ms}ms`);

      // The detached child stamps the cache before it fetches, so a moved
      // checkedAt proves it was spawned — with no network involved, because
      // PROCODER_UPDATE_URL points at a closed loopback port.
      assert.ok(await waitForRefresh(cache, stale), 'the stale cache never got refreshed');

      // An unreachable host must leave the last known version in place rather
      // than blanking it.
      assert.strictEqual(JSON.parse(fs.readFileSync(cache, 'utf8')).latest, '9.9.9');
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
