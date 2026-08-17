// tests/exports.test.js
const test = require('node:test');
const assert = require('node:assert');
const { exportsIn, buildIndex, deadExports } = require('../hooks/checks/exports');

// A name plus the line it was declared on: a rot report that points at the
// wrong line sends the reader hunting through a file for a symbol that is
// three declarations further down, and they stop trusting the report.
const declared = (relPath, ...lines) =>
  exportsIn(relPath, lines.join('\n')).map((s) => `${s.name}:${s.line}`);

// An index over an in-memory tree. A path the caller never listed reads as
// null, which is how the real CLI reports a file it could not read.
const indexOf = (tree) =>
  buildIndex(Object.keys(tree), (relPath) =>
    (Object.prototype.hasOwnProperty.call(tree, relPath) ? tree[relPath] : null));

const dead = (tree, options) => deadExports(indexOf(tree), options || {});
const reported = (tree, options) => dead(tree, options).map((f) => f.message);

test('an ES module offers its functions, values, classes and types by declaration', () => {
  assert.deepStrictEqual(
    declared('src/a.ts',
      'export async function fetchIt() {}',
      'export const NAME = 1;',
      'export abstract class Widget {}',
      'export interface Shape {}',
      'export type Alias = string;',
      'export enum Mode {}'),
    ['fetchIt:1', 'NAME:2', 'Widget:3', 'Shape:4', 'Alias:5', 'Mode:6']);
});

// Half the Node tooling in the wild still assigns onto `exports` one property
// at a time, and a scanner that only knew ESM reported those files as having
// no surface at all — which reads as "nothing here is dead" and is a lie.
test('a CommonJS file offers each property it assigns onto exports', () => {
  assert.deepStrictEqual(
    declared('src/a.js', 'const x = 1;', 'exports.legacy = x;', 'exports.other = 2;'),
    ['legacy:2', 'other:3']);
});

// The dominant shape in this repository's own hooks: one object literal at the
// bottom of the file, spread over as many lines as it has names. The keys are
// the exported names, so `{ run: runCheck }` publishes `run` — reporting
// `runCheck` instead would make every renamed export a phantom dead symbol.
test('a module.exports object publishes its keys, renamed ones under the public name', () => {
  assert.deepStrictEqual(
    declared('src/a.js',
      'function runCheck() {}',
      'module.exports = {',
      '  run: runCheck,',
      '  alpha,',
      '  beta, gamma,',
      '};'),
    ['run:2', 'alpha:2', 'beta:2', 'gamma:2']);
});

// The block is bounded so a `{` that never closes cannot swallow the rest of
// the file and turn every identifier below it into an exported name.
test('a module.exports block stops after its bounded window', () => {
  const many = Array.from({ length: 60 }, (_, i) => `  name${i},`);
  const names = exportsIn('src/a.js', ['module.exports = {', ...many].join('\n'));
  assert.ok(names.length < many.length, 'an unclosed block read the whole file');
  assert.ok(names.some((s) => s.name === 'name0'));
  assert.ok(!names.some((s) => s.name === 'name59'));
});

// Python's surface is what sits at column zero. A method nested inside a class
// is reached through its owner, and reporting it separately would tell people
// to delete methods that are called on every request.
test('a Python module offers its top-level defs and classes, not its nested ones', () => {
  assert.deepStrictEqual(
    declared('src/a.py',
      'async def fetch_it():',
      '    pass',
      'class Widget:',
      '    def method(self):',
      '        pass'),
    ['fetch_it:1', 'Widget:3']);
});

test('a Go file offers only the capitalised names the language itself exports', () => {
  assert.deepStrictEqual(
    declared('src/a.go',
      'func Handle() {}',
      'func internal() {}',
      'type Config struct{}',
      'const Max = 3',
      'var Registry = 1'),
    ['Handle:1', 'Config:3', 'Max:4', 'Registry:5']);
});

test('a Rust file offers only what it marks pub', () => {
  assert.deepStrictEqual(
    declared('src/a.rs',
      'pub async fn parse() {}',
      'fn hidden() {}',
      'pub struct Cfg;',
      'pub trait Sink {}',
      'pub enum Kind {}'),
    ['parse:1', 'Cfg:3', 'Sink:4', 'Kind:5']);
});

test('a JVM file offers its public types wherever they are indented', () => {
  assert.deepStrictEqual(
    declared('src/A.java',
      'public final class Widget {',
      '  public interface Sink {}',
      '}'),
    ['Widget:1', 'Sink:2']);
  assert.deepStrictEqual(
    declared('src/A.kt', 'public enum Mode {}', 'internal class Hidden {}'), ['Mode:1']);
});

// C# is the one language here where a struct is a public type like any other,
// so it gets its own pattern rather than borrowing the JVM one.
test('a C# file offers its public structs alongside its classes', () => {
  assert.deepStrictEqual(
    declared('src/A.cs', 'public sealed struct Point {}', 'public static class Util {}'),
    ['Point:1', 'Util:2']);
});

// Extension decides the language, and a file whose extension maps to nothing
// must not be parsed as JavaScript on a hunch.
test('a file in no known language offers nothing', () => {
  assert.deepStrictEqual(exportsIn('notes/a.txt', 'export function nope() {}'), []);
});

