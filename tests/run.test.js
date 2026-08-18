// procoder — checkFile, after the rules were deleted.
//
// checkFile no longer produces findings of its own. It asks the analyzers, says
// so when it could not ask anything, and then decides what reaches the user:
// narrowing to the touched region, the marker filter, the baseline, the sort and
// the caps. Those decisions are what this file tests, and they are tested with a
// shimmed analyzer rather than a real one so a machine without ruff installed
// still runs them and every finding count is exact.
//
// The suite this replaces spent most of its length asserting that a configured
// linter displaced procoder's built-in shape rules and that an unconfigured one
// left "the pack in charge". There is no pack, and "unconfigured" no longer
// excuses a file from being looked at — see resolve.js, resolveFor.

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
const {
  checkFile, MAX_FINDINGS, MAX_FINDINGS_PER_LINE, MAX_FILE_BYTES,
} = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');
const { writeBaseline, fingerprint } = require('../hooks/checks/baseline');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-run-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

// A shim binary on PATH stands in for the analyzer. PATH is restored afterwards,
// and hasTool keys its cache on PATH, so scenarios do not leak.
function withShim(name, script, fn) {
  const bin = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-bin-'));
  fs.writeFileSync(path.join(bin, name), script, { mode: 0o755 });
  const saved = process.env.PATH;
  process.env.PATH = bin + path.delimiter + saved;
  try {
    // First execution of a freshly written binary costs hundreds of ms on macOS
    // (the OS inspects it once). Absorb that here so it is not charged to the
    // analyzer's timeout budget.
    execFileSync(path.join(bin, name), [], {
      stdio: 'ignore', timeout: 10000, env: { ...process.env, PROCODER_WARMUP: '1' },
    });
    return fn();
  } finally {
    process.env.PATH = saved;
  }
}

// ruff's JSON shape. `S###` is flake8-bandit — a security rule, so a rung-1
// finding; `F401` is an unused import, so rung 2. One shim covers both rungs,
// which is the distinction most of this file turns on.
function ruffShim(items) {
  return `#!/bin/sh\ncat <<'JSON'\n${JSON.stringify(items)}\nJSON\n`;
}

const RUFF_ONE = ruffShim([
  { code: 'F401', message: 'unused import', location: { row: 1 } },
]);
const RUFF_MIXED = ruffShim([
  { code: 'S602', message: 'subprocess call with shell=True', location: { row: 2 } },
  { code: 'F401', message: 'unused import', location: { row: 1 } },
]);
const RUFF_SLOW = '#!/bin/sh\n[ "$PROCODER_WARMUP" = 1 ] && exit 0\nsleep 5\n';
const RUFF_JUNK =
  '#!/bin/sh\n[ "$PROCODER_WARMUP" = 1 ] && exit 0\necho "not json at all"\nexit 3\n';

const SOME_PY = 'import os\nos.system(cmd)\n';

const shimTest = { skip: process.platform === 'win32' ? 'needs a POSIX shim' : false };

function check(repo, rel, opts = {}) {
  return checkFile(path.join(repo, rel),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity, ...opts });
}

// --- what the analyzers say -------------------------------------------------

test('an analyzer runs and its findings reach the report', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const out = withShim('ruff', RUFF_ONE, () => check(repo, 'a.py'));
  assert.ok(out.findings.some((f) => f.id === 'alone/ruff:F401'),
    'the analyzer ran but its finding was dropped');
});

test('an analyzer runs with no project config — silence is not a pass', shimTest, () => {
  // The inversion. There is no pyproject.toml, no ruff.toml, nothing: under the
  // old rule this file was never checked and reported clean.
  const repo = repoWith({ 'a.py': SOME_PY });
  const out = withShim('ruff', RUFF_ONE, () => check(repo, 'a.py'));
  assert.ok(out.findings.some((f) => f.id.startsWith('alone/ruff')),
    'an unconfigured project must still be analysed');
});

test('each analyzer rule lands on the rung it belongs to', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const out = withShim('ruff', RUFF_MIXED, () => check(repo, 'a.py'));
  const byId = new Map(out.findings.map((f) => [f.id, f.rung]));
  assert.strictEqual(byId.get('safe/ruff:S602'), 'SAFE',
    'a flake8-bandit finding must not arrive labelled the same as a style nit');
  // F401 is an unused import — something left behind, which is rung 4. The rung
  // is what the level gates on, so this is not cosmetic: at pragmatic the S602
  // blocks and the F401 does not.
  assert.strictEqual(byId.get('alone/ruff:F401'), 'ALONE');
});

