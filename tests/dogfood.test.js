// tests/dogfood.test.js
//
// procoder run against procoder. A tool that exempts itself from its own
// doctrine has already lost the argument, so this is a hard gate: when it
// fails, fix the source. Do not baseline it, and do not widen an exclusion.
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync, spawnSync } = require('child_process');
const path = require('path');

const root = path.join(__dirname, '..');
const CLI = path.join(root, 'bin', 'procoder.js');

// The scan is the whole tracked tree. It used to be a hand-written list of
// directories, which is cherry-picking: whatever was left off never had to
// pass, and a new directory was covered only when someone remembered to add
// it. The list comes from `git ls-files` now, so a file is in the gate the day
// it lands.  // procoder: literal alone/blanket-suppression this paragraph names the marker mechanism in prose
//
// Nothing is held out here any more. The two categories that used to be —
// docs/superpowers/ (planning documents for work already executed) and
// examples/*/before.* (files that violate a rung on purpose) — are covered by
// a .procoderignore next to each, which is the sanctioned instrument and,
// unlike this list, is honoured by the real `procoder check` a user runs. A
// hold-out that duplicated an ignore file would be exactly the stale
// suppression rung 4 forbids, so both were deleted rather than kept "just in
// case"; the test below fails on any entry that has stopped holding anything
// out. The list stays because the next one will be argued for here, with its
// reason next to the path.
const HELD_OUT = [];

// Findings that belong to a file another change owns right now. Unlike
// HELD_OUT, these are temporary, and the test below fails once they go green —
// a hold-out that has stopped holding anything out is exactly the stale
// suppression rung 4 is about.
const PENDING = [
];

const excluded = (file) =>
  HELD_OUT.some((h) => file.startsWith(h.path) && (!h.match || h.match(file)))
  || PENDING.some((p) => file === p.path);

function trackedFiles() {
  return execFileSync('git', ['ls-files'], { cwd: root, encoding: 'utf8' })
    .split('\n').filter(Boolean).filter((file) => !excluded(file));
}

// spawnSync, not execFileSync: the skip lines go to stderr so they survive a
// piped stdout, and the run that has to count them exits 0 — which is exactly
// the case execFileSync gives you no stderr for.
function selfScan(extraTargets = []) {
  const r = spawnSync('node', [CLI, 'check', ...trackedFiles(), ...extraTargets],
    { cwd: root, encoding: 'utf8' });
  return { code: r.status, out: String(r.stdout || ''), err: String(r.stderr || '') };
}

// Every skip the scan announced, counted from its own stderr rather than
// re-derived from the config — the published number has to be the one the tool
// prints, or the paragraph in README.md is describing a different program.
// Two shapes: one line per file when the file was named on the command line
// (which is every excluded file here, since the targets come from git ls-files),
// and one counted line per .procoderignore.
function skipCount(stderr) {
  let skipped = 0;
  for (const line of stderr.split('\n')) {
    const counted = line.match(/procoder: (\d+) files? skipped by /);
    if (counted) skipped += Number(counted[1]);
    else if (/procoder: skipped \S+ .* not checked\./.test(line)) skipped += 1;
  }
  return skipped;
}

// The self-scan's own arithmetic, and the claim README.md makes about it.
//
// "There is no hold-out list" was true and still misleading: a quarter of the
// tree was skipped by [exclude] paths and two .procoderignore files, and a
// reader of that sentence would not have guessed it. The fix is not a
// disclaimer, it is publishing the three numbers — and pinning them here, so
// the day the tree or an exclusion changes, the paragraph fails with it rather
// than quietly describing a scan that no longer exists.
test('the scan reads the number of files README.md says it does', () => {
  const tracked = execFileSync('git', ['ls-files'], { cwd: root, encoding: 'utf8' })
    .split('\n').filter(Boolean).length;
  const { err } = selfScan();
  const skipped = skipCount(err);

  const readme = require('fs').readFileSync(path.join(root, 'README.md'), 'utf8');
  const claim = readme.match(/of \*\*(\d+)\*\* tracked files the scan\s+reads \*\*(\d+)\*\* and skips \*\*(\d+)\*\*/);
  assert.ok(claim, 'README.md no longer states the tracked/read/skipped numbers — restore them');
  const [, saysTracked, saysRead, saysSkipped] = claim.map(Number);

  assert.strictEqual(saysTracked, tracked, `README.md says ${saysTracked} tracked files, git ls-files reports ${tracked}`);
  assert.strictEqual(saysSkipped, skipped, `README.md says ${saysSkipped} files skipped, the scan skipped ${skipped}`);
  assert.strictEqual(saysRead, tracked - skipped, `README.md says ${saysRead} files read, the scan read ${tracked - skipped}`);
});

