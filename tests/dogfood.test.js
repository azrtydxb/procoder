// tests/dogfood.test.js
//
// procoder run against procoder. A tool that exempts itself from its own
// doctrine has already lost the argument, so this is a hard gate: when it
// fails, fix the source. Do not baseline it, and do not widen an exclusion.
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const path = require('path');

const root = path.join(__dirname, '..');
const CLI = path.join(root, 'bin', 'procoder.js');

// The scan is the whole tracked tree. It used to be a hand-written list of
// directories, which is cherry-picking: whatever was left off never had to
// pass, and a new directory was covered only when someone remembered to add
// it. The list comes from `git ls-files` now, so a file is in the gate the day
// it lands.  // procoder: literal alone/blanket-suppression this paragraph names the marker mechanism in prose
//
// Two categories are held out, and only these two. Each says why, next to the
// path, because an exclusion without a reason is the thing this project spends
// four rungs arguing against.
const HELD_OUT = [
  {
    path: 'docs/superpowers/',
    why: 'planning documents for work already executed — historical by nature, '
      + 'and they quote rule ids and sample violations by the hundred',
  },
  {
    path: 'examples/',
    match: (file) => /\/before\.[^/]+$/.test(file),
    why: 'examples/*/before.* violate a rung on purpose, paired with an after.* '
      + 'that does not. They are instances, not descriptions, so the line marker '
      + 'is the wrong tool — and a .procoder.toml path exclusion is worse, '
      + 'because checkFile() applies it and tests/examples.test.js asserts these '
      + 'files still trip. after.* and the README stay in the scan',
  },
];

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

function selfScan(extraTargets = []) {
  try {
    return {
      code: 0,
      out: execFileSync('node', [CLI, 'check', ...trackedFiles(), ...extraTargets], { cwd: root, encoding: 'utf8' }),
    };
  } catch (e) {
    return { code: e.status, out: String(e.stdout || '') };
  }
}

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
// without ever landing inside the tracked tree — an interrupted run must not
// be able to leave a stray file for `git add` to pick up. It's written under
// the OS temp dir instead and passed to `procoder check` as an extra target
// (checkFile accepts any path, not just ones under the repo), so the scanner
// genuinely evaluates it — this is not a weakened test, just a relocated file.
test('the self-scan is a real gate: a planted violation is reported', () => {
  const fs = require('fs');
  const os = require('os');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-dogfood-'));
  const planted = path.join(dir, 'dogfood-canary.js');
  fs.writeFileSync(planted, '// TODO: no owner, no ticket\nmodule.exports = {};\n');  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  try {
    const { code, out } = selfScan([planted]);
    assert.strictEqual(code, 1, 'a planted orphan TODO must fail the self-scan');  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
    assert.match(out, /dogfood-canary\.js:1 TODO with no owner or ticket/);  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
