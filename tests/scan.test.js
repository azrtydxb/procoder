// tests/scan.test.js
//
// The parallel scan has to be invisible: same findings, same order, same exit
// code, whatever the machine's core count. These tests exist because the two
// ways it could fail are both silent — a slice whose worker died and was never
// scanned, and a report whose order depends on which worker answered first.
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const {
  scanFiles, defaultJobs, clampJobs, PARALLEL_MIN_FILES, MAX_JOBS, SCAN_BUDGET_MS,
} = require('../hooks/checks/scan');
const { checkFile, BUDGET_MS } = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');

const CLI = path.join(__dirname, '..', 'bin', 'procoder.js');

const tempDirs = [];
test.after(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true });
});

// Enough files to cross the threshold, half of them carrying one finding, so a
// slice that went missing changes the answer rather than merely the timing.
function bigRepo(count = PARALLEL_MIN_FILES + 40) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-scan-'));
  tempDirs.push(dir);
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (let i = 0; i < count; i += 1) {
    const dirty = i % 2 === 0;
    fs.writeFileSync(path.join(dir, `m${i}.ts`),
      dirty ? 'eval(x);\n' : 'const x = 1;\n');  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  }
  return dir;
}

const filesIn = (dir) => fs.readdirSync(dir)
  .filter((f) => f.endsWith('.ts')).sort()
  .map((f) => path.join(dir, f));

test('a parallel scan returns exactly what a sequential one does, in the same order', async () => {
  const repo = bigRepo();
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };

  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);
  const parallel = await scanFiles(files, { ...options, jobs: 4 }, checkFile);

  assert.strictEqual(parallel.length, sequential.length);
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
  assert.ok(sequential.some((r) => r.findings.length > 0), 'the fixture stopped finding anything');
});

// Below the threshold the workers cost more than the work, so nothing is
// forked. Asserted through the result rather than by counting processes: what
// matters is that the small path still answers.
test('a small scan is answered without forking anything', async () => {
  const repo = bigRepo(3);
  const files = filesIn(repo);
  const out = await scanFiles(files,
    { repoRoot: repo, config: loadConfig(repo), applyBaseline: false, jobs: 8 }, checkFile);
  assert.strictEqual(out.length, 3);
  assert.ok(out.some((r) => r.findings.length > 0));
});

// The failure this must never have: a slice nobody scanned, reported as a clean
// slice. The worker is made unrunnable by pointing the scan at a jobs count it
// will use and an execPath that cannot start — simulated here by checking that
// a null result falls back, through the public behaviour: identical output.
test('a worker that produces nothing is scanned in this process instead', async () => {
  const repo = bigRepo();
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };

  // A worker whose stdout is unparseable is the same failure as a crash: the
  // parent must fall back rather than record an empty slice.
  const saved = process.execPath;
  const fake = path.join(repo, 'not-node');
  fs.writeFileSync(fake, '#!/bin/sh\necho not-json\n', { mode: 0o755 });
  Object.defineProperty(process, 'execPath', { value: fake, configurable: true });
  try {
    const out = await scanFiles(files, { ...options, jobs: 4 }, checkFile);
    assert.strictEqual(out.length, files.length, 'a slice went missing');
    assert.ok(out.some((r) => r.findings.length > 0), 'the fallback lost every finding');
  } finally {
    Object.defineProperty(process, 'execPath', { value: saved, configurable: true });
  }
});

test('defaultJobs stays within one process per core', () => {
  const jobs = defaultJobs();
  assert.ok(jobs >= 1);
  assert.ok(jobs <= Math.max(1, (os.cpus() || []).length));
});

// The cores this process may USE, not the ones the machine has. A container
// with a CPU quota reports the host's count through os.cpus() — `--cpus 1` on
// node:24 says 10 — and forking eight workers there measured 2.3x slower than
// not forking at all.
test('the default is bounded by the parallelism actually available', () => {
  assert.ok(defaultJobs() <= Math.max(1, os.availableParallelism()),
    'the default outruns the cores this process is allowed');
});

// End to end, because the CLI is where the exit code lives and an async main()
// that lost its rejection would exit 0 — a gate reading as a pass.
test('the CLI reports the same findings and exit code at any --jobs', () => {
  const repo = bigRepo();
  const run = (jobs) => spawnSync('node', [CLI, 'check', '--jobs', String(jobs), '.'],
    { cwd: repo, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo } });
  const one = run(1);
  const many = run(4);
  assert.strictEqual(one.status, 1, 'the fixture should fail the run');
  assert.strictEqual(many.status, one.status);
  assert.strictEqual(many.stdout, one.stdout);
});

