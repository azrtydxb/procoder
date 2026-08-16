// tests/checks-config.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadConfig, isExcluded, isRuleExcluded, DEFAULTS, findRepoRoot } = require('../hooks/checks/config');

function tempRepo(files = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cfg-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('absent config yields the documented defaults', () => {
  const cfg = loadConfig(tempRepo());
  assert.strictEqual(cfg.thresholds.function_lines, 40);
  assert.strictEqual(cfg.thresholds.nesting_depth, 3);
  assert.strictEqual(cfg.thresholds.params, 4);
  assert.strictEqual(cfg.thresholds.complexity, 10);
  assert.strictEqual(cfg.baseline.file, DEFAULTS.baseline.file);
});

test('config values override defaults, unset keys keep them', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[thresholds]\nfunction_lines = 80\n\n[baseline]\nfile = "b.json"\n',
  }));
  assert.strictEqual(cfg.thresholds.function_lines, 80);
  assert.strictEqual(cfg.thresholds.nesting_depth, 3);
  assert.strictEqual(cfg.baseline.file, 'b.json');
});

test('rung severities default to error on 1-2 and warn on 3-4', () => {
  const cfg = loadConfig(tempRepo());
  assert.deepStrictEqual(cfg.rungs,
    { safe: 'error', true: 'error', obvious: 'warn', alone: 'warn' });
});

test('a project can promote a judgment rung to error', () => {
  const cfg = loadConfig(tempRepo({ '.procoder.toml': '[rungs]\nobvious = "error"\n' }));
  assert.strictEqual(cfg.rungs.obvious, 'error');
  assert.strictEqual(cfg.rungs.alone, 'warn');
});

test('malformed config falls back to defaults without throwing', () => {
  const cfg = loadConfig(tempRepo({ '.procoder.toml': '[[[not toml' }));
  assert.strictEqual(cfg.thresholds.function_lines, 40);
});

test('isExcluded matches directory prefixes and glob suffixes', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["vendor/", "**/*.generated.ts", "migrations/"]\n',
  }));
  assert.ok(isExcluded(cfg, 'vendor/lib/x.js'));
  assert.ok(isExcluded(cfg, 'src/api.generated.ts'));
  assert.ok(isExcluded(cfg, 'migrations/001_init.sql'));
  assert.ok(!isExcluded(cfg, 'src/api.ts'));
  assert.ok(!isExcluded(cfg, 'src/vendorish.ts'));
});

test('findRepoRoot walks up to the .git directory', () => {
  const dir = tempRepo({ 'a/b/c.txt': 'x' });
  assert.strictEqual(findRepoRoot(path.join(dir, 'a', 'b')), dir);
});

test('findRepoRoot returns the start dir when no .git exists', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-nogit-'));
  assert.strictEqual(findRepoRoot(dir), dir);
});

test('isExcluded treats a bare "?" as a literal character, never throws', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["?abc.ts"]\n',
  }));
  assert.doesNotThrow(() => isExcluded(cfg, '?abc.ts'));
  assert.ok(isExcluded(cfg, '?abc.ts'));
  assert.ok(!isExcluded(cfg, 'xabc.ts'));
});

test('isExcluded treats other regex metacharacters as literals, never throws', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["a+b(c)[d]$.ts"]\n',
  }));
  assert.doesNotThrow(() => isExcluded(cfg, 'a+b(c)[d]$.ts'));
  assert.ok(isExcluded(cfg, 'a+b(c)[d]$.ts'));
  assert.ok(!isExcluded(cfg, 'aXbXcXdX.ts'));
});

test('isExcluded returns a boolean rather than throwing on any malformed pattern', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["***???+++((("]\n',
  }));
  assert.doesNotThrow(() => {
    const result = isExcluded(cfg, 'anything.ts');
    assert.strictEqual(typeof result, 'boolean');
  });
});

test('a rule exclusion silences only the named check in the named file', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\nrules = ["a/patterns.js:alone/orphan-todo"]\n',
  }));
  assert.ok(isRuleExcluded(cfg, 'a/patterns.js', 'alone/orphan-todo'));
  assert.ok(!isRuleExcluded(cfg, 'a/patterns.js', 'alone/commented-code'));
  assert.ok(!isRuleExcluded(cfg, 'a/other.js', 'alone/orphan-todo'));
  assert.ok(!isExcluded(cfg, 'a/patterns.js'));
});

