// tests/checks-config.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  loadConfig, isExcluded, isRuleExcluded, excludeReason, DEFAULTS, findRepoRoot,
} = require('../hooks/checks/config');

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

// The three that used to be here — one per rung-4 marker rule, all pointing at
// hooks/checks/patterns/markers.js — were replaced by per-line `procoder:
// literal` markers in that file. The mechanism still exists and is still
// tested above; this project just no longer needs it.
test('procoder itself now excludes no rule anywhere', () => {
  const cfg = loadConfig(path.resolve(__dirname, '..'));
  assert.deepStrictEqual(cfg.exclude.rules, []);
  assert.ok(!isRuleExcluded(cfg, 'hooks/checks/patterns/markers.js', 'alone/orphan-todo'));
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

// The README's [rungs] example is the contract users type. mergeSection merges
// by literal key, so a documented key the code does not use is a knob that
// silently does nothing — this fails the moment docs and code drift apart.
test('every rung key the README documents actually changes a severity', () => {
  const readme = fs.readFileSync(path.join(__dirname, '..', 'README.md'), 'utf8');
  const block = /\[rungs\]\n((?:[a-z_]+ = "[a-z]+"\n)+)/.exec(readme);
  assert.ok(block, 'README no longer documents a [rungs] example');
  const keys = block[1].trim().split('\n').map((line) => line.split(' ')[0]);
  assert.deepStrictEqual(keys.slice().sort(), Object.keys(DEFAULTS.rungs).sort());

  for (const key of keys) {
    const cfg = loadConfig(tempRepo({ '.procoder.toml': `[rungs]\n${key} = "warn"\n` }));
    assert.strictEqual(cfg.rungs[key], 'warn', `documented key ${key} did nothing`);
    assert.deepStrictEqual(Object.keys(cfg.rungs).sort(), keys.slice().sort());
  }
});

// ---------------------------------------------------------------------------
// .procoderignore — per-directory ignore files.
//
// The syntax subset is deliberately small and every line of it is tested here,
// because the failure mode this project has already been bitten by is a config
// parser that accepts valid-looking syntax and quietly does nothing with it.

test('an ignore file excludes files beneath it, not siblings or parents', () => {
  const cfg = loadConfig(tempRepo({ 'gen/.procoderignore': '*.ts\n' }));
  assert.ok(isExcluded(cfg, 'gen/a.ts'));
  assert.ok(isExcluded(cfg, 'gen/deep/b.ts'));
  assert.ok(!isExcluded(cfg, 'src/a.ts'));
  assert.ok(!isExcluded(cfg, 'a.ts'));
});

test('excludeReason names the ignore file that skipped a path', () => {
  const cfg = loadConfig(tempRepo({ 'gen/.procoderignore': '*.ts\n' }));
  assert.strictEqual(excludeReason(cfg, 'gen/a.ts'), 'ignored:gen/.procoderignore');
  assert.strictEqual(excludeReason(cfg, 'src/a.ts'), null);
});

test('config.noIgnore turns every ignore file off, but not [exclude] paths', () => {
  const cfg = loadConfig(tempRepo({ 'gen/.procoderignore': '*.ts\n' }));
  const off = { ...cfg, noIgnore: true };
  assert.strictEqual(excludeReason(off, 'gen/a.ts'), null);
  assert.strictEqual(excludeReason(off, 'node_modules/a.ts'), 'excluded');
});

test('blank lines and # comments are skipped, not matched literally', () => {
  const cfg = loadConfig(tempRepo({
    '.procoderignore': '# a comment\n\n   \n*.gen.ts\n',
  }));
  assert.ok(isExcluded(cfg, 'src/a.gen.ts'));
  assert.ok(!isExcluded(cfg, '# a comment'));
  assert.ok(!isExcluded(cfg, 'src/a.ts'));
});

// "out", not "build": build/ is one of the built-in [exclude] paths defaults,
// so a fixture using it would pass without the ignore file being read at all.
test('a trailing slash matches a directory tree, never a file of that name', () => {
  const cfg = loadConfig(tempRepo({ '.procoderignore': 'out/\n' }));
  assert.ok(isExcluded(cfg, 'out/a.ts'));
  assert.ok(isExcluded(cfg, 'src/out/a.ts'), 'a bare name matches at any depth');
  assert.ok(!isExcluded(cfg, 'out'));
});

test('a leading slash anchors to the directory holding the ignore file', () => {
  const cfg = loadConfig(tempRepo({ 'pkg/.procoderignore': '/vendor/\n' }));
  assert.ok(isExcluded(cfg, 'pkg/vendor/a.ts'));
  assert.ok(!isExcluded(cfg, 'pkg/src/vendor/a.ts'));
});

test('a pattern with an interior slash is anchored, one without matches at depth', () => {
  const cfg = loadConfig(tempRepo({ '.procoderignore': 'src/gen/*.ts\nnotes.md\n' }));
  assert.ok(isExcluded(cfg, 'src/gen/a.ts'));
  assert.ok(!isExcluded(cfg, 'pkg/src/gen/a.ts'));
  assert.ok(isExcluded(cfg, 'deep/down/notes.md'));
});

test('a star stops at a slash, a double star crosses one', () => {
  const cfg = loadConfig(tempRepo({ '.procoderignore': 'a/*.ts\nb/**/*.ts\n' }));
  assert.ok(isExcluded(cfg, 'a/x.ts'));
  assert.ok(!isExcluded(cfg, 'a/deep/x.ts'));
  assert.ok(isExcluded(cfg, 'b/x.ts'), 'double-star-slash also matches zero directories');
  assert.ok(isExcluded(cfg, 'b/deep/down/x.ts'));
});

test('negation re-includes a file the same ignore file excluded', () => {
  const cfg = loadConfig(tempRepo({ 'gen/.procoderignore': '*.ts\n!keep.ts\n' }));
  assert.ok(isExcluded(cfg, 'gen/drop.ts'));
  assert.ok(!isExcluded(cfg, 'gen/keep.ts'));
});

test('order decides: a later pattern re-excludes what an earlier negation kept', () => {
  const cfg = loadConfig(tempRepo({ 'gen/.procoderignore': '*.ts\n!keep.ts\nkeep.ts\n' }));
  assert.ok(isExcluded(cfg, 'gen/keep.ts'));
});

test('a nested ignore file wins over its parent, in both directions', () => {
  const cfg = loadConfig(tempRepo({
    'gen/.procoderignore': '*.ts\n',
    'gen/keep/.procoderignore': '!*.ts\n',
    'src/.procoderignore': '!*.ts\n',
    'src/drop/.procoderignore': '*.ts\n',
  }));
  assert.ok(isExcluded(cfg, 'gen/a.ts'));
  assert.ok(!isExcluded(cfg, 'gen/keep/a.ts'), 'the deeper file re-includes');
  assert.ok(!isExcluded(cfg, 'src/a.ts'));
  assert.ok(isExcluded(cfg, 'src/drop/a.ts'), 'the deeper file excludes');
});

test('an ignore file cannot reach above its own directory', () => {
  const cfg = loadConfig(tempRepo({ 'gen/.procoderignore': '*.ts\n/../src/*.ts\n' }));
  assert.ok(!isExcluded(cfg, 'src/a.ts'));
  assert.ok(!isExcluded(cfg, 'a.ts'));
  assert.ok(isExcluded(cfg, 'gen/a.ts'), 'the sane pattern in the same file still works');
});

test('a parent-directory segment in a pattern is dropped, so nothing above matches', () => {
  const cfg = loadConfig(tempRepo({ '.procoderignore': '../**\n..\n' }));
  assert.ok(!isExcluded(cfg, '../outside.ts'));
  assert.ok(!isExcluded(cfg, 'src/a.ts'));
});

test('a malformed ignore file ignores nothing and never throws', () => {
  const cfg = loadConfig(tempRepo({
    '.procoderignore': '***???+++(((\n[unclosed\n!\n/\n binary\n',
  }));
  assert.doesNotThrow(() => isExcluded(cfg, 'src/a.ts'));
  assert.strictEqual(isExcluded(cfg, 'src/a.ts'), false);
});

// A .procoder.toml exclusion is the project-wide contract; a subdirectory file
// may narrow further but must never contradict it.
test('a negation in .procoderignore cannot re-include what .procoder.toml excluded', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["vendor/"]\n',
    'vendor/.procoderignore': '!*.ts\n',
  }));
  assert.strictEqual(excludeReason(cfg, 'vendor/a.ts'), 'excluded');
});