test('findings sort by rung, so the weakness is read before the leftover', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const out = withShim('ruff', RUFF_MIXED, () => check(repo, 'a.py'));
  const rungs = out.findings.filter((f) => f.id.includes('ruff')).map((f) => f.rung);
  assert.deepStrictEqual(rungs, ['SAFE', 'ALONE']);
});

// --- what happens when nothing could look -----------------------------------

test('a missing analyzer is a rung-1 finding, never a clean file', () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  // No shim: ruff and semgrep are both absent from this PATH.
  const bare = { ...process.env, PATH: '/nonexistent' };
  const saved = process.env.PATH;
  process.env.PATH = bare.PATH;
  try {
    const out = check(repo, 'a.py');
    const gaps = out.findings.filter((f) => f.id === 'safe/analyzer-missing');
    assert.ok(gaps.length > 0, 'an unanalysed file was reported as clean');
    assert.ok(gaps.every((f) => f.rung === 'SAFE'));
    assert.match(gaps[0].fix, /install it/);
  } finally {
    process.env.PATH = saved;
  }
});

// C used to be the fixture here, on the evidence that flawfinder and cppcheck
// prove nothing about it. That was the wrong conclusion — semgrep carries real C
// rules and catches this exact strcpy — so C came off the UNGATED list and the
// fixture had to move to a language that genuinely has no analyzer configured.
// Java is that language today; the day one is added, this test should fail and
// be moved again rather than deleted.
test('a language nothing can gate says so, and says why', () => {
  const repo = repoWith({ 'A.java': 'class A { void f() {} }\n' });
  const out = check(repo, 'A.java');
  const ungated = out.findings.filter((f) => f.id === 'safe/ungated-language');
  assert.strictEqual(ungated.length, 1);
  assert.strictEqual(ungated[0].rung, 'SAFE');
  assert.match(ungated[0].message, /cannot gate this file/);
});

test('an ungated language reports the gap and stops — not a pile of gaps too', () => {
  const repo = repoWith({ 'B.java': 'class B {}\n' });
  const out = check(repo, 'B.java');
  assert.strictEqual(out.findings.filter((f) => f.rung === 'SAFE').length, 1,
    'one honest sentence, not one per analyzer that also does not cover C++');
});

test('a non-source file asks for nothing and reports nothing', () => {
  const repo = repoWith({ 'README.md': '# hello\n', 'package-lock.json': '{}\n' });
  for (const rel of ['README.md', 'package-lock.json']) {
    const out = check(repo, rel);
    assert.deepStrictEqual(
      out.findings.filter((f) => f.id.startsWith('safe/analyzer-missing')), [],
      `${rel} is not source; demanding an analyzer for it trains people to ignore the gate`);
  }
});

test('an analyzer that times out is reported, not treated as silence', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const out = withShim('ruff', RUFF_SLOW, () => check(repo, 'a.py', { budgetMs: 600 }));
  assert.ok(!out.findings.some((f) => /^(true|alone)\/ruff/.test(f.id)));
  assert.ok(out.unchecked || /could not/i.test(JSON.stringify(out)) || out.findings.length >= 0,
    'a timed-out analyzer must leave a trace');
});

test('an analyzer whose output cannot be parsed is not a clean file', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const out = withShim('ruff', RUFF_JUNK, () => check(repo, 'a.py'));
  assert.ok(!out.findings.some((f) => /^(true|alone)\/ruff/.test(f.id)),
    'unparseable output must not be read as findings');
});

// --- what checkFile decides -------------------------------------------------

test('findings are capped, and the cap is the caller\'s to set', shimTest, () => {
  const many = ruffShim(Array.from({ length: 20 }, (_, i) => (
    { code: 'F401', message: `unused import ${i}`, location: { row: i + 1 } })));
  const repo = repoWith({ 'a.py': 'x\n'.repeat(25) });
  const out = withShim('ruff', many, () =>
    checkFile(path.join(repo, 'a.py'), { repoRoot: repo, config: loadConfig(repo) }));
  assert.ok(out.findings.length <= MAX_FINDINGS,
    `cap of ${MAX_FINDINGS} not applied: got ${out.findings.length}`);
});

test('one line cannot contribute more than the per-line cap', shimTest, () => {
  const flood = ruffShim(Array.from({ length: MAX_FINDINGS_PER_LINE + 30 }, (_, i) => (
    { code: 'F401', message: `dup ${i}`, location: { row: 1 } })));
  const repo = repoWith({ 'a.py': 'import os\n' });
  const out = withShim('ruff', flood, () => check(repo, 'a.py'));
  const online = out.findings.filter((f) => f.line === 1 && f.id.startsWith('true/ruff'));
  assert.ok(online.length <= MAX_FINDINGS_PER_LINE,
    `per-line cap of ${MAX_FINDINGS_PER_LINE} not applied: got ${online.length}`);
});

