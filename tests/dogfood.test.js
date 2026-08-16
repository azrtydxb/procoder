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
// The scan used to be `hooks bin scripts` — the subset of the repo that had no
// meta-text in it. That is cherry-picking away the tool's own worst weakness:
// every other tracked directory failed only because it *describes* patterns,
// and describing one reads to a regex exactly like committing one. The
// `procoder: literal` line marker fixed that, so those directories are in now.
//
// What is still out, and why. Each is a specific finding, not a category:
//
// `skills/` — skills/procoder/SKILL.md:3 (alone/deprecated-no-trigger, the
//   frontmatter description listing "deprecated" as a trigger word) and :202
//   (alone/blanket-suppression, the paragraph teaching what a blanket
//   suppression looks like). Both are meta-text and both are one line marker
//   away; the change that added the marker was not allowed to edit the
//   doctrine file. Add the two markers and add 'skills' here.
//
// `tests/` — four findings, none of them meta-text, all pre-dating the marker:
//   tests/mcp.test.js:1 (nesting depth 5) and :82 (47-line function),
//   tests/manifest.test.js:1 (nesting depth 4) are real rung-3 debt in test
//   helpers and want refactoring, not marking. tests/checks-config.test.js:54
//   reports an 88-line function that is 10 lines long: the file contains the
//   glob "**/*.generated.ts" inside a string, and shape.js's comment stripper
//   reads the `/*` in it as the start of a block comment and swallows 60 lines
//   of braces. That is a second false-positive class, in the shape scanner
//   rather than in the marker rules, and it is fixed there or not at all.
//
// `examples/` — examples/*/before.ts are violations on purpose, paired with an
//   after.ts. They are instances, not descriptions, so the marker is the wrong
//   tool and marking them would make the teaching material lie.
//
// `procoder-mcp/` — procoder-mcp/server.js:24 is a genuine true/swallowed-error
//   and package.json a genuine safe/missing-lockfile. Real findings, out of
//   scope for the change that widened this list.
//
// `docs/` — 57 findings, all meta-text: planning documents that quote the
//   engine's own rule ids and sample violations. Marking them is mechanical and
//   is the obvious next step; the change that added the marker did not own that
//   tree.
const TARGETS = ['hooks', 'bin', 'scripts', 'commands', '.procoder.toml', 'README.md', 'CHANGELOG.md'];

function selfScan(extraTargets = []) {
  try {
    return {
      code: 0,
      out: execFileSync('node', [CLI, 'check', ...TARGETS, ...extraTargets], { cwd: root, encoding: 'utf8' }),
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