test('a .procoderignore adds to what .procoder.toml excludes', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["vendor/"]\n',
    'gen/.procoderignore': '*.ts\n',
  }));
  assert.strictEqual(excludeReason(cfg, 'vendor/a.ts'), 'excluded');
  assert.strictEqual(excludeReason(cfg, 'gen/a.ts'), 'ignored:gen/.procoderignore');
  assert.strictEqual(excludeReason(cfg, 'src/a.ts'), null);
});

// The hook's whole budget is 2s per file. Resolution is cached per directory,
// so a deep tree costs one chain walk per directory, not one per file.
test('resolving a deep tree with several ignore files stays far inside the budget', () => {
  const files = {};
  let dir = '';
  for (let depth = 0; depth < 12; depth += 1) {
    dir = dir ? `${dir}/d${depth}` : 'd0';
    if (depth % 3 === 0) files[`${dir}/.procoderignore`] = '*.gen.ts\n!keep.gen.ts\n';
  }
  const cfg = loadConfig(tempRepo(files));

  const started = Date.now();
  for (let i = 0; i < 2000; i += 1) isExcluded(cfg, `${dir}/f${i}.gen.ts`);
  const elapsed = Date.now() - started;
  assert.ok(elapsed < 200, `2000 lookups took ${elapsed}ms`);
});