// The reported kill switch: two lines of config that read as noise and turn
// the entire gate off by writing `paths` onto Object.prototype.
test('a [exclude.__proto__] table cannot switch the gate off', () => {
  delete Object.prototype.paths;
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude.__proto__]\npaths = ["**/*"]\n',
  }));
  assert.ok(!cfg.exclude.paths.includes('**/*'));
  assert.ok(!isExcluded(cfg, 'src/a.ts'));
  assert.strictEqual(Object.prototype.paths, undefined);
  delete Object.prototype.paths;
});

// The narrow form must stay narrow: a directory or glob path half would let one
// entry silence a rule across a whole tree, which is the thing path exclusions
// already do and rule exclusions exist to avoid.
test('a rule exclusion with a directory path is dropped, not applied to the tree', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\nrules = ["hooks/:alone/orphan-todo"]\n',
  }));
  assert.deepStrictEqual(cfg.exclude.rules, []);
  assert.ok(!isRuleExcluded(cfg, 'hooks/checks/a.js', 'alone/orphan-todo'));
  assert.ok(!isRuleExcluded(cfg, 'hooks/', 'alone/orphan-todo'));
});

test('a rule exclusion with a glob path is dropped, not applied to matches', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\nrules = ["**/*.js:alone/orphan-todo", "hooks/*.js:safe/eval"]\n',
  }));
  assert.deepStrictEqual(cfg.exclude.rules, []);
  assert.ok(!isRuleExcluded(cfg, 'hooks/a.js', 'alone/orphan-todo'));
  assert.ok(!isRuleExcluded(cfg, 'hooks/a.js', 'safe/eval'));
});

test('an exact rule exclusion path still works, and only for that path', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\nrules = ["hooks/checks/patterns/markers.js:alone/orphan-todo"]\n',
  }));
  assert.ok(isRuleExcluded(cfg, 'hooks/checks/patterns/markers.js', 'alone/orphan-todo'));
  assert.ok(!isRuleExcluded(cfg, 'hooks/checks/patterns/markers.js.bak', 'alone/orphan-todo'));
  assert.ok(!isRuleExcluded(cfg, 'hooks/checks/patterns/other.js', 'alone/orphan-todo'));
});

test("procoder's own rule exclusions survive the exact-path rule", () => {
  const cfg = loadConfig(path.resolve(__dirname, '..'));
  assert.strictEqual(cfg.exclude.rules.length, 3);
  for (const rule of cfg.exclude.rules) {
    assert.strictEqual(rule.path, 'hooks/checks/patterns/markers.js');
    assert.ok(isRuleExcluded(cfg, rule.path, rule.id));
  }
});

test('a rule exclusion missing either half is dropped, never widened', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\nrules = ["a/patterns.js", "alone/orphan-todo", ":x", "y:"]\n',
  }));
  assert.deepStrictEqual(cfg.exclude.rules, []);
  assert.ok(!isRuleExcluded(cfg, 'a/patterns.js', 'alone/orphan-todo'));
});

// An entry is `path:id`, and ids now carry an external tool's own rule id
// ("true/eslint:no-eval"), so the split has to be on the first colon: a path
// with a colon in it is not a real case, an id with one is.
test('a rule exclusion splits on the first colon, so tool rule ids survive', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml':
      '[exclude]\nrules = ["src/a.ts:obvious/complexity", "src/b.ts:true/eslint:no-eval"]\n',
  }));
  assert.deepStrictEqual(cfg.exclude.rules, [
    { path: 'src/a.ts', id: 'obvious/complexity' },
    { path: 'src/b.ts', id: 'true/eslint:no-eval' },
  ]);
  assert.ok(isRuleExcluded(cfg, 'src/a.ts', 'obvious/complexity'));
  assert.ok(isRuleExcluded(cfg, 'src/b.ts', 'true/eslint:no-eval'));
  assert.ok(!isRuleExcluded(cfg, 'src/a.ts', 'true/eslint:no-eval'));
});
