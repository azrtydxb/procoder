// tests/toml.test.js
const test = require('node:test');
const assert = require('node:assert');
const { parseToml } = require('../hooks/checks/toml');

// Parsed tables are null-prototype (defence in depth), so spread before a
// deepStrictEqual against an object literal — it compares prototypes too.
test('parses scalars at the root', () => {
  assert.deepStrictEqual(
    { ...parseToml('level = "strict"\nenabled = true\nmax = 10\n') },
    { level: 'strict', enabled: true, max: 10 });
});

test('parses tables and dotted tables', () => {
  const out = parseToml('[thresholds]\nfunction_lines = 40\n\n[a.b]\nx = "y"\n');
  assert.strictEqual(out.thresholds.function_lines, 40);
  assert.strictEqual(out.a.b.x, 'y');
});

test('parses single-line string arrays', () => {
  const out = parseToml('[exclude]\npaths = ["vendor/", "dist/"]\n');
  assert.deepStrictEqual(out.exclude.paths, ['vendor/', 'dist/']);
});

test('ignores comments and blank lines', () => {
  const out = parseToml('# a comment\n\nlevel = "strict" # trailing\n');
  assert.strictEqual(out.level, 'strict');
});

test('malformed input yields an object, never a throw', () => {
  assert.doesNotThrow(() => parseToml('[[[garbage\nnot a pair\n= 5\n'));
  assert.deepStrictEqual({ ...parseToml('total nonsense') }, {});
});

test('a # inside a quoted string is not a comment', () => {
  assert.strictEqual(parseToml('token = "abc#def"\n').token, 'abc#def');
});

// Prototype pollution: a config file is untrusted input at a trust boundary.
// Each case must leave Object.prototype untouched — the canary proves it.
function canary(name, toml) {
  test(name, () => {
    delete Object.prototype.polluted;
    parseToml(toml);
    assert.strictEqual(Object.prototype.polluted, undefined, 'Object.prototype was polluted');
    assert.strictEqual({}.polluted, undefined);
    delete Object.prototype.polluted;
  });
}

canary('a [__proto__] table cannot pollute Object.prototype',
  '[__proto__]\npolluted = "yes"\n');
canary('a [exclude.__proto__] table cannot pollute Object.prototype',
  '[exclude.__proto__]\npolluted = "yes"\npaths = ["**/*"]\n');
canary('a bare __proto__ key cannot pollute Object.prototype',
  '__proto__ = "x"\npolluted = "yes"\n');
canary('a [constructor.prototype] table cannot pollute Object.prototype',
  '[constructor.prototype]\npolluted = "yes"\n');

test('polluting keys are dropped, not stored', () => {
  const out = parseToml('[exclude.__proto__]\npaths = ["**/*"]\n');
  assert.deepStrictEqual(out.exclude.paths, undefined);
  assert.strictEqual(Object.prototype.paths, undefined);
});

test('a __proto__ key does not become an own key either', () => {
  const out = parseToml('__proto__ = "x"\n');
  assert.ok(!Object.prototype.hasOwnProperty.call(out, '__proto__'));
});

// Multi-line arrays are ordinary TOML — dropping them silently is the defect
// this file fixes. Each shape must round-trip, not vanish.
test('parses a multi-line array with a trailing comma', () => {
  const out = parseToml('[exclude]\npaths = [\n  "vendor/",\n  "generated/",\n]\n');
  assert.deepStrictEqual(out.exclude.paths, ['vendor/', 'generated/']);
});

test('parses a multi-line array split mid-item', () => {
  const out = parseToml('key = ["one", "two",\n       "three"]\n');
  assert.deepStrictEqual(out.key, ['one', 'two', 'three']);
});

test('ignores a comment inside a multi-line array', () => {
  const out = parseToml('paths = [\n  "a/",\n  "b/",     # trailing comma is legal\n]\n');
  assert.deepStrictEqual(out.paths, ['a/', 'b/']);
});

test('a quoted string containing "]" inside a multi-line array is not mis-split', () => {
  const out = parseToml('paths = [\n  "weird]bracket/",\n  "b/",\n]\n');
  assert.deepStrictEqual(out.paths, ['weird]bracket/', 'b/']);
});

test('a quoted string containing "," inside a multi-line array is not mis-split', () => {
  const out = parseToml('paths = [\n  "a,b/",\n  "c/",\n]\n');
  assert.deepStrictEqual(out.paths, ['a,b/', 'c/']);
});

