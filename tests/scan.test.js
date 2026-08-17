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
const { scanFiles, defaultJobs, PARALLEL_MIN_FILES } = require('../hooks/checks/scan');
const { checkFile } = require('../hooks/checks/run');
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