// --- [limits] max_file_bytes ------------------------------------------------
//
// The built-in ceiling is a MEASURED limit, not a preference: past 2MB the
// engine either eats the whole hook budget or overflows the stack. So the key
// clamps downward only — a project on slower hardware may ask for less, and no
// project may ask for more than measurement says is survivable.

function captureStderr(fn) {
  const originalWrite = process.stderr.write;
  let captured = '';
  process.stderr.write = (chunk) => { captured += chunk; return true; };
  try {
    return { value: fn(), captured };
  } finally {
    process.stderr.write = originalWrite;
  }
}

test('max_file_bytes defaults to the built-in ceiling', () => {
  assert.strictEqual(loadConfig(tempRepo()).limits.max_file_bytes,
    DEFAULTS.limits.max_file_bytes);
});

test('a max_file_bytes below the ceiling is honoured', () => {
  const { value, captured } = captureStderr(() => loadConfig(tempRepo({
    '.procoder.toml': '[limits]\nmax_file_bytes = 262144\n',
  })));
  assert.strictEqual(value.limits.max_file_bytes, 262144);
  assert.strictEqual(captured, '', 'a value inside the ceiling is not worth a warning');
});

test('a max_file_bytes above the ceiling is refused, warned about, and clamped', () => {
  const { value, captured } = captureStderr(() => loadConfig(tempRepo({
    '.procoder.toml': '# a comment\n[limits]\nmax_file_bytes = 8388608\n',
  })));
  assert.strictEqual(value.limits.max_file_bytes, DEFAULTS.limits.max_file_bytes,
    'config must never raise a limit measurement says is unsafe');
  assert.match(captured, /max_file_bytes/);
  assert.match(captured, /\.procoder\.toml:3\b/, 'the warning must name file and line');
});

test('a non-numeric max_file_bytes is refused, not coerced', () => {
  const { value, captured } = captureStderr(() => loadConfig(tempRepo({
    '.procoder.toml': '[limits]\nmax_file_bytes = "lots"\n',
  })));
  assert.strictEqual(value.limits.max_file_bytes, DEFAULTS.limits.max_file_bytes);
  assert.match(captured, /max_file_bytes/);
});

test('a zero or negative max_file_bytes is refused, not obeyed', () => {
  // Obeying it would skip every file in the repo and report nothing wrong —
  // the config key turned into a silent kill switch for the whole gate.
  const { value } = captureStderr(() => loadConfig(tempRepo({
    '.procoder.toml': '[limits]\nmax_file_bytes = 0\n',
  })));
  assert.strictEqual(value.limits.max_file_bytes, DEFAULTS.limits.max_file_bytes);
});
