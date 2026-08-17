const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/rust');
const { DEFAULTS } = require('../hooks/checks/config');
const { assertLinear } = require('./perf-guard');

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

// Both directions of the shared principle: a rule sees code, not prose, and
// the string literals a sink is assembled from are code.
test('ignores rules named in comments, not the code beside them', () => {
  assert.ok(!ids('// never parse(input).unwrap() in a library').includes('true/unwrap-in-library'));
  assert.ok(!ids('/// Do not `sqlx::query(&format!("SELECT {}", id))`.').includes('safe/sql-injection'));
  assert.ok(!ids('// never Command::new("sh").arg("-c").arg(user_input)').includes('safe/shell-injection'));
  assert.ok(!ids('// never .danger_accept_invalid_certs(true)').includes('safe/tls-disabled'));
  assert.ok(!ids('// let token = rand::random::<u64>(); is not a CSPRNG').includes('safe/weak-random'));
  assert.ok(!ids('// println!("here") was removed').includes('alone/debug-leftover'));
  assert.ok(!ids('// unsafe { ptr::read(p) } needs a SAFETY note').includes('safe/unsafe-block'));

  assert.ok(ids('let v = parse(input).unwrap(); // never do this').includes('true/unwrap-in-library'));
  assert.ok(ids('sqlx::query(&format!("SELECT {}", id)); // bad').includes('safe/sql-injection'));
  assert.ok(ids('Command::new("sh").arg("-c").arg(user_input); // bad').includes('safe/shell-injection'));
  assert.ok(ids('.danger_accept_invalid_certs(true) // bad').includes('safe/tls-disabled'));
  assert.ok(ids('let token = rand::random::<u64>(); // bad').includes('safe/weak-random'));
  assert.ok(ids('println!("here"); // bad').includes('alone/debug-leftover'));
  assert.ok(ids('unsafe { ptr::read(p) } // no safety note').includes('safe/unsafe-block'));
});

// The SAFETY comment is the one place a comment is the subject of the rule,
// so it keeps reading raw lines.
test('a SAFETY comment still discharges the unsafe rule', () => {
  assert.ok(!ids('// SAFETY: p is non-null and aligned, checked above.\nunsafe { ptr::read(p) }')
    .includes('safe/unsafe-block'));
});

// A commented-out #[test] is not a test module, and must not blind the
// checker to the library code that follows it.
test('a commented-out test attribute does not open a test region', () => {
  const src = '// #[cfg(test)]\nmod helpers {\n    fn t() {\n        parse("x").unwrap();\n    }\n}\n';
  assert.ok(ids(src).includes('true/unwrap-in-library'));
});

test('keeps seeing sinks built inside string literals', () => {
  assert.ok(ids('sqlx::query(&format!("SELECT * FROM t WHERE id = {}", id))')
    .includes('safe/sql-injection'));
  assert.ok(ids('Command::new("bash").arg("-c").arg(format!("rm {}", dir))')
    .includes('safe/shell-injection'));
});

// The structural shapes taint.js closed, each with the safe twin that must
// stay silent. Every one of these reported nothing before the statement model,
// the block-aware merge, the path bindings and the return pass went in.
const SHAPES = [
  ['a field',
    'self.q = format!("SELECT id={}", id);\nconn.query(&self.q);',
    'self.q = "SELECT * FROM t";\nconn.query(&self.q);'],
  ['a helper\'s return value',
    'fn build(id: &str) -> String { return format!("SELECT id={}", id); }\nlet q = build(x);\nconn.query(&q);',
    'fn build() -> String { return "SELECT * FROM t".to_string(); }\nlet q = build();\nconn.query(&q);'],
  ['a return straight into the sink',
    'fn b(id: &str) -> String { return format!("SELECT id={}", id); }\nconn.query(&b(x));',
    'fn b() -> String { return "SELECT * FROM t".to_string(); }\nconn.query(&b());'],
  ['a binding made inside a branch',
    'let mut q = "SELECT".to_string();\nif x {\n    q = format!("SELECT id={}", id);\n}\nconn.query(&q);',
    'let mut q = "SELECT 1".to_string();\nif x {\n    q = "SELECT 2".to_string();\n}\nconn.query(&q);'],
  ['a value built in a loop',
    'let mut q = "SELECT".to_string();\nfor p in ps {\n    q = q + p;\n}\nconn.query(&q);',
    'let mut q = "SELECT".to_string();\nfor p in ps {\n    log(q.clone() + p);\n}\nconn.query(&q);'],
  ['a wrapped right-hand side',
    'let q =\n    format!("SELECT id={}", id);\nconn.query(&q);',
    'let q =\n    "SELECT ".to_string() + "* FROM t";\nconn.query(&q);'],
  ['a transformation at the sink',
    'let q = format!("SELECT id={}", id);\nconn.query(&q.trim());',
    'let q = "SELECT * FROM t";\nconn.query(&q.trim());'],
  ['a container',
    'let parts = vec!["SELECT id=", id];\nlet q = parts.join("");\nconn.query(&q);',
    'let parts = vec!["SELECT ", "* FROM t"];\nlet q = parts.join("");\nconn.query(&q);'],
  ['an inner binding of the same name',
    'let q = format!("SELECT id={}", id);\nfn g() { let q = "SELECT 1"; }\nconn.query(&q);',
    'let q = "SELECT 1";\nfn g() { let q = format!("SELECT id={}", id); }\nconn.query(&q);'],
];

