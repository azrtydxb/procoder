const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
const {
  checkFile, MAX_FINDINGS_PER_LINE, MAX_FILE_BYTES, BUDGET_MS,
} = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');
const { writeBaseline, fingerprint } = require('../hooks/checks/baseline');
const { finding } = require('../hooks/checks/finding');
// Every budget below times an in-process, synchronous scan, so it is measured
// in CPU milliseconds: `node --test` runs the test files concurrently, and a
// wall-clock budget scores how loaded the machine is as much as how expensive
// checkFile is. See tests/perf-guard.js.
const { cpuMs } = require('./perf-guard');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-run-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

// A shim binary on PATH stands in for the project's linter. PATH is restored
// afterwards, and hasTool keys its cache on PATH, so scenarios do not leak.
function withShim(name, script, fn) {
  const bin = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-bin-'));
  fs.writeFileSync(path.join(bin, name), script, { mode: 0o755 });
  const saved = process.env.PATH;
  process.env.PATH = bin + path.delimiter + saved;
  try {
    // First execution of a freshly written binary costs hundreds of ms on macOS
    // (the OS inspects it once). Absorb that here so it is not charged to the
    // tool's timeout budget.
    execFileSync(path.join(bin, name), [], {
      stdio: 'ignore', timeout: 10000, env: { ...process.env, PROCODER_WARMUP: '1' },
    });
    return fn();
  } finally {
    process.env.PATH = saved;
  }
}

const RUFF_OK = '#!/bin/sh\necho \'[{"code":"F401","message":"unused import","location":{"row":1}}]\'\n';
const RUFF_SLOW = '#!/bin/sh\n[ "$PROCODER_WARMUP" = 1 ] && exit 0\nsleep 5\n';
const UNSAFE_PY = 'import os\nos.system("rm " + user_input)\neval(payload)\ndef f(a,b,c,d,e,g,h):\n    return 1\n';
const CONFIGURED = '[project]\nname = "x"\n\n[tool.ruff]\nline-length = 100\n';
const NOT_CONFIGURED = '[project]\nname = "x"\n';

const shimTest = { skip: process.platform === 'win32' ? 'needs a POSIX shim' : false };

// One minified line of the given size — the input the shape scanners are
// quadratic on, and the one a leaked credential hides in.
function minifiedLine(bytes) {
  let line = '';
  while (line.length < bytes) line += `function f${line.length}(a,b){return a&&b?a:b;}`;
  return line;
}