test('unsupported syntax warns with file and line, but still returns defaults-friendly output', () => {
  const originalWrite = process.stderr.write;
  let captured = '';
  process.stderr.write = (chunk) => { captured += chunk; return true; };
  let out;
  try {
    out = parseToml('level = "strict"\n{inline = "table"}\n', 'my-config.toml');
  } finally {
    process.stderr.write = originalWrite;
  }
  assert.strictEqual(out.level, 'strict');
  assert.match(captured, /my-config\.toml/);
  assert.match(captured, /\bline 2\b|:2:/);
});

test('an unterminated array does not throw, does not hang, and warns', () => {
  const originalWrite = process.stderr.write;
  let captured = '';
  process.stderr.write = (chunk) => { captured += chunk; return true; };
  let out;
  try {
    assert.doesNotThrow(() => { out = parseToml('paths = [\n  "a/",\n', 'weird.toml'); });
  } finally {
    process.stderr.write = originalWrite;
  }
  assert.deepStrictEqual({ ...out }, {});
  assert.match(captured, /weird\.toml/);
  assert.match(captured, /\bline 1\b|:1:/i);
});

// --- what a .procoder.toml plausibly contains ------------------------------
//
// Escapes and literal strings are here because an exclusion pattern is a path
// or a regex: "C:\\src\\" and 'C:\src\' are both things a Windows user writes.
// Everything below either round-trips exactly or refuses loudly — what must
// never happen is a value loading as something the file does not say.

// Captures stderr for one parse, so a warning can be asserted on.
function parseCapturing(text, fileName) {
  const originalWrite = process.stderr.write;
  let captured = '';
  process.stderr.write = (chunk) => { captured += chunk; return true; };
  let out;
  try {
    assert.doesNotThrow(() => { out = parseToml(text, fileName); });
  } finally {
    process.stderr.write = originalWrite;
  }
  return { out, captured };
}

// A key that must not load: absent, with a warning naming file and line.
function refuses(name, toml, key) {
  test(name, () => {
    const { out, captured } = parseCapturing(toml, 'my.toml');
    assert.strictEqual(out[key], undefined, `${key} loaded a value anyway`);
    assert.match(captured, /my\.toml:\d+:/);
  });
}

test('decodes escapes in a basic string', () => {
  assert.strictEqual(parseToml('s = "a\\"b"\n').s, 'a"b');
  assert.strictEqual(parseToml('s = "a\\\\b"\n').s, 'a\\b');
  assert.strictEqual(parseToml('s = "a\\nb\\tc"\n').s, 'a\nb\tc');
  assert.strictEqual(parseToml('s = "\\u00e9\\U0001F600"\n').s, '\u00e9\u{1F600}');
});

test('a Windows path round-trips in both string forms', () => {
  assert.strictEqual(parseToml('p = "C:\\\\src\\\\gen\\\\"\n').p, 'C:\\src\\gen\\');
  assert.strictEqual(parseToml("p = 'C:\\src\\gen\\'\n").p, 'C:\\src\\gen\\');
});

test('an escaped quote does not start a comment or end the string', () => {
  assert.strictEqual(parseToml('s = "a\\"#b" # trailing\n').s, 'a"#b');
});

test('escapes decode inside array items', () => {
  assert.deepStrictEqual(parseToml('paths = ["a\\tb", "c\\\\d"]\n').paths, ['a\tb', 'c\\d']);
});

refuses('an unknown escape refuses rather than guessing', 's = "a\\qb"\n', 's');
refuses('an unterminated basic string refuses', 's = "abc\n', 's');
refuses('a string with trailing junk refuses', 's = "abc" oops\n', 's');
refuses('a literal string with trailing junk refuses', "s = 'a'b'\n", 's');
refuses('an inline table refuses', 't = {a=1}\n', 't');
refuses('a date refuses', 'd = 1979-05-27\n', 'd');
refuses('a bare unquoted value refuses', 'level = strict\n', 'level');
refuses('an unsupported item inside an array refuses the whole key',
  'paths = ["a/", {b=1}]\n', 'paths');

test('a dotted key writes into the table it names', () => {
  const out = parseToml('thresholds.params = 4\n[exclude]\npaths = ["a/"]\n');
  assert.strictEqual(out.thresholds.params, 4);
  assert.deepStrictEqual(out.exclude.paths, ['a/']);
});

canary('a dotted key cannot pollute Object.prototype', '__proto__.polluted = "yes"\n');
canary('a nested dotted key cannot pollute Object.prototype',
  'a.constructor.prototype.polluted = "yes"\n');