for (const [what, unsafe, safe] of SHAPES) {
  test(`taint follows ${what}, and its safe twin stays silent`, () => {
    assert.ok(ids(unsafe).includes('safe/sql-injection'), `unsafe form of ${what} went unreported`);  // procoder: literal safe/sql-injection the id the shape must report
    assert.ok(!ids(safe).includes('safe/sql-injection'), `safe form of ${what} was reported`);
  });
}

// A multi-line literal's interior is data, and quoting code is most of what one
// is for. The scan reads statements across lines now, so a quoted assignment
// inside a raw string looked exactly like a real one.
test('code quoted inside a raw string is not code', () => {
  assert.ok(!ids('let doc = r#"\n  let q = format!("SELECT id={}", id);\n  conn.query(&q);\n"#;\n').includes('safe/sql-injection'));
  // The real thing on the same shape still fires.
  assert.ok(ids('let doc = "x";\nlet q = format!("SELECT id={}", id);\nconn.query(&q);\n').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the unquoted twin the pack must still report
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'rust');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.rs'), 'utf8'),
    { relPath: 'clean.rs', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.rs'), 'utf8'),
    { relPath: 'dirty.rs', config }).length >= 5);
});

// Perf guard — see tests/perf-guard.js for the bound and why it is relative.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check,
    relPath: 'x.rs',
    config,
    baseline: 'return;  ',
    units: [
      'Command::new("sh") ',
      'let token: ',
      'fn f(a) x ',
      'let x = f(a, b) + "s" + c; ',
      'let q = format!("SELECT {}", id); ',
      'sqlx::query(&q); ',
    ],
    sources: [
      'x'.repeat(100 * 1024),
      'Command::new("sh") '.repeat(5000),
      'let token: a '.repeat(8000),
    ],
  });
});

// Local taint: the assign-then-use form, at least as common as the inline one.
// Reported at the sink, naming the line the value was built on.
test('tracks a format! value from its binding to a sink', () => {
  const src = 'fn f(pool: &Pool, id: &str) {\n    let q = format!("SELECT * FROM t WHERE id={}", id);\n    sqlx::query(&q);\n}';
  const hit = check(src, { relPath: 'x.rs', config }).find((f) => f.id === 'safe/sql-injection');
  assert.ok(hit, 'no safe/sql-injection for the bind-then-use form');
  assert.strictEqual(hit.line, 3, 'reported at the sink, not the binding');
  assert.match(hit.message, /line 2/);

  assert.ok(ids('fn f(pool: &Pool, id: &str) {\n    let mut q = String::from("SELECT ") + id;\n    sqlx::execute(&q);\n}')
    .includes('safe/sql-injection'));
});

