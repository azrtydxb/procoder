const test = require('node:test');
const assert = require('node:assert');
const { packFor, toolFor, PACKS } = require('../hooks/checks/registry');

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