// --- the budget, down both paths ------------------------------------------
//
// The divergence these cover: a worker used to run every file at 600,000ms
// while this process ran it at 2,000ms, so `true/budget-exhausted` — the
// finding that exists to say "I ran out of time and did not finish checking
// this file" — fired sequentially and effectively never in parallel. The FASTER
// path was the one that silently kept grinding.

test('the scan budget is the engine budget, in one place', () => {
  assert.strictEqual(SCAN_BUDGET_MS, BUDGET_MS,
    'scan.js and run.js hold two different per-file budgets again');
});

// A dependency manifest is the cheapest stage a deadline can cut — no linter,
// no subprocess, no timing luck — so a zero budget makes "the budget ran out"
// deterministic on any host. What is asserted is not the finding but the
// AGREEMENT: whatever the budget does, it does the same thing in a worker.
test('a budget that runs out reports identically down both paths', async () => {
  const repo = bigRepo(8);
  fs.writeFileSync(path.join(repo, 'package.json'),
    '{"name":"x","dependencies":{"left-pad":"^1.0.0"}}\n');
  const files = filesIn(repo).concat(path.join(repo, 'package.json'));
  const options = {
    repoRoot: repo, config: loadConfig(repo), applyBaseline: false, budgetMs: 0,
  };

  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);
  const parallel = await scanFiles(files,
    { ...options, jobs: 2, forceParallel: true }, checkFile);

  const exhausted = (out) => out
    .filter((r) => r.findings.some((f) => f.id === 'true/budget-exhausted'))
    .map((r) => r.relPath);
  assert.deepStrictEqual(exhausted(sequential), files.map((f) => path.basename(f)),
    'the fixture stopped exhausting its budget');
  assert.deepStrictEqual(exhausted(parallel), exhausted(sequential),
    'a worker ran the file to a different budget than this process would have');
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
});

// --- the forked path, exercised as itself ----------------------------------
//
// This repository has fewer tracked files than PARALLEL_MIN_FILES, so its own
// gate never forks and every parity test that reaches the pool by file count is
// really a test of the threshold. These reach it directly, and prove a child
// actually ran rather than assuming it.

// A stand-in for `node` that records every invocation and then runs the real
// one. Proof of forking that does not depend on counting processes.
function recordingNode(dir, body) {
  const marker = path.join(dir, 'forks.log');
  const shim = path.join(dir, 'node-shim');
  fs.writeFileSync(shim,
    `#!/bin/sh\necho fork >> ${JSON.stringify(marker)}\n${body}\n`, { mode: 0o755 });
  return { shim, marker };
}

async function withExecPath(value, fn) {
  const saved = process.execPath;
  Object.defineProperty(process, 'execPath', { value, configurable: true });
  try {
    return await fn();
  } finally {
    Object.defineProperty(process, 'execPath', { value: saved, configurable: true });
  }
}

test('the forked path runs on a small file list when it is asked to', async () => {
  const repo = bigRepo(6);
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };
  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);

  const { shim, marker } = recordingNode(repo, `exec ${JSON.stringify(process.execPath)} "$@"`);
  const parallel = await withExecPath(shim,
    () => scanFiles(files, { ...options, jobs: 3, forceParallel: true }, checkFile));

  assert.strictEqual(fs.readFileSync(marker, 'utf8').trim().split('\n').length, 3,
    'the pool did not fork three workers');
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
});

test('a worker that dies mid-slice loses no file and no finding', async () => {
  const repo = bigRepo(6);
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };
  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);

  // Killed by a signal rather than exited: a crash, not a clean non-zero.
  const { shim, marker } = recordingNode(repo, 'kill -9 $$');
  const parallel = await withExecPath(shim,
    () => scanFiles(files, { ...options, jobs: 3, forceParallel: true }, checkFile));

  assert.ok(fs.existsSync(marker), 'no worker was forked, so none could die');
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
  assert.ok(parallel.some((r) => r.findings.length > 0), 'the fallback lost every finding');
});

