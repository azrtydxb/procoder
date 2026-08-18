// tests/resolve.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { hasTool, isConfigured, resolveFor, runTool, runToolResult } = require('../hooks/checks/resolve');
const { TOOLS } = require('../hooks/checks/registry');

// Tracked centrally and swept in one `after` hook, rather than a try/finally
// per test, so a test that adds a new tempRepo() call later can't forget it.
const tempDirs = [];
test.after(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true });
});

function tempRepo(files = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-res-'));
  tempDirs.push(dir);
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('hasTool finds node and does not find a nonsense binary', () => {
  assert.strictEqual(hasTool('node'), true);
  assert.strictEqual(hasTool('procoder-definitely-not-a-real-binary'), false);
});

test('isConfigured requires one of the tool config files', () => {
  assert.strictEqual(isConfigured(tempRepo({ '.eslintrc.json': '{}' }), TOOLS.ts), true);
  assert.strictEqual(isConfigured(tempRepo({ 'ruff.toml': '' }), TOOLS.py), true);
  assert.strictEqual(isConfigured(tempRepo(), TOOLS.ts), false);
});

test('a shared manifest is not evidence unless it names the tool', () => {
  assert.strictEqual(isConfigured(tempRepo({ 'pyproject.toml': '[project]\nname="x"\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'pyproject.toml': '[tool.ruff]\n' }), TOOLS.py), true);
  // setup.cfg is not evidence for ruff in any form: ruff does not read it.
  // Verified against ruff 0.16.3 — a setup.cfg carrying `[ruff] line-length =
  // 20` changes nothing about the run — so counting it deferred procoder's
  // obvious/* rules to a ruff on its defaults, which has no shape rule at all.
  assert.strictEqual(isConfigured(tempRepo({ 'setup.cfg': '[metadata]\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'setup.cfg': '[ruff]\nline-length = 20\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'setup.cfg': '[flake8]\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'Cargo.toml': '[package]\nname="x"\n' }), TOOLS.rust), false);
  assert.strictEqual(isConfigured(tempRepo({ 'Cargo.toml': '[lints.clippy]\n' }), TOOLS.rust), true);
  assert.strictEqual(isConfigured(tempRepo({ 'clippy.toml': '' }), TOOLS.rust), true);
});

// The inversion at the heart of the rewrite. This test used to assert the
// opposite — that an unconfigured tool does not resolve — and that rule is why a
// repository with no ruff config was never checked at all, with the silence
// reading as a pass. An analyzer now runs because the file is in a language it
// answers for; the project's own config is still honoured when present, because
// the analyzer reads it, but its ABSENCE no longer excuses the file.
test('resolveFor runs the analyzer even when the project has not configured it', () => {
  const resolved = resolveFor('a.py', { repoRoot: tempRepo() });
  // Present-and-unconfigured must resolve; absent still yields null, and which
  // of those this machine is running is not the test's business.
  if (hasTool('ruff')) {
    assert.ok(resolved, 'an installed analyzer must run without project config');
    assert.strictEqual(resolved.name, 'ruff');
  } else {
    assert.strictEqual(resolved, null);
  }
});

test('resolveFor yields null for a file type with no tool', () => {
  assert.strictEqual(resolveFor('a.cs', { repoRoot: tempRepo({ '.eslintrc': '{}' }) }), null);
});

test('runTool returns an empty array when the binary is missing', () => {
  const fake = { name: 'procoder-missing-binary', argv: () => ['x'], parse: () => [{}] };
  assert.deepStrictEqual(runTool(fake, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 500 }), []);
});

test('runTool honours the timeout and returns an empty array', () => {
  const slow = { name: 'node', argv: () => ['-e', 'setTimeout(()=>{}, 10000)'], parse: () => [{}] };
  const started = Date.now();
  const out = runTool(slow, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 400 });
  assert.deepStrictEqual(out, []);
  assert.ok(Date.now() - started < 3000, 'runTool did not abandon the slow process');
});

test('runToolResult reports an abnormal exit as not ok', () => {
  const missing = { name: 'procoder-missing-binary', argv: () => ['x'], parse: () => [] };
  assert.strictEqual(runToolResult(missing, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 500 }).ok, false);

  const slow = { name: 'node', argv: () => ['-e', 'setTimeout(()=>{}, 10000)'], parse: () => [] };
  assert.strictEqual(runToolResult(slow, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 400 }).ok, false);
});

test('runToolResult reports a clean exit with no output as ok', () => {
  const clean = { name: 'node', argv: () => ['-e', 'process.stdout.write("[]")'], parse: () => [] };
  const out = runToolResult(clean, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.deepStrictEqual(out, { findings: [], ok: true });
});

test('runTool parses stdout through the tool parser', () => {
  const echo = {
    name: 'node',
    argv: () => ['-e', 'process.stdout.write(JSON.stringify([{filename:"a.py",location:{row:3},code:"E1",message:"boom"}]))'],
    parse: TOOLS.py.parse,
  };
  const out = runTool(echo, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.strictEqual(out.length, 1);
  assert.strictEqual(out[0].line, 3);
});

// --- stream selection -------------------------------------------------------
//
// cargo clippy writes every diagnostic to stderr and exits 0. A runner that
// reads stdout only sees an empty string from a clean exit, which is
// indistinguishable from "the crate is clean" — and the caller then skips the
// built-in pack. Configuring clippy would make procoder check LESS.

test('a tool that reports on stderr is read, not discarded', () => {
  const onStderr = {
    name: 'node',
    stream: 'stderr',
    argv: () => ['-e', 'process.stderr.write("src/lib.rs:7:5: warning: unneeded `return` statement\\n")'],
    parse: TOOLS.rust.parse,
  };
  TOOLS.rust.argv('/repo/src/lib.rs');
  const out = runToolResult(onStderr, { repoRoot: '/tmp', absPath: '/repo/src/lib.rs', timeoutMs: 2000 });
  assert.strictEqual(out.ok, true, 'a stderr-reporting tool that answered must be ok');
  assert.strictEqual(out.findings.length, 1, 'the stderr diagnostic must be parsed');
  assert.strictEqual(out.findings[0].line, 7);
});

test('a stderr-reporting tool that exits 0 with nothing at all is clean, not unreadable', () => {
  const silent = { name: 'node', stream: 'stderr', argv: () => ['-e', '0'], parse: TOOLS.rust.parse };
  assert.deepStrictEqual(
    runToolResult(silent, { repoRoot: '/tmp', absPath: '/repo/src/lib.rs', timeoutMs: 2000 }),
    { findings: [], ok: true },
  );
});

test('a stdout tool is still read when a stderr tool exists', () => {
  const onStdout = {
    name: 'node',
    argv: () => ['-e', 'process.stdout.write(JSON.stringify([{location:{row:4},code:"E2",message:"x"}]))'],
    parse: TOOLS.py.parse,
  };
  const out = runToolResult(onStdout, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.strictEqual(out.ok, true);
  assert.strictEqual(out.findings.length, 1);
});

// --- clean vs unreadable ----------------------------------------------------

test('output the parser cannot read is not ok, even though it is not empty', () => {
  const noise = {
    name: 'node',
    argv: () => ['-e', 'process.stdout.write("Segmentation fault (core dumped)\\n"); process.exit(2)'],
    parse: TOOLS.py.parse,
  };
  const out = runToolResult(noise, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.deepStrictEqual(out.findings, []);
  assert.strictEqual(out.ok, false, 'unreadable output must fall back to the pack');
});

test('a clean exit printing an empty result set is ok and does not rerun the pack', () => {
  const clean = { name: 'node', argv: () => ['-e', 'process.stdout.write("[]")'], parse: TOOLS.py.parse };
  assert.deepStrictEqual(
    runToolResult(clean, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 }),
    { findings: [], ok: true },
  );
});

// --- the four states, per tool, on PATH shims -------------------------------
//
// No test here may need a real cargo, ruff or eslint: CI runs on machines that
// have none of them. Each state builds a PATH containing only the shim dir and
// the system directories `which` itself lives in, so a tool that happens to be
// installed on the developer's machine cannot leak into the "absent" state.
const SYSTEM_PATH = ['/usr/bin', '/bin', '/usr/sbin', '/sbin'].join(path.delimiter);
const shimmable = process.platform !== 'win32';

function shimDir(shims) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-shim-'));
  tempDirs.push(dir);
  for (const [name, body] of Object.entries(shims)) {
    const file = path.join(dir, name);
    fs.writeFileSync(file, `#!/bin/sh\n${body}\n`);
    fs.chmodSync(file, 0o755);
  }
  return dir;
}

function withPath(dir, fn) {
  const saved = process.env.PATH;
  process.env.PATH = dir ? `${dir}${path.delimiter}${SYSTEM_PATH}` : SYSTEM_PATH;
  try {
    return fn();
  } finally {
    process.env.PATH = saved;
  }
}

// state → what the caller must end up doing. `null` from resolveFor means no
// tool ran at all, so run.js reports answered:false and the pack covers the
// file; ok:false means the same after a run that told us nothing.
function runShimmed({ toolName, relFile, files, shim }) {
  const repo = tempRepo(files);
  return withPath(shimDir(shim === null ? {} : { [toolName]: shim }), () => {
    const tool = resolveFor(relFile, { repoRoot: repo });
    if (!tool) return null;
    return runToolResult(tool, { repoRoot: repo, absPath: path.join(repo, relFile), timeoutMs: 2000 });
  });
}

function fourStates(spec) {
  const { toolName, relFile, configured, unconfigured, answers, fails } = spec;
  const run = (files, shim) => runShimmed({ toolName, relFile, files, shim });

  test(`${toolName}: absent → no tool resolves; toolchain.js reports the gap`, { skip: !shimmable }, () => {
    assert.strictEqual(run(configured, null), null);
  });

  test(`${toolName}: present but unconfigured → it still runs, and still reports`, { skip: !shimmable }, () => {
    const out = run(unconfigured, answers);
    assert.ok(out, `${toolName} must run without project config — silence is not a pass`);
    assert.ok(out.findings.length > 0, `${toolName} findings were dropped`);
    assert.strictEqual(out.ok, true);
  });

  test(`${toolName}: configured and answering → its findings appear`, { skip: !shimmable }, () => {
    const out = run(configured, answers);
    assert.ok(out, 'a configured, present tool must resolve');
    assert.ok(out.findings.length > 0, `${toolName} findings were dropped`);
    assert.strictEqual(out.ok, true);
    // A finding is namespaced by the rung it lands on: security rules are rung 1
    // so they sort and block ahead of style, everything else stays rung 2.
    for (const f of out.findings) assert.match(f.id, /^(safe|true)\//);
  });

  test(`${toolName}: configured and failing → not ok, never silently clean`, { skip: !shimmable }, () => {
    const out = run(configured, fails);
    assert.deepStrictEqual(out.findings, []);
    assert.strictEqual(out.ok, false, 'a failing analyzer must never read as a clean file');
  });

  test(`${toolName}: configured and genuinely clean → ok, and nothing is invented`, { skip: !shimmable }, () => {
    assert.deepStrictEqual(run(configured, 'exit 0'), { findings: [], ok: true });
  });
}

// clippy: diagnostics on stderr, exit 0 even when it found something.
fourStates({
  toolName: 'cargo',
  relFile: 'src/lib.rs',
  configured: { 'Cargo.toml': '[package]\nname="x"\n\n[lints.clippy]\nall = "warn"\n', 'src/lib.rs': 'pub fn f() {}\n' },
  unconfigured: { 'Cargo.toml': '[package]\nname="x"\n', 'src/lib.rs': 'pub fn f() {}\n' },
  answers: 'printf "src/lib.rs:1:76: warning: unneeded \\`return\\` statement\\n" >&2\nexit 0',
  fails: 'printf "error: could not compile \\`x\\` (lib)\\n" >&2\nexit 101',
});

// ruff: diagnostics on stdout as JSON, non-zero exit when it found something.
fourStates({
  toolName: 'ruff',
  relFile: 'a.py',
  configured: { 'ruff.toml': '', 'a.py': 'x = 1\n' },
  unconfigured: { 'a.py': 'x = 1\n' },
  answers: 'printf \'[{"location":{"row":1},"code":"E722","message":"do not use bare except"}]\'\nexit 1',
  fails: 'printf "ruff: unrecognized subcommand\\n"\nexit 2',
});

// --- the four states, per tool, against the REAL binary ---------------------
//
// A shim proves only that the parser handles the shape the shim author
// imagined. Two of the four wired tools were broken in ways shims never
// caught — clippy reporting on stderr, golangci-lint v2 appending a tally to
// its JSON — and eslint and ruff had never been run at all. These tests run
// the real binary when it is on PATH and skip when it is not: a developer or
// CI runner that has the tool exercises the integration, one that does not
// still passes. No test here may REQUIRE a binary.
//
// Findings are asserted by file, line and id, not merely by count: the whole
// class of defect being guarded against is a parser that reads real output as
// something other than what it is.
const REAL_TIMEOUT_MS = 60000;

function toolMajor(name, args) {
  const run = spawnSync(name, args, { encoding: 'utf8', timeout: 10000 });  // procoder: literal safe/semgrep:javascript.lang.security.detect-child-process.detect-child-process a test spawning the shimmed linter it is testing
  const banner = `${run.stdout || ''}${run.stderr || ''}`;
  const m = /(\d+)\.\d+/.exec(banner);
  return m ? Number(m[1]) : -1;
}

function realRun(relFile, files) {
  const repo = tempRepo(files);
  const tool = resolveFor(relFile, { repoRoot: repo });
  if (!tool) return null;
  return runToolResult(tool, { repoRoot: repo, absPath: path.join(repo, relFile), timeoutMs: REAL_TIMEOUT_MS });
}

// `which` has to stay reachable for hasTool to answer at all, so the blanked
// PATH keeps the system directories. A tool installed in one of them cannot be
// hidden that way, and the shim states above already cover absence, so the
// real absence test says it is skipping rather than passing vacuously.
function systemVisible(name) {
  return shimmable && withPath(shimDir({}), () => hasTool(name));
}

const ESLINT = !shimmable ? 'shims unavailable on this platform'
  : !hasTool('eslint') ? 'eslint is not on PATH'
    : toolMajor('eslint', ['--version']) < 9 ? 'eslint older than 9 has no flat config'
      : false;
const RUFF = !shimmable ? 'shims unavailable on this platform'
  : !hasTool('ruff') ? 'ruff is not on PATH' : false;

// Flat config only — eslint 9 rejects an .eslintignore outright and eslint 10
// dropped .eslintrc, so mixing the two eras in one fixture tests neither.
const ESLINT_CONFIG = "module.exports = [{ rules: { 'no-unused-vars': 'error', 'no-eval': 'error' } }];\n";
const ESLINT_REPO = {
  'package.json': '{"name":"procoder-fixture","version":"1.0.0","private":true}\n',
  'eslint.config.js': ESLINT_CONFIG,
  'a.js': 'var dead = 1;\n',
};
const RUFF_REPO = {
  'ruff.toml': '[lint]\nselect = ["F"]\n',
  'a.py': 'import os\n',
};

test('eslint (real): absent → no tool resolves, the pack runs', {
  skip: ESLINT || (systemVisible('eslint') && 'eslint lives in a system dir; the shim states cover absence'),
}, () => {
  const repo = tempRepo(ESLINT_REPO);
  assert.strictEqual(withPath(shimDir({}), () => resolveFor('a.js', { repoRoot: repo })), null);
});

test('eslint (real): present but unconfigured → it still runs', { skip: ESLINT }, () => {
  // eslint with no flat config reports its own error rather than linting, which
  // is `ok: false` — a gap, not a clean file. Either way it must not be null:
  // resolving to null is how the old design turned "unconfigured" into "passed".
  const out = realRun('a.js', { 'a.js': 'var dead = 1;\n' });
  assert.ok(out, 'an installed eslint must be asked, config or not');
});

test('eslint (real): configured and answering → its findings carry file, line and rule id', { skip: ESLINT }, () => {
  const out = realRun('a.js', ESLINT_REPO);
  assert.strictEqual(out.ok, true);
  assert.deepStrictEqual(out.findings.map((f) => [f.rung, f.id, f.line]), [['ALONE', 'alone/eslint:no-unused-vars', 1]]);
});

test('eslint (real): configured and genuinely clean → ok, so the shape rules stay deferred', { skip: ESLINT }, () => {
  assert.deepStrictEqual(
    realRun('a.js', { ...ESLINT_REPO, 'a.js': 'module.exports = 1;\n' }),
    { findings: [], ok: true },
  );
});

test('eslint (real): configured but failing → not ok, the pack runs', { skip: ESLINT }, () => {
  const out = realRun('a.js', { ...ESLINT_REPO, 'eslint.config.js': 'this is not valid javascript {{{\n' });
  assert.deepStrictEqual(out.findings, []);
  assert.strictEqual(out.ok, false, 'a failing eslint must fall back to the pack, never to silence');
});

test('eslint (real): a file eslint ignores is not a clean file', { skip: ESLINT }, () => {
  // eslint answers an ignored path with one line-less warning and exit 0. The
  // runner drops line-less findings, so this read as "answered, nothing found"
  // and deleted the pack's obvious/* rules for every path in an `ignores` list.
  const out = realRun('a.js', { ...ESLINT_REPO, 'eslint.config.js': "module.exports = [{ ignores: ['a.js'] }];\n" });
  assert.deepStrictEqual(out.findings, []);
  assert.strictEqual(out.ok, false, 'a file eslint declined to lint must fall back to the pack');
});

test('eslint (real): a file eslint cannot parse is not a clean file', { skip: ESLINT }, () => {
  const out = realRun('a.js', { ...ESLINT_REPO, 'a.js': 'function ( {\n' });
  assert.strictEqual(out.ok, false, 'a fatal parse error means eslint linted nothing');
});

test('ruff (real): absent → no tool resolves, the pack runs', {
  skip: RUFF || (systemVisible('ruff') && 'ruff lives in a system dir; the shim states cover absence'),
}, () => {
  const repo = tempRepo(RUFF_REPO);
  assert.strictEqual(withPath(shimDir({}), () => resolveFor('a.py', { repoRoot: repo })), null);
});

test('ruff (real): present but unconfigured → it still runs, and still reports', { skip: RUFF }, () => {
  const out = realRun('a.py', { 'a.py': 'import os\n' });
  assert.ok(out, 'an installed ruff must be asked, config or not');
  assert.strictEqual(out.ok, true);
});

test('ruff (real): configured and answering → its findings carry file, line and rule id', { skip: RUFF }, () => {
  const out = realRun('a.py', RUFF_REPO);
  assert.strictEqual(out.ok, true);
  assert.deepStrictEqual(out.findings.map((f) => [f.rung, f.id, f.line]), [['ALONE', 'alone/ruff:F401', 1]]);
});

test('ruff (real): configured and genuinely clean → ok, so the shape rules stay deferred', { skip: RUFF }, () => {
  assert.deepStrictEqual(realRun('a.py', { ...RUFF_REPO, 'a.py': 'x = 1\n' }), { findings: [], ok: true });
});

test('ruff (real): configured but failing → not ok, the pack runs', { skip: RUFF }, () => {
  const out = realRun('a.py', { ...RUFF_REPO, 'ruff.toml': 'not-a-real-ruff-setting = 3\n' });
  assert.deepStrictEqual(out.findings, []);
  assert.strictEqual(out.ok, false, 'a failing ruff must fall back to the pack, never to silence');
});

test('ruff (real): a path the project excludes is still answered, not reported clean', { skip: RUFF }, () => {
  // With --force-exclude, ruff answered an excluded path with `[]` and exit 0 —
  // byte for byte what a clean file produces — so procoder deferred its shape
  // rules to a run that never opened the file. Without it, `[]` means clean.
  const out = realRun('sub/a.py', {
    'ruff.toml': '[lint]\nselect = ["F"]\n\nexclude = ["sub"]\n',
    'sub/a.py': 'import os\n',
  });
  assert.strictEqual(out.ok, true);
  assert.deepStrictEqual(out.findings.map((f) => f.id), ['alone/ruff:F401']);
});

test('ruff (real): a file ruff cannot parse is not a clean file', { skip: RUFF }, () => {
  const out = realRun('a.py', { ...RUFF_REPO, 'a.py': 'def f(\n' });
  assert.strictEqual(out.ok, false, 'a syntax error means ruff linted nothing else');
});

// --- cargo's cached replay --------------------------------------------------
//
// cargo replays a fresh unit's cached diagnostics in the format that compiled
// it, so a hand-run `cargo clippy` leaves a cache procoder's `--message-format
// short` run cannot read. Two ways that ends badly, both silent: the whole
// output becomes unreadable and every clippy finding is dropped, or — when one
// recompiled unit does emit a short line — the run reads as answered while the
// replayed diagnostics for the file under inspection vanish, taking the Rust
// pack's obvious/* rules with them. A tool that cannot be read must cost
// coverage nowhere.

test('a cached long-format replay is read, not silently dropped', () => {
  const replay = {
    name: 'node',
    stream: 'stderr',
    argv: () => ['-e', 'process.stderr.write("warning: unneeded `return` statement\\n --> src/lib.rs:2:5\\n  |\\n")'],
    parse: TOOLS.rust.parse,
  };
  TOOLS.rust.argv('/repo/src/lib.rs');
  const out = runToolResult(replay, { repoRoot: '/tmp', absPath: '/repo/src/lib.rs', timeoutMs: 2000 });
  assert.strictEqual(out.findings.length, 1, 'the replayed diagnostic was dropped');
  assert.strictEqual(out.findings[0].line, 2);
  assert.strictEqual(out.ok, true);
});

test('a replay mixed with one recompiled unit never reads as a clean file', () => {
  // The worst shape: the short line belongs to another file and is filtered
  // out, so parse() succeeds with nothing left, exit 0 says clean, and the
  // pack is skipped — while the file under inspection had a warning all along.
  const mixed = {
    name: 'node',
    stream: 'stderr',
    argv: () => ['-e', 'process.stderr.write("src/other.rs:9:1: warning: needless return\\nwarning: unneeded `return` statement\\n --> src/lib.rs:2:5\\n")'],
    parse: TOOLS.rust.parse,
  };
  TOOLS.rust.argv('/repo/src/lib.rs');
  const out = runToolResult(mixed, { repoRoot: '/tmp', absPath: '/repo/src/lib.rs', timeoutMs: 2000 });
  assert.deepStrictEqual(out.findings.map((f) => f.line), [2],
    'the replayed diagnostic for this file was reported as a clean run');
});

const CARGO = !shimmable ? 'shims unavailable on this platform'
  : !hasTool('cargo') ? 'cargo is not on PATH' : false;

test('cargo (real): a cache primed by a hand-run cargo clippy still answers', { skip: CARGO }, () => {
  const repo = tempRepo({
    'Cargo.toml': '[package]\nname = "cratea"\nversion = "0.1.0"\nedition = "2021"\n\n[lints.clippy]\nall = "warn"\n',
    'src/lib.rs': 'pub fn f(x: i32) -> i32 {\n    return x + 1;\n}\n',
  });
  const primed = spawnSync('cargo', ['clippy'], { cwd: repo, encoding: 'utf8', timeout: REAL_TIMEOUT_MS });
  assert.ok(!primed.error, `cargo did not run: ${primed.error && primed.error.message}`);
  const tool = resolveFor('src/lib.rs', { repoRoot: repo });
  assert.ok(tool, 'a configured cargo must resolve');
  const out = runToolResult(tool, {
    repoRoot: repo, absPath: path.join(repo, 'src/lib.rs'), timeoutMs: REAL_TIMEOUT_MS,
  });
  assert.ok(out.findings.length >= 1, 'running your own clippy made procoder blind');
  assert.strictEqual(out.findings[0].line, 2);
  assert.strictEqual(out.ok, true);
});

test('cargo (real): a compile error plus a warning is not an answer', { skip: CARGO }, () => {
  // Two compilation units in one package: lib.rs carries a lint warning and
  // compiles, bin/main.rs does not compile at all. cargo prints the warning
  // AND `error[E0425]`, and exits 101. The warning is attributed to the file
  // under inspection, so parse() used to succeed with one finding and the run
  // read as answered — deleting the Rust pack's obvious/* rules for a crate
  // clippy never finished analysing.
  const repo = tempRepo({
    'Cargo.toml': '[package]\nname = "cratea"\nversion = "0.1.0"\nedition = "2021"\n\n[lints.clippy]\nall = "warn"\n',
    'src/lib.rs': 'pub fn f(x: i32) -> i32 {\n    return x + 1;\n}\n',
    'src/main.rs': 'fn main() {\n    let _ = missing_value;\n}\n',
  });
  const tool = resolveFor('src/lib.rs', { repoRoot: repo });
  assert.ok(tool, 'a configured cargo must resolve');
  const out = runToolResult(tool, {
    repoRoot: repo, absPath: path.join(repo, 'src/lib.rs'), timeoutMs: REAL_TIMEOUT_MS,
  });
  assert.strictEqual(out.ok, false, 'a crate that does not compile is a crate clippy did not lint');
});

test('cargo (real): a genuinely clean crate still answers', { skip: CARGO }, () => {
  // The over-correction to guard against: if a clean run stopped counting as an
  // answer, the built-in Rust pack would run alongside clippy on every crate.
  const repo = tempRepo({
    'Cargo.toml': '[package]\nname = "cleanc"\nversion = "0.1.0"\nedition = "2021"\n\n[lints.clippy]\nall = "warn"\n',
    'src/lib.rs': 'pub fn f(x: i32) -> i32 {\n    x + 1\n}\n',
  });
  const tool = resolveFor('src/lib.rs', { repoRoot: repo });
  const out = runToolResult(tool, {
    repoRoot: repo, absPath: path.join(repo, 'src/lib.rs'), timeoutMs: REAL_TIMEOUT_MS,
  });
  assert.strictEqual(out.ok, true, 'a clean crate must stay answered, or the pack duplicates it');
  assert.deepStrictEqual(out.findings, []);
});

test('the SAFE rung is never deferred to an external tool', () => {
  // Every external finding lands on TRUE, so no tool run can ever stand in for
  // a safe/* rule — run.js keeps the pack's SAFE rules whatever the tool says.
  const onStderr = {
    name: 'node',
    stream: 'stderr',
    argv: () => ['-e', 'process.stderr.write("src/lib.rs:7:5: warning: bad\\n")'],
    parse: TOOLS.rust.parse,
  };
  TOOLS.rust.argv('/repo/src/lib.rs');
  const { findings } = runToolResult(onStderr, { repoRoot: '/tmp', absPath: '/repo/src/lib.rs', timeoutMs: 2000 });
  for (const f of findings) {
    assert.strictEqual(f.rung, 'TRUE');
    assert.ok(!f.id.startsWith('safe/'));
  }
});
