// procoder — the deep tier.
//
// CodeQL is the only analyzer procoder runs that can catch a weakness with no
// syntactic tell: an idiomatic `fmt.Sprintf` that happens to be log injection,
// a path that happens to come from argv. It is also the only one that needs a
// database, minutes of wall time, and a build command for compiled languages —
// so it is the tier most likely to be quietly broken and least likely to be
// noticed, because "no findings" is what a healthy run looks like too.
//
// These tests do not run CodeQL. They cover the decisions AROUND it, which is
// where the silent failures live: what gets classified as security, which
// languages are claimed as analysable, and whether a language that cannot be
// analysed is reported or dropped. The one test that does shell out is skipped
// unless a real codeql is on PATH.

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  deepScan, hasCodeql, languagesIn, findingsFrom, AUTO_LANGUAGES, BUILT_LANGUAGES,
} = require('../hooks/checks/codeql');

const HAS_CODEQL = hasCodeql();

function tempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-codeql-'));
}

// A SARIF document shaped like CodeQL's, with the two tag styles that decide the
// rung: a query tagged with a CWE, and one tagged neither security nor CWE.
function sarif(results, rules) {
  return JSON.stringify({
    runs: [{
      tool: { driver: { rules } },
      results,
    }],
  });
}

test('a query tagged with a CWE is rung 1, and carries the CWE into the message', () => {
  const dir = tempDir();
  const file = path.join(dir, 'a.sarif');
  fs.writeFileSync(file, sarif(
    [{
      ruleId: 'go/path-injection',
      message: { text: 'This path depends on a user-provided value.' },
      locations: [{ physicalLocation: { artifactLocation: { uri: 'src/a.go' }, region: { startLine: 12 } } }],
    }],
    [{ id: 'go/path-injection', properties: { tags: ['security', 'external/cwe/cwe-022'] } }],
  ));
  const out = findingsFrom(file, dir);
  const list = out.get(path.join(dir, 'src/a.go'));
  assert.ok(list, 'the finding was not attributed to its file');
  assert.strictEqual(list[0].rung, 'SAFE');
  assert.strictEqual(list[0].id, 'safe/codeql:go/path-injection');
  assert.strictEqual(list[0].line, 12);
  assert.match(list[0].message, /CWE-22:/);
});

// The reason this test exists: the suite is `security-and-quality`, which
// contains maintainability queries by definition. Filing those at rung 1 would
// put a missing-override warning beside a path traversal, and rung 1 would stop
// meaning anything.
test('a quality query is rung 2, not rung 1', () => {
  const dir = tempDir();
  const file = path.join(dir, 'q.sarif');
  fs.writeFileSync(file, sarif(
    [{
      ruleId: 'go/unused-variable',
      message: { text: 'Variable is never read.' },
      locations: [{ physicalLocation: { artifactLocation: { uri: 'src/b.go' }, region: { startLine: 3 } } }],
    }],
    [{ id: 'go/unused-variable', properties: { tags: ['maintainability'] } }],
  ));
  const list = findingsFrom(file, dir).get(path.join(dir, 'src/b.go'));
  assert.strictEqual(list[0].rung, 'TRUE');
  assert.strictEqual(list[0].id, 'true/codeql:go/unused-variable');
});

test('a security-tagged query with no CWE is still rung 1', () => {
  const dir = tempDir();
  const file = path.join(dir, 's.sarif');
  fs.writeFileSync(file, sarif(
    [{
      ruleId: 'js/weak-cryptographic-algorithm',
      message: { text: 'Weak algorithm.' },
      locations: [{ physicalLocation: { artifactLocation: { uri: 'a.js' }, region: { startLine: 1 } } }],
    }],
    [{ id: 'js/weak-cryptographic-algorithm', properties: { tags: ['security'] } }],
  ));
  assert.strictEqual(findingsFrom(file, dir).get(path.join(dir, 'a.js'))[0].rung, 'SAFE');
});

test('an unreadable SARIF yields no findings rather than throwing', () => {
  const dir = tempDir();
  const file = path.join(dir, 'broken.sarif');
  fs.writeFileSync(file, 'not json');
  assert.strictEqual(findingsFrom(file, dir).size, 0);
});

test('languagesIn splits what CodeQL can build alone from what it cannot', () => {
  const { auto, built } = languagesIn(['a.py', 'b.go', 'c.ts', 'd.c', 'E.java', 'readme.md']);
  assert.deepStrictEqual([...auto].sort(), ['go', 'javascript', 'python']);
  assert.deepStrictEqual([...built].sort(), ['cpp', 'java']);
});

test('every extension in either table maps to a language CodeQL names', () => {
  const known = new Set(['python', 'javascript', 'ruby', 'go', 'cpp', 'java', 'csharp']);
  for (const [ext, language] of [...AUTO_LANGUAGES, ...BUILT_LANGUAGES]) {
    assert.ok(known.has(language), `${ext} maps to ${language}, which CodeQL does not call that`);
  }
});

// The failure this tier is most likely to have in the field, and the one that
// looks exactly like success: a compiled language with no build command. CodeQL
// cannot extract it, procoder cannot guess the build, and reporting nothing
// would say "C++ is clean" about a language nobody analysed.
test('a compiled language with no build command is reported, not skipped', () => {
  const dir = tempDir();
  fs.writeFileSync(path.join(dir, 'a.cpp'), 'int main(){}\n');
  const { skipped } = deepScan([path.join(dir, 'a.cpp')], { repoRoot: dir, buildCommand: null });
  if (!HAS_CODEQL) {
    assert.ok(skipped.some((s) => /not installed/.test(s.why)));
    return;
  }
  const cpp = skipped.find((s) => s.language === 'cpp');
  assert.ok(cpp, 'a language that could not be analysed vanished from the report');
  assert.match(cpp.why, /watching a build/);
  assert.match(cpp.fix, /build_command/);
});

test('a missing codeql is a reported gap, never an empty clean result', () => {
  const dir = tempDir();
  fs.writeFileSync(path.join(dir, 'a.py'), 'x = 1\n');
  const saved = process.env.PATH;
  process.env.PATH = '/nonexistent';
  try {
    const { version, skipped, findings } = deepScan([path.join(dir, 'a.py')], { repoRoot: dir });
    assert.strictEqual(version, null);
    assert.strictEqual(findings.size, 0);
    assert.ok(skipped.length, 'no codeql, no findings, and nothing said — that reads as clean');
    assert.match(skipped[0].fix, /install/i);
  } finally {
    process.env.PATH = saved;
  }
});

// The flag the whole tier turns on. Asserted against the source rather than a
// run, because a run costs minutes and the thing worth pinning is that nobody
// quietly makes it configurable: CodeQL's default threat model treats argv as
// trusted, which took recall from 53% to 7% on CWEval.
test('--threat-model=all is not configurable', () => {
  const source = fs.readFileSync(
    path.join(__dirname, '..', 'hooks', 'checks', 'codeql.js'), 'utf8');
  assert.match(source, /'--threat-model=all'/);
  assert.doesNotMatch(source, /threatModel\s*[=:]/,
    'the threat model became a setting — see the header for why it must not be');
  assert.match(source, /security-and-quality/,
    'the suite changed; re-measure before trusting the numbers in the header');
});

test('procoder deep is reachable from the CLI', () => {
  const cli = fs.readFileSync(path.join(__dirname, '..', 'bin', 'procoder.js'), 'utf8');
  assert.match(cli, /command === 'deep'/);
  assert.match(cli, /procoder deep/, 'deep is dispatched but not in the usage text');
});
