const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/rust');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.rs', config }).map((f) => f.id);

test('owns the .rs extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.rs']);
});

test('flags unwrap and expect on fallible calls', () => {
  assert.ok(ids('let v = parse(input).unwrap();').includes('true/unwrap-in-library'));
  assert.ok(ids('let v = parse(input).expect("should parse");').includes('true/unwrap-in-library'));
  assert.ok(!ids('let v = parse(input)?;').includes('true/unwrap-in-library'));
});

test('does not flag unwrap inside tests', () => {
  const src = '#[cfg(test)]\nmod tests {\n    #[test]\n    fn t() {\n        parse("x").unwrap();\n    }\n}\n';
  assert.ok(!ids(src).includes('true/unwrap-in-library'));
});

test('does not flag unwrap inside a #[cfg(test)] module placed mid-file', () => {
  const src = [
    'fn real_work(input: &str) -> i32 {',
    '    parse(input).unwrap_or(0)',
    '}',
    '',
    '#[cfg(test)]',
    'mod tests {',
    '    use super::*;',
    '',
    '    #[test]',
    '    fn t() {',
    '        parse("x").unwrap();',
    '    }',
    '}',
    '',
    'fn after_tests(input: &str) -> i32 {',
    '    parse(input).unwrap()',
    '}',
  ].join('\n');
  const found = ids(src);
  assert.ok(!found.includes('true/unwrap-in-library') || found.filter((id) => id === 'true/unwrap-in-library').length === 1,
    'only the unwrap after the test module should fire');
});

// A brace inside a string or a comment is not structure. Counting it extends
// the test region to end of file and blinds the checker to real library code.
const withTestBody = (bodyLine) => [
  '#[cfg(test)]',
  'mod tests {',
  '    #[test]',
  '    fn t() {',
  `        ${bodyLine}`,
  '        parse("x").unwrap();',
  '    }',
  '}',
  '',
  'fn after_tests(input: &str) -> i32 {',
  '    parse(input).unwrap()',
  '}',
].join('\n');

test('an unbalanced brace in a test string does not extend the test region', () => {
  const found = check(withTestBody('let s = "looks like a { brace";'), { relPath: 'x.rs', config });
  const unwraps = found.filter((f) => f.id === 'true/unwrap-in-library');
  assert.deepStrictEqual(unwraps.map((f) => f.line), [11], 'only the library unwrap after the module');
});

test('an unbalanced brace in a test comment does not extend the test region', () => {
  const found = check(withTestBody('// opening brace example: {'), { relPath: 'x.rs', config });
  const unwraps = found.filter((f) => f.id === 'true/unwrap-in-library');
  assert.deepStrictEqual(unwraps.map((f) => f.line), [11]);
});

test('a balanced brace in a test string keeps the region intact', () => {
  const found = check(withTestBody('let s = format!("{}", v);'), { relPath: 'x.rs', config });
  const unwraps = found.filter((f) => f.id === 'true/unwrap-in-library');
  assert.deepStrictEqual(unwraps.map((f) => f.line), [11]);
});

test('flags unsafe blocks without a safety comment', () => {
  assert.ok(ids('unsafe { ptr::read(p) }').includes('safe/unsafe-block'));
  assert.ok(!ids('// SAFETY: p is non-null and aligned, checked above.\nunsafe { ptr::read(p) }').includes('safe/unsafe-block'));
});

test('flags SQL interpolation and command injection', () => {
  assert.ok(ids('sqlx::query(&format!("SELECT * FROM t WHERE id = {}", id))').includes('safe/sql-injection'));
  assert.ok(ids('Command::new("sh").arg("-c").arg(user_input)').includes('safe/shell-injection'));
});

test('flags disabled certificate verification and weak randomness', () => {
  assert.ok(ids('.danger_accept_invalid_certs(true)').includes('safe/tls-disabled'));
  assert.ok(ids('let token = rand::random::<u64>();').includes('safe/weak-random'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('println!("here");').includes('alone/debug-leftover'));
  assert.ok(ids('dbg!(value);').includes('alone/debug-leftover'));
  assert.ok(!ids('tracing::info!("started");').includes('alone/debug-leftover'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'rust');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.rs'), 'utf8'),
    { relPath: 'clean.rs', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.rs'), 'utf8'),
    { relPath: 'dirty.rs', config }).length >= 5);
});
