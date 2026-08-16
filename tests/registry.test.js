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
  assert.strictEqual(toolFor('a.rs').name, 'clippy');
  assert.strictEqual(toolFor('README.md'), null);
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