test('a symbol another file calls is not reported', () => {
  assert.deepStrictEqual(
    reported({ 'src/a.js': 'export function used() {}', 'src/b.js': 'used();' }), []);
});

// The plain deletion tier: nothing anywhere in the scanned set says this name.
// It carries the file it came from because the finding travels away from the
// per-file loop that every other check in the engine runs inside.
test('a symbol nothing else mentions is reported as a deletion, not a question', () => {
  const [f, ...rest] = dead({ 'src/a.js': 'const x = 1;\nexport function orphan() {}' });
  assert.deepStrictEqual(rest, []);
  assert.strictEqual(f.id, 'alone/dead-export');
  assert.strictEqual(f.rung, 'ALONE');
  assert.strictEqual(f.file, 'src/a.js');
  assert.strictEqual(f.line, 2);
  assert.strictEqual(f.needsConfirmation, false);
  assert.match(f.message, /orphan is exported and mentioned nowhere else/);
});

// The honest tier. A name reached through a route table, a DI container or
// getattr has callers this index cannot follow, so it is asked about rather
// than declared dead — silently dropping it would be the tool deciding on its
// own that a symbol is fine, and asserting it is dead would delete live code.
test('a symbol reached only by its name in a string is reported as needing confirmation', () => {
  const src = 'export function routeIt() {}\nregister("routeIt", routeIt);';
  const [f, ...rest] = dead({ 'src/a.js': src });
  assert.deepStrictEqual(rest, []);
  assert.strictEqual(f.needsConfirmation, true);
  assert.match(f.message, /names it in a string/);
  assert.match(f.fix, /confirm/);
});

// Worth pinning because it is weaker than the message it prints: the quoted
// tier only ever fires on a string in the symbol's OWN file. A string in any
// other file is also a word in that file, so it counts as a plain mention and
// the symbol drops out of the report entirely. The message says "mentioned
// outside this file only inside a string", which cannot actually happen.
test('a name in a string in another file counts as an ordinary mention and is not reported', () => {
  assert.deepStrictEqual(
    reported({
      'src/a.js': 'export function routeIt() {}',
      'src/b.js': 'const routes = { home: "routeIt" };',
    }), []);
});

// Documents are kept out of the quoted scan on purpose: prose is full of
// apostrophes, and a sentence between two of them made every candidate in this
// repository come back "needs confirmation". A name a README explains is still
// a mention, so it drops out of the report rather than changing tier.
test('a name only a document mentions is an ordinary mention, not a string reference', () => {
  assert.deepStrictEqual(
    reported({
      'src/a.js': 'export function widgetize() {}',
      'README.md': 'Call `widgetize` to build one.',
    }), []);
});

// An entry point's callers live outside the scan by definition — a published
// package's consumers, a `main`, a route registration. Reporting one is the
// single failure that makes people stop running the command.
test('a symbol exported from a conventional entry file is never reported', () => {
  for (const entry of ['src/index.js', 'src/index.ts', 'src/index.mjs']) {
    assert.deepStrictEqual(reported({ [entry]: 'export function api() {}' }), []);
  }
  assert.deepStrictEqual(reported({ 'src/lib.rs': 'pub fn api() {}' }), []);
  assert.deepStrictEqual(reported({ 'src/mod.rs': 'pub fn api() {}' }), []);
  assert.deepStrictEqual(reported({ 'pkg/__init__.py': 'def api():\n    pass' }), []);
});

test('a symbol exported from a file the caller declared an entry point is never reported', () => {
  const tree = { 'src/cli.js': 'export function main() {}' };
  assert.strictEqual(reported(tree).length, 1, 'the fixture must be reportable without the entry');
  assert.deepStrictEqual(reported(tree, { entryFiles: new Set(['src/cli.js']) }), []);
});

// The CLI hands over paths it walked, and a file can vanish or be unreadable
// between the walk and the read. Throwing there would lose the whole scan over
// one deleted file.
test('a file the reader cannot open contributes nothing and does not stop the scan', () => {
  const index = buildIndex(['src/a.js', 'src/gone.js'],
    (relPath) => (relPath === 'src/a.js' ? 'export function solo() {}' : null));
  assert.strictEqual(index.exports.length, 1);
  assert.strictEqual(index.quoted.has('src/gone.js'), false);
  assert.strictEqual(deadExports(index, {}).length, 1);
});

// Mentions are counted per word, not per substring, so a longer identifier that
// merely contains the name does not keep it alive. The real boundary is the
// tokenizer's, not the language's: prose and package names split on a hyphen,
// so `left-pad` in a document genuinely does count as a mention of `pad`. That
// is a miss in the safe direction — an unreported symbol, never a wrong delete.
test('a longer identifier containing the name does not count as a mention of it', () => {
  assert.strictEqual(
    reported({ 'src/a.js': 'export function pad() {}', 'src/b.js': 'padStart(x);' }).length, 1);
  assert.deepStrictEqual(
    reported({ 'src/a.js': 'export function pad() {}', 'docs/x.md': 'we use left-pad here' }), []);
});