test('a bind-parameter or literal-only query binding stays silent', () => {
  assert.ok(!ids('fn f(pool: &Pool, id: &str) {\n    let q = "SELECT * FROM t WHERE id = $1";\n    sqlx::query(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('fn f(pool: &Pool) {\n    let q = "SELECT ".to_string() + "a, b";\n    sqlx::query(&q);\n}')
    .includes('safe/sql-injection'));
});

test('taint clears on a literal rebinding and does not leave its block', () => {
  assert.ok(!ids('fn f(pool: &Pool, id: &str) {\n    let mut q = format!("SELECT {}", id);\n    q = "SELECT * FROM t";\n    sqlx::query(&q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('fn a(id: &str) {\n    let q = format!("SELECT {}", id);\n}\nfn b(pool: &Pool, q: &str) {\n    sqlx::query(q);\n}')
    .includes('safe/sql-injection'));
});

// The 500-character span ceilings the SAFE rules used to carry: a sink whose
// interpolation sits further than that from the call was missed entirely.
const PAD = 'a'.repeat(600);

test('sees a sink whose interpolation is more than 500 characters from the call', () => {
  assert.ok(ids(`Command::new("sh").env("P", "${PAD}").arg("-c").arg(input);`).includes('safe/shell-injection'));
  assert.ok(ids(`let token: Wrapper<${PAD}> = rand::random();`).includes('safe/weak-random'));
});

test('the safe forms stay silent however long the arguments are', () => {
  assert.ok(!ids(`Command::new("git").env("P", "${PAD}").arg("-c").arg(input);`).includes('safe/shell-injection'));
  assert.ok(!ids(`let jitter: Wrapper<${PAD}> = rand::random();`).includes('safe/weak-random'));
});

// Defect: a sink whose arguments are wholly constant carries nothing
// untrusted, and a rung-1 rule that fires on obviously-safe code is what gets
// the whole pack turned off. An identifier, a nested call or a format! is
// data, and those still report.
test('a data sink whose arguments are wholly constant stays silent', () => {
  assert.ok(!ids('Command::new("sh").arg("-c").arg("ls /tmp").spawn();').includes('safe/shell-injection'));
  assert.ok(!ids('Command::new("bash").arg("-c").arg("ls -la").status();').includes('safe/shell-injection'));
});

test('the same sink still reports anything that is not a constant', () => {
  assert.ok(ids('Command::new("sh").arg("-c").arg(user_input).spawn();').includes('safe/shell-injection'));
  assert.ok(ids('Command::new("sh").arg("-c").arg(&format!("rm {}", dir)).spawn();').includes('safe/shell-injection'));
});

// Defect: `q = q + id` cleared the taint at the moment it should have been
// introduced — the right-hand side is two names with no literal, so the source
// patterns missed it and the clear-on-any-other-assignment rule fired.
test('an assignment whose right-hand side is a tainted name carries the taint', () => {
  assert.ok(ids('fn f(pool: &Pool, id: &str) {\n    let mut q = String::from("SELECT id=");\n    q = q + id;\n    sqlx::query(&q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('fn f(pool: &Pool, id: &str) {\n    let b = format!("x{}", id);\n    let a = b;\n    sqlx::query(&a);\n}')
    .includes('safe/sql-injection'));
});

test('an append carries the taint of the whole value it builds', () => {
  assert.ok(ids('fn f(pool: &Pool, id: &str) {\n    let mut q = String::from("SELECT id=");\n    q += id;\n    sqlx::query(&q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('fn f(pool: &Pool, id: &str) {\n    let mut q = format!("SELECT {}", id);\n    q += " LIMIT 1";\n    sqlx::query(&q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('fn f(pool: &Pool) {\n    let mut q = String::from("SELECT a");\n    q += " FROM t";\n    sqlx::query(&q);\n}')
    .includes('safe/sql-injection'));
});

// Defect: a parameter is a fresh binding. One that happens to share a name
// with a tainted variable in an enclosing scope must start clean.
test('a parameter shadowing a tainted name starts clean', () => {
  assert.ok(!ids('fn f(pool: &Pool, id: &str) {\n    let q = format!("SELECT {}", id);\n    fn g(q: String) {\n        sqlx::query(&q);\n    }\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('fn f(pool: &Pool, id: &str) {\n    let q = format!("SELECT {}", id);\n    fn g(other: String) {\n        sqlx::query(&q);\n    }\n}')
    .includes('safe/sql-injection'));
});

// ---------------------------------------------------------------------------
// Round 3. Item 1: a constant table spliced into an otherwise parameterized
// query. `const` and `static` were also missing from the binding recogniser,
// so `const TABLE: &str = "users"` matched nothing at all.
test("a constant fragment in a parameterized query stays silent", () => {
  assert.ok(!ids('const TABLE: &str = "users";\nlet rows = sqlx::query(&format!("SELECT * FROM {} WHERE id = $1", TABLE)).bind(id);\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

test("the same shape reports when the fragment is not provably constant", () => {
  assert.ok(ids('let rows = sqlx::query(&format!("SELECT * FROM t WHERE id = {}", user_id));\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('const TABLE: &str = "users";\nlet q = format!("SELECT * FROM {} WHERE n = {}", TABLE, name);\nlet r = sqlx::query(&q);\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});
