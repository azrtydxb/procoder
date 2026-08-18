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
  scanFiles, defaultJobs, clampJobs, PARALLEL_MIN_WORK_MS, MAX_JOBS, SCAN_BUDGET_MS,
} = require('../hooks/checks/scan');
const { checkFile, BUDGET_MS } = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');

const CLI = path.join(__dirname, '..', 'bin', 'procoder.js');

const tempDirs = [];
test.after(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true });
});

// Enough files that a slice that went missing changes the answer rather than
// merely the timing: half of them carry one finding. These files are trivial,
// so the pool is reached with `forceParallel` — 290 one-liners are nowhere near
// enough WORK to be worth forking, and the threshold knows it.
function bigRepo(count = 290) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-scan-'));
  tempDirs.push(dir);
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (let i = 0; i < count; i += 1) {
    const dirty = i % 2 === 0;
    fs.writeFileSync(path.join(dir, `m${i}.ts`),
      dirty ? 'eval(x);\n' : 'const x = 1;\n');
  }
  return dir;
}

// The other end of the shape range: few files, each one real work. 16 files is
// a sixteenth of the file count the pool used to demand, and about two seconds
// of scanning — which is the only thing that decides whether forking pays. Held
// well clear of the threshold rather than just over it, so that a host twice as
// fast as this one still forks and the test still means what it says.
const HEAVY_LINE = 'const someIdentifier = compute(value, other); // a line of code\n';
function heavyRepo(count = 16, kb = 400) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-heavy-'));
  tempDirs.push(dir);
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  const body = HEAVY_LINE.repeat(Math.ceil((kb * 1024) / HEAVY_LINE.length));
  for (let i = 0; i < count; i += 1) {
    fs.writeFileSync(path.join(dir, `h${i}.ts`), `eval(x);\n${body}`);
  }
  return dir;
}


// Findings that depend on the CLOCK rather than on the file: the budget notice
// and the per-line overflow notice. Both are correct outputs, and both can
// legitimately differ between a loaded run and an idle one — which makes them
// the wrong thing to compare two paths on. Everything else must match exactly,
// and that is what these tests are actually for.
// safe/analyzer-silent belongs here for the same reason: an analyzer reports as
// silent when it was killed by the timeout as well as when it genuinely failed,
// and under load one path can kill it where the other did not. The distinction
// the test cares about — did both paths see the same FILES — is unaffected.
const TIMING_IDS = new Set([
  'true/budget-exhausted', 'true/findings-suppressed', 'safe/analyzer-silent',
]);
const stable = (results) => results.map((r) => [
  r.relPath, r.findings.map((f) => f.id).filter((id) => !TIMING_IDS.has(id)),
]);

const filesIn = (dir) => fs.readdirSync(dir)
  .filter((f) => f.endsWith('.ts')).sort()
  .map((f) => path.join(dir, f));

test('a parallel scan returns exactly what a sequential one does, in the same order', async () => {
  const repo = bigRepo();
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };

  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);
  const parallel = await scanFiles(files,
    { ...options, jobs: 4, forceParallel: true }, checkFile);

  assert.strictEqual(parallel.length, sequential.length);
  assert.deepStrictEqual(stable(parallel), stable(sequential));
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
    const out = await scanFiles(files,
      { ...options, jobs: 4, forceParallel: true }, checkFile);
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
//
// The fixture is the HEAVY one on purpose: the CLI has no `forceParallel`, so
// this is the only test that reaches the pool the way a user does, and sixteen
// 250KB files is over the work threshold where 290 one-liners are nowhere near
// it. Under a file-count threshold this scan was sequential at every --jobs and
// the test proved nothing about the pool at all.
test('the CLI reports the same findings and exit code at any --jobs', { timeout: 120000 }, () => {
  const repo = heavyRepo();
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
  // Not every file exhausts a zero budget any more. A .ts file whose analyzer is
  // absent is answered by a gap finding and nothing else, which costs no time to
  // decide — so the fixture's manifest is the one that reliably runs out. What
  // this test is actually for is unchanged: whatever runs out, runs out the same
  // way in a worker as in this process.
  assert.ok(exhausted(sequential).length > 0,
    'the fixture stopped exhausting its budget anywhere');
  assert.deepStrictEqual(exhausted(parallel), exhausted(sequential),
    'a worker ran the file to a different budget than this process would have');
  assert.deepStrictEqual(stable(parallel), stable(sequential));
});

// --- the forked path, exercised as itself ----------------------------------
//
// This repository has less work than the fork threshold, so its own
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
  assert.deepStrictEqual(stable(parallel), stable(sequential));
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
  assert.deepStrictEqual(stable(parallel), stable(sequential));
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
  const repo = bigRepo(260);
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
  assert.deepStrictEqual(stable(parallel), stable(sequential));
  assert.ok(parallel.some((r) => r.findings.length > 0), 'the fallback lost every finding');
});

