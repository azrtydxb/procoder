// tests/resolve.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
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
  assert.strictEqual(isConfigured(tempRepo({ 'setup.cfg': '[metadata]\n' }), TOOLS.py), false);
  assert.strictEqual(isConfigured(tempRepo({ 'Cargo.toml': '[package]\nname="x"\n' }), TOOLS.rust), false);
  assert.strictEqual(isConfigured(tempRepo({ 'Cargo.toml': '[lints.clippy]\n' }), TOOLS.rust), true);
  assert.strictEqual(isConfigured(tempRepo({ 'clippy.toml': '' }), TOOLS.rust), true);
});

test('resolveFor yields null when the tool is unconfigured', () => {
  assert.strictEqual(resolveFor('a.py', { repoRoot: tempRepo() }), null);
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

  test(`${toolName}: absent → no tool resolves, the pack runs`, { skip: !shimmable }, () => {
    assert.strictEqual(run(configured, null), null);
  });

  test(`${toolName}: present but unconfigured → no tool resolves, the pack runs`, { skip: !shimmable }, () => {
    assert.strictEqual(run(unconfigured, answers), null);
  });

  test(`${toolName}: configured and answering → its findings appear and the pack defers`, { skip: !shimmable }, () => {
    const out = run(configured, answers);
    assert.ok(out, 'a configured, present tool must resolve');
    assert.ok(out.findings.length > 0, `${toolName} findings were dropped`);
    assert.strictEqual(out.ok, true);
    for (const f of out.findings) assert.match(f.id, /^true\//);
  });

  test(`${toolName}: configured and failing → not ok, the pack runs`, { skip: !shimmable }, () => {
    const out = run(configured, fails);
    assert.deepStrictEqual(out.findings, []);
    assert.strictEqual(out.ok, false, 'a failing tool must fall back to the pack, never to silence');
  });

  test(`${toolName}: configured and genuinely clean → ok, the pack does not double-report`, { skip: !shimmable }, () => {
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