// The instrument that keeps the numbers above from rotting in the other
// direction: an exclusion still in the list after it stopped holding anything
// back. procoder judges its own [exclude] paths on every verify, so this is the
// tool's own rule applied to the tool's own config, not a second mechanism.
test('every path exclusion procoder sets for itself still holds something back', () => {
  let out = '';
  try {
    out = execFileSync('node', [CLI, 'verify', '--unused-exclusions', '.'],
      { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] });
  } catch (e) {
    out = String(e.stdout || '');
    assert.fail(`procoder's own exclusions are stale:\n${out}`);
  }
  assert.doesNotMatch(out, /exclude nothing|excludes nothing/);
});

test('procoder reports no findings against its own source', () => {
  const { code, out } = selfScan();
  assert.strictEqual(code, 0,
    `procoder fails its own rungs:\n${out}\nFix the source, do not baseline it.`);
});

// A hold-out that no longer holds anything out is a stale suppression, and it
// is how the previous list rotted: entries outlived the findings that put them
// there. Every PENDING path must still report something, so the day the change
// that owns it lands, this test says to delete the entry.
// PENDING has that check below, but HELD_OUT never did: a permanent exemption
// for a path that has gone clean — or that no longer matches any tracked file —
// narrows the gate silently, which is the same rung-4 rot the marker rules are
// about. These entries are meant to be permanent, not unexamined.
test('every hold-out still earns its exemption', () => {
  const all = execFileSync('git', ['ls-files'], { cwd: root, encoding: 'utf8' })
    .split('\n').filter(Boolean);

  for (const h of HELD_OUT) {
    const covered = all.filter((f) => f.startsWith(h.path) && (!h.match || h.match(f)));
    assert.ok(covered.length > 0,
      `HELD_OUT entry ${h.path} matches no tracked file — delete it`);

    const { code } = selfScan(covered.map((f) => path.join(root, f)));
    assert.strictEqual(code, 1,
      `${h.path} is clean now — delete its HELD_OUT entry so the path is gated again`);
  }
});

test('every pending hold-out still has the finding that put it there', () => {
  for (const p of PENDING) {
    const { code } = selfScan([path.join(root, p.path)]);
    assert.strictEqual(code, 1,
      `${p.path} is clean now — delete its PENDING entry so the file is gated again`);
  }
});

// The canary must prove the self-scan actually fails on a planted violation,
// without ever landing inside the tracked tree — an interrupted run must not be
// able to leave a stray file for `git add` to pick up. It is written under the
// OS temp dir instead and passed to `procoder check` as an extra target
// (checkFile accepts any path, not just ones under the repo).
//
// What it plants changed with the engine. It used to be an orphan TODO, caught
// by a rule procoder owned; procoder owns no rules now, so the canary has to be
// something a real analyzer catches, and the temp dir has to carry the two
// things an analyzer needs to answer at all — this repo's eslint config and its
// node_modules. That is not scaffolding around the test, it IS the test: if
// either is missing, procoder reports the file as ungated rather than clean, and
// the assertions below fail exactly as they should.
test('the self-scan is a real gate: a planted violation is reported', () => {
  const fs = require('fs');
  const os = require('os');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-dogfood-'));
  const planted = path.join(dir, 'dogfood-canary.js');
  try {
    fs.copyFileSync(path.join(root, 'eslint.config.mjs'), path.join(dir, 'eslint.config.mjs'));
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'), 'dir');
    fs.writeFileSync(planted,
      'module.exports = (p) => new RegExp(p);\n');
    const { code, out } = selfScan([planted]);
    assert.strictEqual(code, 1, 'a planted weakness must fail the self-scan');
    assert.match(out, /dogfood-canary\.js:1/);
    assert.match(out, /detect-non-literal-regexp/,
      'the analyzer answered, but not with the finding the canary plants');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