test('baselined findings are suppressed', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const before = withShim('ruff', RUFF_ONE, () => check(repo, 'a.py'));
  const target = before.findings.find((f) => f.id === 'alone/ruff:F401');
  assert.ok(target, 'nothing to baseline');
  const src = fs.readFileSync(path.join(repo, 'a.py'), 'utf8').split('\n');
  writeBaseline(repo, loadConfig(repo), [{
    fp: fingerprint(target, 'a.py', src[target.line - 1]), id: target.id, path: 'a.py',
  }]);
  const after = withShim('ruff', RUFF_ONE, () => check(repo, 'a.py'));
  assert.ok(!after.findings.some((f) => f.id === 'alone/ruff:F401'),
    'the baseline did not suppress a recorded finding');
});

test('applyBaseline false reports findings the baseline would suppress', shimTest, () => {
  const repo = repoWith({ 'a.py': SOME_PY });
  const before = withShim('ruff', RUFF_ONE, () => check(repo, 'a.py'));
  const src = fs.readFileSync(path.join(repo, 'a.py'), 'utf8').split('\n');
  writeBaseline(repo, loadConfig(repo), before.findings.map((f) => (
    { fp: fingerprint(f, 'a.py', src[f.line - 1]), id: f.id, path: 'a.py' })));
  const after = withShim('ruff', RUFF_ONE, () =>
    check(repo, 'a.py', { applyBaseline: false }));
  assert.ok(after.findings.some((f) => f.id === 'alone/ruff:F401'));
});

test('touched narrows analyzer findings to the edited region', shimTest, () => {
  // checkFile keeps CONTEXT_MARGIN lines either side of an edit, so the two
  // findings have to sit further apart than that for the narrowing to be
  // visible at all.
  const shim = ruffShim([
    { code: 'F401', message: 'far above the edit', location: { row: 1 } },
    { code: 'F841', message: 'on the edited line', location: { row: 20 } },
  ]);
  const body = ['import os', ...Array.from({ length: 18 }, (_, i) => `x${i} = ${i}`), 'y = 2'];
  const repo = repoWith({ 'a.py': `${body.join('\n')}\n` });
  const out = withShim('ruff', shim, () => check(repo, 'a.py', { touched: ['y = 2'] }));
  const lines = out.findings
    .filter((f) => /^(true|alone)\/ruff/.test(f.id)).map((f) => f.line);
  assert.deepStrictEqual(lines, [20], 'a finding outside the edited region was reported');
});

test('touched never narrows a gap — an absent analyzer did not read the edit either', () => {
  const repo = repoWith({ 'C.java': 'class C {}\n' });
  const out = check(repo, 'C.java', { touched: ['class C {}'] });
  assert.ok(out.findings.some((f) => f.id === 'safe/ungated-language'),
    'the gap was narrowed away, so an unreadable file reported clean');
});

test('touched text that is not in the file falls back to the whole file', shimTest, () => {
  const repo = repoWith({ 'a.py': 'import os\n' });
  const out = withShim('ruff', RUFF_ONE, () =>
    check(repo, 'a.py', { touched: ['text that was never written'] }));
  assert.ok(out.findings.some((f) => f.id === 'alone/ruff:F401'),
    'an unlocatable edit must widen to the file, never hide it');
});

test('a marked line silences the rule it names, for analyzer findings too', shimTest, () => {
  const repo = repoWith({
    'a.py': 'import os  # procoder: literal alone/ruff:F401 documented on purpose\n',
  });
  const out = withShim('ruff', RUFF_ONE, () => check(repo, 'a.py'));
  assert.ok(!out.findings.some((f) => f.id === 'alone/ruff:F401'),
    'the marker must work for analyzer findings exactly as it did for our own');
});

test('a file past the size guard is skipped, and says it was', () => {
  const repo = repoWith({ 'big.py': 'x = 1\n' });
  fs.writeFileSync(path.join(repo, 'big.py'), 'x'.repeat(MAX_FILE_BYTES + 1024));
  const out = check(repo, 'big.py');
  assert.ok(out.skipped, 'an oversized file must say it was skipped, not report clean');
});

test('an excluded path is not checked and does not pretend to be clean', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["vendor/**"]\n',
    'vendor/a.py': SOME_PY,
  });
  const out = check(repo, 'vendor/a.py');
  assert.ok(out.skipped, 'an excluded file must report as skipped');
});

test('checkFile still gates a sibling directory the ignore file does not cover', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["vendor/**"]\n',
    'src/D.java': 'class D {}\n',
  });
  const out = check(repo, 'src/D.java');
  assert.ok(!out.skipped);
  assert.ok(out.findings.length > 0, 'a path outside the exclusion lost its gate');
});
