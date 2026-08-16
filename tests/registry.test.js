const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { packFor, toolFor, PACKS } = require('../hooks/checks/registry');

const tempDirs = [];
test.after(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true });
});

function tempRepo(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-reg-'));
  tempDirs.push(dir);
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('maps every supported extension to exactly one pack', () => {
  const seen = new Map();
  for (const pack of PACKS) {
    for (const ext of pack.EXTENSIONS) {
      assert.ok(!seen.has(ext), `${ext} claimed by two packs`);
      seen.set(ext, pack);
    }
  }
  assert.ok(seen.size >= 12);
});

test('packFor resolves by extension and is case-insensitive', () => {
  assert.ok(packFor('src/a.ts'));
  assert.ok(packFor('src/A.PY'));
  assert.strictEqual(packFor('README.md'), null);
  assert.strictEqual(packFor('Makefile'), null);
});

test('toolFor names the external tool preferred for each language', () => {
  assert.strictEqual(toolFor('a.py').name, 'ruff');
  assert.strictEqual(toolFor('a.ts').name, 'eslint');
  assert.strictEqual(toolFor('a.go').name, 'golangci-lint');
  // The rust entry invokes the `cargo` binary (argv starts with 'clippy') —
  // there is no standalone `clippy` binary on a normal PATH, only
  // `cargo-clippy`, which cargo dispatches to internally.
  assert.strictEqual(toolFor('a.rs').name, 'cargo');
  assert.strictEqual(toolFor('README.md'), null);
});

test('clippy argv invokes cargo clippy, not a nonexistent clippy binary', () => {
  const rust = toolFor('a.rs');
  const argv = rust.argv('/repo/src/main.rs');
  assert.strictEqual(argv[0], 'clippy');
});

test('clippy parse discards findings attributed to a different file', () => {
  const rust = toolFor('a.rs');
  // cargo clippy cannot be scoped to a single file — it always compiles the
  // whole crate — so its output can legitimately contain warnings from
  // files other than the one being checked. argv() records which absolute
  // path is under inspection; parse() must discard anything not from it.
  rust.argv('/repo/src/main.rs');
  const stdout = [
    'src/main.rs:10:5: warning: unused variable: `x` [unused_variables]',
    'src/other.rs:412:3: warning: needless return [clippy::needless_return]',
  ].join('\n');
  const parsed = rust.parse(stdout);
  assert.strictEqual(parsed.length, 1);
  assert.strictEqual(parsed[0].line, 10);
  assert.match(parsed[0].message, /unused variable/);
});

test('each tool entry can parse its own output format', () => {
  const ruff = toolFor('a.py');
  const parsed = ruff.parse(JSON.stringify([
    { filename: 'a.py', location: { row: 7 }, code: 'E722', message: 'do not use bare except' },
  ]));
  assert.strictEqual(parsed.length, 1);
  assert.strictEqual(parsed[0].line, 7);
  assert.match(parsed[0].message, /bare except/);
});

test('each finding id carries the tool\'s own rule id, not just the tool name', () => {
  const ruff = toolFor('a.py');
  const [ruffFinding] = ruff.parse(JSON.stringify([
    { filename: 'a.py', location: { row: 7 }, code: 'E722', message: 'do not use bare except' },
  ]));
  assert.strictEqual(ruffFinding.id, 'true/ruff:E722');

  const golangci = toolFor('a.go');
  const [golangciFinding] = golangci.parse(JSON.stringify({
    Issues: [{ Pos: { Line: 12 }, FromLinter: 'govet', Text: 'nilness' }],
  }));
  assert.strictEqual(golangciFinding.id, 'true/golangci-lint:govet');
});

test('the clippy entry declares that it reports on stderr', () => {
  // cargo clippy writes every diagnostic to stderr and still exits 0. An entry
  // that does not say so is read on stdout, where there is nothing — which the
  // runner cannot tell from a clean crate, so the Rust pack is skipped too.
  assert.strictEqual(toolFor('a.rs').stream, 'stderr');
  assert.strictEqual(toolFor('a.py').stream, undefined);
  assert.strictEqual(toolFor('a.ts').stream, undefined);
  assert.strictEqual(toolFor('a.go').stream, undefined);
});

test('golangci-lint v2 output is parsed despite the summary it appends to stdout', () => {
  // Verified against golangci-lint 2.12.2: `run --output.json.path stdout`
  // writes the JSON document AND then a human-readable tally to stdout, so
  // JSON.parse over the whole stream throws and every finding is dropped.
  const golangci = toolFor('a.go');
  const v2 = `${JSON.stringify({
    Issues: [{ Pos: { Filename: 'main.go', Line: 6, Column: 2 }, FromLinter: 'typecheck', Text: 'declared and not used: x' }],
    Report: { Linters: [{ Name: 'govet', Enabled: true }] },
  })}\n1 issues:\n* typecheck: 1\n`;
  const parsed = golangci.parse(v2);
  assert.strictEqual(parsed.length, 1);
  assert.strictEqual(parsed[0].line, 6);
  assert.strictEqual(parsed[0].id, 'true/golangci-lint:typecheck');
});

test('golangci-lint v2 output with no issues parses to no findings, not to an error', () => {
  const empty = `${JSON.stringify({ Issues: [], Report: { Linters: [] } })}\n0 issues.\n`;
  assert.deepStrictEqual(toolFor('a.go').parse(empty), []);
});

test('golangci-lint v1 pure-JSON output still parses', () => {
  const parsed = toolFor('a.go').parse(JSON.stringify({
    Issues: [{ Pos: { Line: 12 }, FromLinter: 'govet', Text: 'nilness' }],
  }));
  assert.strictEqual(parsed.length, 1);
  assert.strictEqual(parsed[0].line, 12);
});

test('a parser that cannot read its input throws rather than reporting a clean file', () => {
  // Returning [] for unreadable output is the defect in miniature: the runner
  // reads it as "the tool answered, nothing found" and skips the pack. Every
  // parser must instead signal that it could not read what it was given.
  assert.throws(() => toolFor('a.py').parse('ruff: unrecognized subcommand\n'));
  assert.throws(() => toolFor('a.ts').parse('<html>404</html>'));
  assert.throws(() => toolFor('a.go').parse('golangci-lint: unknown flag --out-format\n'));
  assert.throws(() => toolFor('a.rs').parse('error: could not compile `x` (lib)\n'));
});

test('a parser reading genuinely empty output reports no findings and does not throw', () => {
  assert.deepStrictEqual(toolFor('a.rs').parse(''), []);
  assert.deepStrictEqual(toolFor('a.py').parse('[]'), []);
});

// --- what the real tools actually emit, and what argv actually asks for -----

test('the eslint entry names flat config in every extension eslint loads it from', () => {
  // eslint 9 introduced .cjs/.ts flat config and eslint 10 dropped .eslintrc
  // entirely. A project on `eslint.config.cjs` or `eslint.config.ts` was read
  // as having no linter at all, so the integration never ran for it.
  const names = toolFor('a.ts').configFiles;
  for (const ext of ['js', 'mjs', 'cjs', 'ts', 'mts', 'cts']) {
    assert.ok(names.includes(`eslint.config.${ext}`), `eslint.config.${ext} is not recognised`);
  }
});

test('a file eslint declined to lint is not reported as clean', () => {
  // Verified against eslint 10.8.1: an ignored path answers with exactly this
  // — one warning, ruleId null, NO line field — and exits 0. The runner drops
  // line-less findings, so it read as "answered, nothing found" and took the
  // pack's obvious/* rules down with it for every path in an `ignores` list.
  const ignored = JSON.stringify([{
    filePath: 'a.js',
    messages: [{
      ruleId: null,
      fatal: false,
      severity: 1,
      message: 'File ignored because of a matching ignore pattern. Use "--no-ignore" to disable file ignore settings or use "--no-warn-ignored" to suppress this warning.',
    }],
  }]);
  assert.throws(() => toolFor('a.ts').parse(ignored));

  const fatal = JSON.stringify([{
    filePath: 'a.js',
    messages: [{ ruleId: null, fatal: true, severity: 2, message: 'Parsing error: Unexpected token (', line: 1, column: 10 }],
  }]);
  assert.throws(() => toolFor('a.ts').parse(fatal), 'a file eslint could not parse was not linted');
});

test('a file ruff could not parse is not reported as clean', () => {
  // ruff 0.16.3 answers an unparseable file with one `invalid-syntax` item and
  // lints nothing else in it.
  const syntax = JSON.stringify([{
    code: 'invalid-syntax', location: { row: 2 }, message: 'unexpected EOF while parsing', filename: 'a.py',
  }]);
  assert.throws(() => toolFor('a.py').parse(syntax));
});

test('ruff is asked about the file, not allowed to decline it', () => {
  // --force-exclude made ruff answer an excluded path with `[]` and exit 0 —
  // byte for byte what a clean file produces — so procoder deferred its shape
  // rules to a run that never opened the file.
  const argv = toolFor('a.py').argv('/repo/a.py');
  assert.ok(!argv.includes('--force-exclude'), 'ruff must not be allowed to silently decline a file');
  assert.deepStrictEqual(argv, ['check', '--output-format', 'json', '/repo/a.py']);
  // ruff reads pyproject.toml, ruff.toml and .ruff.toml — and nothing else.
  assert.ok(!toolFor('a.py').configFiles.includes('setup.cfg'));
});

test('clippy is scoped to the package that owns the file', () => {
  // clippy cannot be scoped to a file, but it can be scoped to a package, and
  // in a workspace that is the difference between compiling one member and
  // compiling every member inside a 1.5s budget.
  const repo = tempRepo({
    'Cargo.toml': '[workspace]\nmembers = ["crate-a"]\n',
    'crate-a/Cargo.toml': '[package]\nname = "crate-a"\nversion = "0.1.0"\n\n[lints]\nworkspace = true\n',
    'crate-a/src/lib.rs': 'pub fn f() {}\n',
  });
  const argv = toolFor('a.rs').argv(path.join(repo, 'crate-a/src/lib.rs'));
  assert.deepStrictEqual(argv, ['clippy', '-p', 'crate-a', '--message-format', 'short', '--quiet']);
});

test('clippy falls back to an unscoped run when no package owns the file', () => {
  const repo = tempRepo({ 'src/lib.rs': 'pub fn f() {}\n' });
  assert.deepStrictEqual(
    toolFor('a.rs').argv(path.join(repo, 'src/lib.rs')),
    ['clippy', '--message-format', 'short', '--quiet'],
  );
});

// --- the format contract, against the real binary ---------------------------
//
// Everything above this line is a shim, and a shim proves only that the parser
// handles the shape its author imagined — which is exactly how clippy's stderr
// and golangci-lint's trailing tally survived. These run the real binary when
// it is on PATH and skip when it is not, so a machine that has the tool proves
// the format contract and a machine that does not still passes.
const { hasTool } = require('../hooks/checks/resolve');

function realStream(tool, repo, relFile) {
  const run = spawnSync(tool.name, tool.argv(path.join(repo, relFile)), {
    cwd: repo, encoding: 'utf8', timeout: 120000, shell: false, maxBuffer: 8 * 1024 * 1024,
  });
  assert.ok(!run.error, `${tool.name} did not run: ${run.error && run.error.message}`);
  return String((tool.stream === 'stderr' ? run.stderr : run.stdout) || '');
}

const missing = (name) => !hasTool(name) && `${name} is not on PATH`;

test('eslint (real): the parser reads what eslint actually prints', { skip: missing('eslint') }, () => {
  const repo = tempRepo({
    'package.json': '{"name":"procoder-fixture","version":"1.0.0","private":true}\n',
    'eslint.config.js': "module.exports = [{ rules: { 'no-unused-vars': 'error' } }];\n",
    'a.js': 'var dead = 1;\n',
  });
  const parsed = toolFor('a.ts').parse(realStream(toolFor('a.ts'), repo, 'a.js'));
  assert.deepStrictEqual(parsed.map((f) => [f.id, f.line]), [['true/eslint:no-unused-vars', 1]]);
});

test('ruff (real): the parser reads what ruff actually prints', { skip: missing('ruff') }, () => {
  const repo = tempRepo({ 'ruff.toml': '[lint]\nselect = ["F"]\n', 'a.py': 'import os\n' });
  const parsed = toolFor('a.py').parse(realStream(toolFor('a.py'), repo, 'a.py'));
  assert.deepStrictEqual(parsed.map((f) => [f.id, f.line]), [['true/ruff:F401', 1]]);
});

test('golangci-lint (real): the parser reads what golangci-lint actually prints', {
  skip: missing('golangci-lint') || missing('go'),
}, () => {
  const repo = tempRepo({
    '.golangci.yml': 'version: "2"\nlinters:\n  default: standard\n',
    'go.mod': 'module demo\n\ngo 1.24\n',
    'a.go': 'package main\n\nfunc main() {}\n\nfunc unusedA() {}\n',
  });
  const parsed = toolFor('a.go').parse(realStream(toolFor('a.go'), repo, 'a.go'));
  assert.deepStrictEqual(parsed.map((f) => [f.id, f.line]), [['true/golangci-lint:unused', 5]]);
});

test('clippy (real): the parser reads what clippy actually prints, on stderr', {
  skip: missing('cargo'),
}, () => {
  const repo = tempRepo({
    'Cargo.toml': '[package]\nname = "cratea"\nversion = "0.1.0"\nedition = "2021"\n\n[lints.clippy]\nall = "warn"\n',
    'src/lib.rs': 'pub fn f(v: &Vec<i32>) -> usize {\n    return v.len();\n}\n',
  });
  const parsed = toolFor('a.rs').parse(realStream(toolFor('a.rs'), repo, 'src/lib.rs'));
  assert.ok(parsed.length >= 1, 'clippy diagnostics were dropped');
  assert.strictEqual(parsed[0].rung, 'TRUE');
  assert.match(parsed[0].id, /^true\/clippy/);
  assert.ok(parsed.every((f) => f.line > 0));
});

test('two different eslint rules firing on the same line get distinct ids — a baselined finding must not silently swallow a different rule at the same location', () => {
  const eslint = toolFor('a.ts');
  const parsed = eslint.parse(JSON.stringify([
    {
      filePath: 'a.ts',
      messages: [
        { line: 10, ruleId: 'no-unused-vars', message: 'unused' },
        { line: 10, ruleId: 'no-eval', message: 'eval is evil' },
      ],
    },
  ]));
  assert.strictEqual(parsed.length, 2);
  const ids = parsed.map((f) => f.id);
  assert.strictEqual(ids[0], 'true/eslint:no-unused-vars');
  assert.strictEqual(ids[1], 'true/eslint:no-eval');
  assert.notStrictEqual(ids[0], ids[1]);
});
