const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
const { checkFile } = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');
const { writeBaseline, fingerprint } = require('../hooks/checks/baseline');
const { finding } = require('../hooks/checks/finding');

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

test('a configured linter never displaces the SAFE rung', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const out = withShim('ruff', RUFF_OK, () =>
    checkFile(path.join(repo, 'a.py'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/shell-injection'), 'shell injection was deferred to ruff');
  assert.ok(ids.includes('safe/dynamic-eval'), 'eval was deferred to ruff');
  assert.ok(ids.includes('true/ruff'), 'the configured linter did not run');
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
  assert.ok(!ids.includes('true/ruff'), 'ran a linter the project has not configured');
  assert.ok(ids.includes('safe/shell-injection'));
});

test('a configured but absent linter leaves the pack in charge', shimTest, () => {
  const repo = repoWith({ 'pyproject.toml': CONFIGURED, 'a.py': UNSAFE_PY });
  const out = checkFile(path.join(repo, 'a.py'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  const ids = out.findings.map((f) => f.id);
  assert.ok(!ids.includes('true/ruff'));
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
    'a.ts': 'eval(payload);\nel.innerHTML = danger;\n',
  });
  const eslint = '#!/bin/sh\necho \'[{"messages":[{"line":1,"ruleId":"no-unused-vars","message":"unused"}]}]\'\n';
  const out = withShim('eslint', eslint, () =>
    checkFile(path.join(repo, 'a.ts'),
      { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity }));
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('true/eslint'), 'the configured linter did not run');
  assert.ok(ids.includes('safe/dynamic-eval'), 'eval was deferred to eslint');
  assert.ok(ids.includes('safe/xss-sink'), 'the XSS sink was deferred to eslint');
});

test('runs both the language pack and the universal pack', () => {
  const repo = repoWith({ 'src/a.ts': 'el.innerHTML = x;\n// TODO: later\n' });
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/xss-sink'), 'language pack did not run');
  assert.ok(ids.includes('alone/orphan-todo'), 'universal pack did not run');
});

test('runs the universal pack even for unsupported file types', () => {
  const repo = repoWith({ 'notes.md': 'key = "AKIAIOSFODNN7EXAMPLE"\n' });
  const out = checkFile(path.join(repo, 'notes.md'), { repoRoot: repo, config: loadConfig(repo) });
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret'));
});

test('excluded paths are skipped entirely', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["generated/"]\n',
    'generated/a.ts': 'eval(x);\n',
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
      '// TODO: one',
      '// TODO: two',
      'eval(a);',
      'el.innerHTML = b;',
      'debugger;',
      'console.log(1);',
    ].join('\n'),
  });
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: 3 });
  assert.strictEqual(out.findings.length, 3);
  assert.strictEqual(out.findings[0].rung, 'SAFE');
});

test('baselined findings are suppressed', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });
  const config = loadConfig(repo);
  writeBaseline(repo, config, [
    fingerprint(finding({ rung: 'SAFE', id: 'safe/dynamic-eval', line: 1, message: 'm', fix: 'f' }),
      'src/a.ts', 'eval(a);'),
  ]);
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.deepStrictEqual(out.findings.map((f) => f.id), []);
});

test('applyBaseline false reports findings the baseline would suppress', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });
  const config = loadConfig(repo);
  writeBaseline(repo, config, [
    fingerprint(finding({ rung: 'SAFE', id: 'safe/dynamic-eval', line: 1, message: 'm', fix: 'f' }),
      'src/a.ts', 'eval(a);'),
  ]);
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), applyBaseline: false });
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('a file past the size cap is skipped, not scanned', () => {
  const repo = repoWith({ 'bundle.ts': 'const x = 1;\n'.repeat(30000) });
  const out = checkFile(path.join(repo, 'bundle.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'too-large');
});

test('a minified file finishes well inside the 2s budget', () => {
  let line = '';
  while (line.length < 200 * 1024) line += `function f${line.length}(a,b){return a&&b?a:b;}`;
  const repo = repoWith({ 'min.ts': line });
  const started = Date.now();
  const out = checkFile(path.join(repo, 'min.ts'), { repoRoot: repo, config: loadConfig(repo) });
  const elapsed = Date.now() - started;
  assert.ok(elapsed < 2000, `checkFile took ${elapsed}ms`);
  assert.strictEqual(out.skipped, null);
});

test('a long line does not stall a file of otherwise normal lines', () => {
  const repo = repoWith({
    'mixed.ts': `eval(a);\n${'x'.repeat(100 * 1024)}\nel.innerHTML = b;\n`,
  });
  const started = Date.now();
  const out = checkFile(path.join(repo, 'mixed.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity });
  assert.ok(Date.now() - started < 2000, 'the long line was scanned anyway');
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/dynamic-eval'));
  assert.ok(ids.includes('safe/xss-sink'), 'lines after the long one were dropped');
});

test('touched narrows the language pack to the edited region', () => {
  const repo = repoWith({
    'src/a.ts': `eval(old);\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,
  });
  const out = checkFile(path.join(repo, 'src/a.ts'), {
    repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity, touched: ['eval(fresh);'],
  });
  const lines = out.findings.filter((f) => f.id === 'safe/dynamic-eval').map((f) => f.line);
  assert.deepStrictEqual(lines, [42]);
});

test('touched never narrows the universal pack — a secret anywhere counts', () => {
  const repo = repoWith({
    'src/a.ts': `const k = "AKIAIOSFODNN7EXAMPLE";\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,
  });
  const out = checkFile(path.join(repo, 'src/a.ts'), {
    repoRoot: repo, config: loadConfig(repo), maxFindings: Infinity, touched: ['eval(fresh);'],
  });
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret' && f.line === 1));
});

test('touched text that is not in the file falls back to the whole file', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });
  const out = checkFile(path.join(repo, 'src/a.ts'), {
    repoRoot: repo, config: loadConfig(repo), touched: ['nothing like this'],
  });
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('relPath is repo-relative and uses forward slashes', () => {
  const repo = repoWith({ 'src/deep/a.ts': 'eval(a);\n' });
  const out = checkFile(path.join(repo, 'src/deep/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.relPath, 'src/deep/a.ts');
});