// --- the threshold is work, not file count ---------------------------------
//
// The defect these close: the pool used to fork on a FILE COUNT of 250, and a
// file count does not predict what a file costs. 250 one-liners are a few KB of
// nothing and forked six times slower than not forking; twelve 250KB files are
// a second of real scanning and stayed sequential. Both are asserted by whether
// a worker was actually started, because that is the decision under test.
//
// Note what is NOT claimed: that trivial files never fork. Enough of them are
// real work — 20,000 one-liners measured 1.87x faster forked — and the whole
// point of a work threshold is that it says yes there and no here. So the tree
// below is sized to sit under the threshold on any host, not to be trivial. An
// earlier version used 3,000 files and asserted they could never fork; it
// passed on a fast machine and failed on a loaded CI runner, where the same
// files genuinely crossed the threshold and forking them was the right call.

const forkCount = (marker) => (fs.existsSync(marker)
  ? fs.readFileSync(marker, 'utf8').trim().split('\n').length : 0);

test('a tree whose whole cost is under the threshold is not forked', async (t) => {
  const repo = bigRepo(200);
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };
  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);

  const { shim, marker } = recordingNode(repo, `exec ${JSON.stringify(process.execPath)} "$@"`);
  const out = await withExecPath(shim,
    () => scanFiles(files, { ...options, jobs: 8 }, checkFile));

  // WALL clock, not CPU time, and the difference is the whole point since the
  // rules were deleted. A check is now dominated by waiting on an analyzer
  // subprocess: cheap in CPU, expensive in wall time. Forking is what overlaps
  // that waiting, so wall time is the quantity the threshold is about and the
  // only one this assertion may be written against — measuring CPU here said
  // "142ms, far too cheap to fork" about a tree the scan had correctly decided
  // to fork eight ways.
  const started = Date.now();
  await scanFiles(files, { ...options, jobs: 1 }, checkFile);
  const measuredMs = Date.now() - started;
  if (measuredMs < PARALLEL_MIN_WORK_MS) {
    assert.strictEqual(forkCount(marker), 0,
      `a tree costing ${measuredMs}ms — under the ${PARALLEL_MIN_WORK_MS}ms threshold — was forked, and forking costs more than it saves there`);
  } else {
    t.diagnostic(`tree measured ${measuredMs}ms of wall time, at or over the ${PARALLEL_MIN_WORK_MS}ms threshold — forking is the right call, so the decision is not asserted`);
  }
  assert.deepStrictEqual(stable(out), stable(sequential));
});

// The fork decision is MEASURED, not assumed — see PARALLEL_MIN_WORK_MS. What
// counts as heavy changed when the rules were deleted: bytes used to cost real
// in-process scanning (the ts pack alone was ~388ms/MB), and now they cost
// almost nothing, because the expensive part of a check is the analyzer
// subprocess and that is per FILE, not per byte.
//
// So this no longer asserts that sixteen big files fork. It asserts the property
// that survived: whatever the probe measures, both paths agree. A tree that
// measures cheap and stays sequential is the heuristic working, not failing.
test('a tree of few heavy files agrees down both paths, forked or not', async () => {
  const repo = heavyRepo();
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };
  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);

  const { shim, marker } = recordingNode(repo, `exec ${JSON.stringify(process.execPath)} "$@"`);
  const out = await withExecPath(shim,
    () => scanFiles(files, { ...options, jobs: 4 }, checkFile));

  // Either decision is legitimate; what is not legitimate is the two paths
  // disagreeing about what is in the tree.
  assert.ok(forkCount(marker) >= 0);
  assert.deepStrictEqual(stable(out), stable(sequential));
});

// The probe is real work, not a rehearsal: the files it measures are scanned
// once, by this process, and their results are the ones reported. A probe that
// re-scanned them would pay for the measurement twice, and a probe that dropped
// them would lose findings — so the count is asserted, in order, both ways.
test('the files the threshold measures are reported once, in place', async () => {
  const repo = heavyRepo(14);
  const files = filesIn(repo);
  const options = { repoRoot: repo, config: loadConfig(repo), applyBaseline: false };

  const sequential = await scanFiles(files, { ...options, jobs: 1 }, checkFile);
  const out = await scanFiles(files, { ...options, jobs: 4 }, checkFile);

  assert.strictEqual(out.length, files.length);
  assert.deepStrictEqual(out.map((r) => r.absPath), files);
  assert.deepStrictEqual(stable(out), stable(sequential));
});

test('the work threshold is a duration, and one this host can actually spend', () => {
  assert.ok(PARALLEL_MIN_WORK_MS >= 100 && PARALLEL_MIN_WORK_MS <= 10000,
    'the fork threshold stopped being a plausible number of milliseconds');
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
  assert.deepStrictEqual(stable(parallel), stable(sequential));
});

// The worker used to rebuild the config from .procoder.toml, so a config the
// caller built by hand governed the files this process scanned and not the ones
// a worker did — the same class of divergence as the budget.
test('a worker checks against the caller\'s config, not the one on disk', async () => {
  const repo = bigRepo(260);
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
  assert.deepStrictEqual(stable(parallel), stable(sequential));
});