// The failure a dead worker never had: a LIVE one that answers nothing. Before
// the watchdog there was no timeout anywhere in the pool, so this hung the
// whole scan for as long as the child stayed up — a gate that never returns,
// which is why this test carries a timeout of its own: without the fix it does
// not fail, it never finishes.
//
// Over the threshold rather than forced, so that the hang is reachable on the
// code this replaces too. The budget is zero only to keep the derived slice
// bound at its startup floor and to make what each file reports deterministic
// — at 1ms it is a coin flip whether the pack starts, and the two paths then
// disagree for a reason that is not the one under test.
test('a worker that hangs is killed, and its slice is scanned here', { timeout: 60000 }, async () => {
  const repo = bigRepo(PARALLEL_MIN_FILES + 10);
  const files = filesIn(repo);
  const options = {
    repoRoot: repo, config: loadConfig(repo), applyBaseline: false, budgetMs: 0,
  };
  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);

  const { shim } = recordingNode(repo, 'exec sleep 120');
  const started = Date.now();
  const parallel = await withExecPath(shim,
    () => scanFiles(files, { ...options, jobs: 3, forceParallel: true }, checkFile));

  assert.ok(Date.now() - started < 30000, 'the hung worker was waited on, not killed');
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
  assert.ok(parallel.some((r) => r.findings.length > 0), 'the fallback lost every finding');
});

// --- --jobs, clamped -------------------------------------------------------
//
// Same semantics as `max_file_bytes` in config.js: under the ceiling honoured,
// over it refused with a warning naming the value AND the ceiling, and anything
// that is not a count refused rather than coerced.

function withStderr(fn) {
  const written = [];
  const saved = process.stderr.write;
  process.stderr.write = (chunk) => { written.push(String(chunk)); return true; };
  try {
    return { value: fn(), stderr: written.join('') };
  } finally {
    process.stderr.write = saved;
  }
}

test('--jobs above the ceiling is refused with a warning, and the ceiling used', () => {
  const { value, stderr } = withStderr(() => clampJobs(9999));
  assert.strictEqual(value, MAX_JOBS);
  assert.match(stderr, /9999/);
  assert.match(stderr, new RegExp(String(MAX_JOBS)));
});

test('--jobs below the ceiling is honoured silently', () => {
  const { value, stderr } = withStderr(() => clampJobs(2));
  assert.strictEqual(value, 2);
  assert.strictEqual(stderr, '');
  assert.strictEqual(withStderr(() => clampJobs(MAX_JOBS)).value, MAX_JOBS);
});

test('--jobs that is not a count is refused rather than coerced', () => {
  for (const bad of [0, -3, 'abc', NaN, Infinity, 0.5, '', true]) {
    const { value, stderr } = withStderr(() => clampJobs(bad));
    assert.strictEqual(value, defaultJobs(), `${String(bad)} was coerced`);
    assert.match(stderr, /--jobs/, `${String(bad)} was accepted in silence`);
  }
  // An absent flag is not a bad value: no warning, and the default.
  const absent = withStderr(() => clampJobs(null));
  assert.strictEqual(absent.value, defaultJobs());
  assert.strictEqual(absent.stderr, '');
});

test('a scan asked for 9999 jobs forks the ceiling, not 9999', async () => {
  const repo = bigRepo(40);
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };
  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);

  const { shim, marker } = recordingNode(repo, `exec ${JSON.stringify(process.execPath)} "$@"`);
  const parallel = await withExecPath(shim, () => withStderr(
    () => scanFiles(files, { ...options, jobs: 9999, forceParallel: true }, checkFile)).value);

  assert.ok(fs.readFileSync(marker, 'utf8').trim().split('\n').length <= MAX_JOBS,
    'the pool forked more workers than the ceiling');
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
});

// The worker used to rebuild the config from .procoder.toml, so a config the
// caller built by hand governed the files this process scanned and not the ones
// a worker did — the same class of divergence as the budget.
test('a worker checks against the caller\'s config, not the one on disk', async () => {
  const repo = bigRepo(PARALLEL_MIN_FILES + 10);
  const files = filesIn(repo);
  const config = loadConfig(repo);
  // An exclusion that exists only in the caller's object: nothing on disk says
  // it, so a worker that reloads the config will keep reporting the rule.
  config.exclude.rules = files.map((f) => (
    { path: path.basename(f), id: 'safe/dynamic-eval' }));
  const options = { repoRoot: repo, config, applyBaseline: false };

  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);
  const parallel = await scanFiles(files,
    { ...options, jobs: 3, forceParallel: true }, checkFile);

  assert.ok(sequential.every((r) => r.findings.every((f) => f.id !== 'safe/dynamic-eval')),
    'the caller-built exclusion stopped working');
  assert.deepStrictEqual(
    parallel.map((r) => [r.relPath, r.findings.map((f) => f.id)]),
    sequential.map((r) => [r.relPath, r.findings.map((f) => f.id)]));
});
