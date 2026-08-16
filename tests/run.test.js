const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
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

test('relPath is repo-relative and uses forward slashes', () => {
  const repo = repoWith({ 'src/deep/a.ts': 'eval(a);\n' });
  const out = checkFile(path.join(repo, 'src/deep/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.relPath, 'src/deep/a.ts');
});