test('a configured linter never displaces the SAFE rung', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const out = withShim('ruff', RUFF_OK, () =>
    checkFile(path.join(repo, 'a.py'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/shell-injection'), 'shell injection was deferred to ruff');
  assert.ok(ids.includes('safe/dynamic-eval'), 'eval was deferred to ruff');
  assert.ok(ids.some((id) => id.startsWith('true/ruff')), 'the configured linter did not run');
});

test('a configured linter does displace the built-in shape rules', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const config = loadConfig(repo);
  const withTool = withShim('ruff', RUFF_OK, () =>
    checkFile(path.join(repo, 'a.py'), { repoRoot: repo, config, maxFindings: Infinity }));
  assert.ok(!withTool.findings.some((f) => f.id.startsWith('obvious/')),
    'shape rules should defer to the project linter');

  const alone = checkFile(path.join(repo, 'a.py'),
    { repoRoot: repo, config, maxFindings: Infinity });
  assert.ok(alone.findings.some((f) => f.id === 'obvious/too-many-params'),
    'shape rules should run when no linter is configured');
});

test('a present but unconfigured linter leaves the pack in charge', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': NOT_CONFIGURED, 'a.py': UNSAFE_PY });
  const out = withShim('ruff', RUFF_OK, () =>
    checkFile(path.join(repo, 'a.py'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(!ids.some((id) => id.startsWith('true/ruff')), 'ran a linter the project has not configured');
  assert.ok(ids.includes('safe/shell-injection'));
});

test('a configured but absent linter leaves the pack in charge', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const out = checkFile(path.join(repo, 'a.py'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  const ids = out.findings.map((f) => f.id);
  assert.ok(!ids.some((id) => id.startsWith('true/ruff')));
  assert.ok(ids.includes('safe/dynamic-eval'));
});

test('a linter that times out falls back to the pack, not to silence', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const config = loadConfig(repo);
  const out = withShim('ruff', RUFF_SLOW, () =>
    checkFile(path.join(repo, 'a.py'), { repoRoot: repo, config, maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/shell-injection'), 'a timed-out linter produced silence');
  assert.ok(ids.includes('obvious/too-many-params'),
    'nothing answered for the shape rules, so the pack should have');
});

test('a linter that crashes without parseable output falls back to the pack', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const crash = '#!/bin/sh\n[ "$PROCODER_WARMUP" = 1 ] && exit 0\necho "ruff: internal error"\nexit 2\n';
  const out = withShim('ruff', crash, () =>
    checkFile(path.join(repo, 'a.py'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/shell-injection'), 'a crashed linter produced silence');
  assert.ok(ids.includes('obvious/too-many-params'),
    'nothing answered for the shape rules, so the pack should have');
});

test('the same preference applies to the other ecosystems', shimTest, () => {
  const repo = repoWith({
    '.eslintrc.json': '{}',
    'a.ts': 'eval(payload);\nel.innerHTML = danger;\n',  // procoder: literal safe/dynamic-eval, safe/xss-sink the two-line fixture this test writes for the pack to find
  });
  const eslint = '#!/bin/sh\necho \'[{"messages":[{"line":1,"ruleId":"no-unused-vars","message":"unused"}]}]\'\n';
  const out = withShim('eslint', eslint, () =>
    checkFile(path.join(repo, 'a.ts'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.some((id) => id.startsWith('true/eslint')), 'the configured linter did not run');
  assert.ok(ids.includes('safe/dynamic-eval'), 'eval was deferred to eslint');
  assert.ok(ids.includes('safe/xss-sink'), 'the XSS sink was deferred to eslint');
});

test('runs both the language pack and the universal pack', () => {
  const repo = repoWith({ 'src/a.ts': 'el.innerHTML = x;\n// TODO: later\n' });  // procoder: literal safe/xss-sink, alone/orphan-todo scanner input for that rule, not an instance of it
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/xss-sink'), 'language pack did not run');
  assert.ok(ids.includes('alone/orphan-todo'), 'universal pack did not run');
});

test('runs the universal pack even for unsupported file types', () => {
  const repo = repoWith({ 'notes.md': 'key = "AKIAIOSFODNN7EXAMPLE"\n' });  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  const out = checkFile(path.join(repo, 'notes.md'), { repoRoot: repo, config: loadConfig(repo) });
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret'));
});

test('excluded paths are skipped entirely', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["generated/"]\n',
    'generated/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'generated/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'excluded');
  assert.deepStrictEqual(out.findings, []);
});

test('an unreadable file yields skipped, not a throw', () => {
  const repo = repoWith({});
  const out = checkFile(path.join(repo, 'nope.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'unreadable');
});

test('findings are sorted by rung and capped', () => {
  const repo = repoWith({
    'src/a.ts': [
      '// TODO: one',  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
      '// TODO: two',  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
      'eval(a);',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
      'el.innerHTML = b;',  // procoder: literal safe/xss-sink one line of the fixture whose findings this test sorts
      'debugger;',  // procoder: literal alone/debug-leftover scanner input for that rule, not an instance of it
      'console.log(1);',  // procoder: literal alone/debug-leftover scanner input for that rule, not an instance of it
    ].join('\n'),
  });
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: 3 });
  assert.strictEqual(out.findings.length, 3);
  assert.strictEqual(out.findings[0].rung, 'SAFE');
});

test('baselined findings are suppressed', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const config = loadConfig(repo);
  writeBaseline(repo, config, [
    fingerprint(finding({ rung: 'SAFE', id: 'safe/dynamic-eval', line: 1, message: 'm', fix: 'f' }),
      'src/a.ts', 'eval(a);'),  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  ]);
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.deepStrictEqual(out.findings.map((f) => f.id), []);
});

test('applyBaseline false reports findings the baseline would suppress', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const config = loadConfig(repo);
  writeBaseline(repo, config, [
    fingerprint(finding({ rung: 'SAFE', id: 'safe/dynamic-eval', line: 1, message: 'm', fix: 'f' }),
      'src/a.ts', 'eval(a);'),  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  ]);
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), applyBaseline: false });
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('a file past the size cap is skipped, not scanned', () => {
  const repo = repoWith({ 'bundle.ts': 'const x = 1;\n'.repeat(400000) });
  const out = checkFile(path.join(repo, 'bundle.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'too-large');
});

// The old 256KB cap threw away every finding on an ordinary large source. The
// cap exists for files no human edits, so a 400KB one must still be scanned.
test('a large but ordinary source is scanned, not skipped', () => {
  const repo = repoWith({
    'big.ts': `${'const x = 1;\n'.repeat(30000)}var k = "AKIAIOSFODNN7EXAMPLE";\n`,  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  });
  let out;
  const elapsed = cpuMs(() => {
    out = checkFile(path.join(repo, 'big.ts'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  });
  assert.ok(elapsed < 2000, `a 400KB source blew the budget: ${elapsed}ms`);
  assert.strictEqual(out.skipped, null);
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret'));
});

test('a minified file finishes well inside the 2s budget', () => {
  let line = '';
  while (line.length < 200 * 1024) line += `function f${line.length}(a,b){return a&&b?a:b;}`;
  const repo = repoWith({ 'min.ts': line });
  let out;
  const elapsed = cpuMs(() => {
    out = checkFile(path.join(repo, 'min.ts'), { repoRoot: repo, config: loadConfig(repo) });
  });
  assert.ok(elapsed < 2000, `checkFile took ${elapsed}ms`);
  assert.strictEqual(out.skipped, null);
});

test('a long line does not stall a file of otherwise normal lines', () => {
  const repo = repoWith({
    'mixed.ts': `eval(a);\n${'x'.repeat(100 * 1024)}\nel.innerHTML = b;\n`,  // procoder: literal safe/dynamic-eval, safe/xss-sink the short lines either side of the 100KB one, so the scan must reach both
  });
  let out;
  const elapsed = cpuMs(() => {
    out = checkFile(path.join(repo, 'mixed.ts'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  });
  assert.ok(elapsed < 2000, `the long line was scanned anyway: ${elapsed}ms`);
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/dynamic-eval'));
  assert.ok(ids.includes('safe/xss-sink'), 'lines after the long one were dropped');
});

// The line guard blanks long lines before the scanners see them. It must not
// blank them before the universal pack, which is the rung-1 path: a minified
// bundle or a generated file is exactly where a leaked key hides.
test('a secret on a minified line is still reported', () => {
  const long = `function f(a,b){return a&&b?a:b;}`.repeat(300);
  const repo = repoWith({ 'bundle.js': `${long}var k="AKIAIOSFODNN7EXAMPLE";${long}\n` });  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  const out = checkFile(path.join(repo, 'bundle.js'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret'),
    'the long line was blanked before the universal pack saw it');
});

// A minified bundle, a generated API client and a vendored file are exactly
// where an injection sink hides, and the line guard used to blank them before
// the language pack ran. RED before the guard became shape-only: both of these
// reported nothing.
test('an injection sink on a minified line is reported', () => {
  const filler = minifiedLine(20 * 1024);
  const repo = repoWith({
    'bundle.ts': `${filler}db.query(\`SELECT * FROM t WHERE id=\${id}\`);${filler}\n`,
  });
  const out = checkFile(path.join(repo, 'bundle.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(out.findings.some((f) => f.id === 'safe/sql-injection'),
    'the long line was blanked before the language pack saw it');
});

test('disabled TLS verification on a minified line is reported', () => {
  const filler = minifiedLine(20 * 1024);
  const repo = repoWith({
    'bundle.ts': `${filler}https.get({rejectUnauthorized:false});${filler}\n`,  // procoder: literal safe/tls-disabled scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'bundle.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(out.findings.some((f) => f.id === 'safe/tls-disabled'),
    'the long line was blanked before the language pack saw it');
});

// Shape metrics measured on a minified line are noise: every function on it
// starts and ends on line 1, so "function is 1 line" and the depth of the whole
// bundle say nothing about the code a human wrote. The guard stays there.
test('the shape path still does not see a minified line', () => {
  const repo = repoWith({ 'min.ts': `${minifiedLine(20 * 1024)}\n` });
  const out = checkFile(path.join(repo, 'min.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(!out.findings.some((f) => f.id.startsWith('obvious/')),
    'shape rules ran on a minified line');
});

// With the packs reading long lines, one minified line reports 3,000 swallowed
// errors. The per-line cap is what keeps that a report rather than a flood.
test('a minified line that matches thousands of times is capped, and says so', () => {
  const repo = repoWith({ 'bundle.ts': `${'try{a();}catch(e){}'.repeat(3000)}\n` });  // procoder: literal true/swallowed-error the synthetic minified line this test floods the cap with
  let out;
  const elapsed = cpuMs(() => {
    out = checkFile(path.join(repo, 'bundle.ts'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  });
  assert.ok(elapsed < 2000, `the minified line blew the budget: ${elapsed}ms`);
  assert.strictEqual(out.findings.length, MAX_FINDINGS_PER_LINE + 1);
  assert.ok(out.findings.some((f) => f.id === 'true/findings-suppressed'));
});

// A word run used to make the ts signature scan quadratic, so the packs never
// saw the line carrying it. The scan is linear now and the guard is gone, so a
// sink sharing that line is reported — and 1MB stays far inside the budget.
test('a sink on a line with a runaway word run is reported', () => {
  const repo = repoWith({
    'bundle.ts': `db.query(\`select * from t where id = \${id}\`);${'x'.repeat(1024 * 1024)}\n`,
  });
  let out;
  const elapsed = cpuMs(() => {
    out = checkFile(path.join(repo, 'bundle.ts'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  });
  assert.ok(elapsed < 500, `checkFile took ${elapsed}ms on a 1MB word run`);
  assert.ok(out.findings.some((f) => f.id === 'safe/sql-injection'),
    'the sink on the word-run line was invisible');
});

test('a long line stays cheap: the shape path never sees it', () => {
  const repo = repoWith({ 'min.ts': minifiedLine(400 * 1024) });
  const elapsed = cpuMs(() => checkFile(path.join(repo, 'min.ts'),
    { repoRoot: repo, config: loadConfig(repo) }));
  assert.ok(elapsed < 500, `checkFile took ${elapsed}ms — the shape path is quadratic in line length`);
});

// One line can match a rule thousands of times. The cap keeps that off the
// report — and says where it cut, because a silently truncated result is worse
// than a long one.
const FLOODED_LINE = 'try{a();}catch(e){}'.repeat(30);  // procoder: literal true/swallowed-error the fixture line whose repeats exercise the per-line cap

test('one line cannot contribute more than the per-line cap', () => {
  const repo = repoWith({ 'src/a.ts': `${FLOODED_LINE}\n` });
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  const onLine1 = out.findings.filter((f) => f.line === 1 && f.id !== 'true/findings-suppressed');
  assert.strictEqual(onLine1.length, MAX_FINDINGS_PER_LINE);
});

test('the suppressed overflow is reported, not silent', () => {
  const repo = repoWith({ 'src/a.ts': `${FLOODED_LINE}\n` });
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  const notice = out.findings.find((f) => f.id === 'true/findings-suppressed');
  assert.ok(notice, 'findings were dropped without saying so');
  assert.strictEqual(notice.line, 1);
  assert.match(notice.message, /line 1: \d+ further findings suppressed/);
});

test('a line under the cap gets no suppression notice', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a); el.innerHTML = b;\n' });  // procoder: literal safe/dynamic-eval, safe/xss-sink the two-finding fixture that must stay under the per-line cap
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(!out.findings.some((f) => f.id === 'true/findings-suppressed'));
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('a flooded line does not crowd out findings from other lines', () => {
  const repo = repoWith({ 'src/a.ts': `${FLOODED_LINE}\neval(fresh);\n` });
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo) });
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval' && f.line === 2),
    'the per-file cap was spent on one line');
});

test('touched narrows the language pack to the edited region', () => {
  const repo = repoWith({
    'src/a.ts': `eval(old);\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'src/a.ts'), {
    repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity, touched: ['eval(fresh);'],  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const lines = out.findings.filter((f) => f.id === 'safe/dynamic-eval').map((f) => f.line);
  assert.deepStrictEqual(lines, [42]);
});

test('touched never narrows the universal pack — a secret anywhere counts', () => {
  const repo = repoWith({
    'src/a.ts': `const k = "AKIAIOSFODNN7EXAMPLE";\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,  // procoder: literal safe/hardcoded-secret, safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'src/a.ts'), {
    repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity, touched: ['eval(fresh);'],  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret' && f.line === 1));
});

test('touched text that is not in the file falls back to the whole file', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const out = checkFile(path.join(repo, 'src/a.ts'), {
    repoRoot: repo, config: loadConfig(repo), touched: ['nothing like this'],
  });
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('relPath is repo-relative and uses forward slashes', () => {
  const repo = repoWith({ 'src/deep/a.ts': 'eval(a);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const out = checkFile(path.join(repo, 'src/deep/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.relPath, 'src/deep/a.ts');
});

// A .procoderignore skip travels the same channel as an [exclude] paths skip,
// so the hook and the MCP server honour it without knowing it exists — both
// already stop on any `skipped` value.
test('checkFile skips a file covered by a .procoderignore, naming the file', () => {
  const repo = repoWith({
    'gen/.procoderignore': '*.ts\n',
    'gen/a.ts': 'eval(a);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'gen/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'ignored:gen/.procoderignore');
  assert.deepStrictEqual(out.findings, []);
});

test('checkFile still gates a sibling directory the ignore file does not cover', () => {
  const repo = repoWith({
    'gen/.procoderignore': '*.ts\n',
    'src/a.ts': 'eval(a);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, null);
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('a negated pattern puts a file back in the gate', () => {
  const repo = repoWith({
    'gen/.procoderignore': '*.ts\n!keep.ts\n',
    'gen/keep.ts': 'eval(a);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = checkFile(path.join(repo, 'gen/keep.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, null);
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

// --- the caps, re-derived from measurement ---------------------------------

function grow(unit, bytes) {
  let s = '';
  while (s.length < bytes) s += unit;
  return s.slice(0, bytes);
}

// A file that yields one finding per line. At 3MB that is ~157,000 findings,
// and `push(...findings)` spreads every one of them onto the call stack.
//
// RED against the 4MB cap: this threw `Maximum call stack size exceeded` at 3MB
// and 4MB, which the hook's top-level catch turns into a silent exit — the file
// is reported as clean rather than as skipped. That is the exact failure mode a
// cap is supposed to prevent, reachable from inside the cap.
const ONE_FINDING_PER_LINE = 'try{a();}catch(e){}\n';  // procoder: literal true/swallowed-error the unit this test repeats to flood the finding list

test('a file at the cap yields findings rather than overflowing the stack', () => {
  const repo = repoWith({ 'gen.ts': grow(ONE_FINDING_PER_LINE, MAX_FILE_BYTES) });
  const out = checkFile(path.join(repo, 'gen.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.strictEqual(out.skipped, null);
  assert.ok(out.findings.length > 0, 'a file at the cap produced no findings at all');
});

// The same input one byte past the cap: skipped, and skipped for a reason the
// caller can report. Never silently clean.
test('one byte past the cap is skipped with a reason', () => {
  const repo = repoWith({ 'gen.ts': grow(ONE_FINDING_PER_LINE, MAX_FILE_BYTES + 1) });
  const out = checkFile(path.join(repo, 'gen.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'too-large');
  assert.deepStrictEqual(out.findings, []);
});

// A project on slower hardware can ask for a tighter guarantee than the
// measured ceiling. It must be the engine that honours it, not just the config
// that records it.
test('a project may clamp the size cap downward and the engine obeys', () => {
  const repo = repoWith({
    '.procoder.toml': '[limits]\nmax_file_bytes = 4096\n',
    'gen.ts': grow('const x = 1;\n', 8192),
    'small.ts': 'const x = 1;\n',
  });
  const config = loadConfig(repo);
  assert.strictEqual(checkFile(path.join(repo, 'gen.ts'), { repoRoot: repo, config }).skipped,
    'too-large');
  assert.strictEqual(checkFile(path.join(repo, 'small.ts'), { repoRoot: repo, config }).skipped,
    null, 'a file inside the tightened cap is still checked');
});

// RED against the 4MB cap: a 4MB source is inside it, so this was `null` and the
// engine spent ~1.8s of a 2s budget on it once the project's linter was in play.
test('the cap is set below the size that cannot be handled in budget', () => {
  const repo = repoWith({ 'gen.ts': grow('const x = 1;\n', 4 * 1024 * 1024) });
  const out = checkFile(path.join(repo, 'gen.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'too-large', '4MB is past what fits the budget');
});

// Parameter counts are the one shape metric a long line does not corrupt: a
// signature's parameters are all on the same line whether or not the file was
// minified, so an 8-parameter function in a generated client is an 8-parameter
// function. RED before the shape guard became rule-scoped: no obvious/* finding
// at all, because the whole line was blanked before the pack saw it.
test('parameter counts are measured on a generated long line', () => {
  const repo = repoWith({
    'client.ts': `${'export function q(a,b,c,d,e,f,g,h) { return a; }'.repeat(200)}\n`,
  });
  const out = checkFile(path.join(repo, 'client.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(out.findings.some((f) => f.id === 'obvious/too-many-params'),
    'the long line was blanked before the parameter count was taken');
});

// The metrics a long line does corrupt stay guarded. Every function on a
// minified line starts and ends on that line, so its length is 1, its nesting
// depth is the whole bundle's and its complexity is every branch in the file
// added together — "complexity ~497" repeated 248 times is not a measurement.
test('span-derived shape metrics still do not see a minified line', () => {
  const repo = repoWith({ 'min.ts': `${minifiedLine(20 * 1024)}\n` });
  const out = checkFile(path.join(repo, 'min.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  const ids = out.findings.map((f) => f.id);
  for (const id of ['obvious/function-too-long', 'obvious/complexity', 'obvious/nesting-depth']) {
    assert.ok(!ids.includes(id), `${id} was measured across a minified line`);
  }
});

// The budget's worst realistic composition: the project's linter hangs and the
// file is at the cap. The linter is given the budget that is left after the
// pack's share of it is reserved, so growing the file does not grow the total —
// it moves time from the linter to the pack.
//
// Relative by construction: both sides run the same hung linter on the same
// machine, so load moves them together. RED against a fixed linter timeout:
// the tiny file cost ~1.0s and the file at the cap ~1.8s, a ratio of 1.8.
//
// Wall-clock, deliberately, and NOT perf-guard's CPU-time bestOf: the property
// under test is elapsed time spent waiting on a hung child process, which
// costs this process no CPU at all. The budget it defends is a wall-clock
// budget. Being a ratio between two measurements taken back to back is what
// keeps it load-proof here.
function wallBestOf(runs, work) {
  let best = Infinity;
  for (let i = 0; i < runs; i += 1) {
    const started = Date.now();
    work();
    best = Math.min(best, Date.now() - started);
  }
  return best;
}

test('a hung linter plus a file at the cap costs no more than a hung linter alone', shimTest, () => {
  const hang = '#!/bin/sh\n[ "$PROCODER_WARMUP" = 1 ] && exit 0\nsleep 5\n';
  const repo = repoWith({
    '.eslintrc.json': '{}',
    'tiny.ts': 'const x = 1;\n',
    'atcap.ts': grow('export function f(a, b) { if (a) { return b; } return a; }\n', MAX_FILE_BYTES),
  });
  const config = loadConfig(repo);
  const run = (name) => checkFile(path.join(repo, name), { repoRoot: repo, config });

  withShim('eslint', hang, () => {
    const tiny = wallBestOf(2, () => run('tiny.ts'));
    const atCap = wallBestOf(2, () => run('atcap.ts'));
    assert.ok(atCap < tiny * 1.35 + 100,
      `a file at the cap cost ${atCap}ms against ${tiny}ms for a one-line file — `
      + 'the linter did not yield its slice to the pack');
    assert.ok(atCap < BUDGET_MS * 0.8,
      `the worst realistic composition took ${atCap}ms of a ${BUDGET_MS}ms budget`);
  });
});
