// tests/lang-ts.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/ts');
const { DEFAULTS } = require('../hooks/checks/config');
const { assertLinear } = require('./perf-guard');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.ts', config }).map((f) => f.id);

test('declares the extensions it owns', () => {
  assert.ok(EXTENSIONS.includes('.ts') && EXTENSIONS.includes('.jsx'));
});

test('flags SQL built by string concatenation or template', () => {
  assert.ok(ids('db.query(`SELECT * FROM users WHERE id = ${id}`)').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
  assert.ok(ids('db.query("SELECT * FROM t WHERE a = " + a)').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
  assert.ok(!ids('db.query("SELECT * FROM users WHERE id = ?", [id])').includes('safe/sql-injection'));
});

test('flags SQL concatenation where the literal holds the other quote character', () => {
  assert.ok(ids(`db.query("SELECT * FROM u WHERE id = '" + id + "'")`).includes('safe/sql-injection'));  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
  assert.ok(ids(`db.query('SELECT * FROM u WHERE name = "' + name + '"')`).includes('safe/sql-injection'));  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
  assert.ok(ids('db.query(`SELECT * FROM u WHERE id = \'${id}\'`)').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
  assert.ok(!ids('db.query("SELECT * FROM users WHERE id = $1", [id])').includes('safe/sql-injection'));
  assert.ok(!ids('const msg = "hello " + name;').includes('safe/sql-injection'));
  assert.ok(!ids('// build the query with "SELECT " + cols').includes('safe/sql-injection'));
});

test('flags XSS sinks and dynamic evaluation', () => {
  assert.ok(ids('el.innerHTML = userInput;').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the TS snippet handed to the pack as input
  assert.ok(ids('<div dangerouslySetInnerHTML={{ __html: body }} />').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the JSX variant of the same input
  assert.ok(ids('eval(payload)').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.ok(!ids('el.textContent = userInput;').includes('safe/xss-sink'));
});

test('flags disabled TLS verification and weak randomness for tokens', () => {
  assert.ok(ids('rejectUnauthorized: false').includes('safe/tls-disabled'));  // procoder: literal safe/tls-disabled scanner input for that rule, not an instance of it
  assert.ok(ids('const token = Math.random().toString(36);').includes('safe/weak-random'));  // procoder: literal safe/weak-random scanner input for that rule, not an instance of it
  assert.ok(!ids('const jitter = Math.random() * 100;').includes('safe/weak-random'));
});

test('flags shell injection', () => {
  assert.ok(ids('exec(`git log ${branch}`);').includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(ids("execSync('rm -rf ' + dir);").includes('safe/shell-injection'));  // procoder: literal safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(ids("spawn('sh', [cmd], { shell: true });").includes('safe/shell-injection'));  // procoder: literal safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(!ids("execFile('git', ['log', branch]);").includes('safe/shell-injection'));
  assert.ok(!ids("spawn('ls', [dir]);").includes('safe/shell-injection'));
});

test('flags shell injection where the literal holds the other quote character', () => {
  assert.ok(ids(`exec("rm '" + x + "'")`).includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(ids(`exec('rm "' + x + '"')`).includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(ids('exec(`rm \'${x}\'`)').includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(!ids("execFile('git', ['log', branch]);").includes('safe/shell-injection'));
  assert.ok(!ids('const msg = "hello " + name;').includes('safe/shell-injection'));
});

test('flags swallowed errors and floating promises', () => {
  assert.ok(ids('try { go(); } catch (e) {}').includes('true/swallowed-error'));  // procoder: literal true/swallowed-error the empty-catch snippet the pack must flag
  assert.ok(ids('try { go(); } catch (e) { /* ignore */ }').includes('true/swallowed-error'));  // procoder: literal true/swallowed-error the comment-only-catch variant of it
  assert.ok(!ids('try { go(); } catch (e) { logger.error(e); }').includes('true/swallowed-error'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('console.log("here")').includes('alone/debug-leftover'));  // procoder: literal alone/debug-leftover scanner input for that rule, not an instance of it
  assert.ok(ids('debugger;').includes('alone/debug-leftover'));  // procoder: literal alone/debug-leftover scanner input for that rule, not an instance of it
  assert.ok(!ids('logger.info("started")').includes('alone/debug-leftover'));
});

test('flags shape violations via the shared metrics', () => {
  const long = 'function big(a, b, c, d, e) {\n' + '  work();\n'.repeat(60) + '}\n';
  const found = ids(long);
  assert.ok(found.includes('obvious/function-too-long'));
  assert.ok(found.includes('obvious/too-many-params'));
});

test('flags nested ternaries', () => {
  assert.ok(ids('const x = a ? b ? 1 : 2 : 3;').includes('obvious/nested-ternary'));
  assert.ok(!ids('const x = a ? 1 : 2;').includes('obvious/nested-ternary'));
});

// Both directions of the shared principle: a rule sees code, not prose or a
// pattern that merely spells the sink, and string literals are code.
test('ignores rules named in comments, not the code beside them', () => {
  assert.ok(!ids('// never el.innerHTML = userInput;').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the commented sink the pack must NOT flag
  assert.ok(!ids('// never eval(payload)').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval the commented eval the pack must NOT flag
  assert.ok(!ids('/* db.query(`SELECT ${id}`) is how not to do it */').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the commented query the pack must NOT flag
  assert.ok(!ids("// never exec('rm -rf ' + dir)").includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection the commented exec the pack must NOT flag
  assert.ok(!ids('// rejectUnauthorized: false is never acceptable').includes('safe/tls-disabled'));  // procoder: literal safe/tls-disabled the commented TLS option the pack must NOT flag
  assert.ok(!ids('// const token = Math.random(); is not a secret').includes('safe/weak-random'));  // procoder: literal safe/weak-random the commented Math.random the pack must NOT flag
  assert.ok(!ids('// console.log("here") was removed').includes('alone/debug-leftover'));  // procoder: literal alone/debug-leftover the commented console.log the pack must NOT flag
  assert.ok(!ids('// const x = a ? b ? 1 : 2 : 3;').includes('obvious/nested-ternary'));

  assert.ok(ids('el.innerHTML = userInput; // never do this').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the same sink uncommented, which the pack must flag
  assert.ok(ids('eval(payload); // bad').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval the same eval uncommented, which the pack must flag
  assert.ok(ids('db.query(`SELECT ${id}`); // bad').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the same query uncommented, which the pack must flag
  assert.ok(ids("exec('rm -rf ' + dir); // bad").includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection the same exec uncommented, which the pack must flag
  assert.ok(ids('rejectUnauthorized: false, // bad').includes('safe/tls-disabled'));  // procoder: literal safe/tls-disabled the same TLS option uncommented, which the pack must flag
  assert.ok(ids('const token = Math.random(); // bad').includes('safe/weak-random'));  // procoder: literal safe/weak-random the same Math.random uncommented, which the pack must flag
  assert.ok(ids('console.log("here"); // bad').includes('alone/debug-leftover'));  // procoder: literal alone/debug-leftover the same console.log uncommented, which the pack must flag
  assert.ok(ids('const x = a ? b ? 1 : 2 : 3; // bad').includes('obvious/nested-ternary'));  // procoder: literal obvious/nested-ternary the same nested ternary uncommented, which the pack must flag
});

// A regex spelling a sink is a matcher for it, not a call to it — the pattern
// a linter, a WAF rule or a codemod is built from.
test('a sink named inside a regex literal is not a sink', () => {
  assert.ok(!ids('const re = /dangerouslySetInnerHTML|\\.innerHTML\\s*=/;').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the regex that matches the sink, given to the pack as input
  assert.ok(!ids('const re = /\\beval\\(/;').includes('safe/dynamic-eval'));
  assert.ok(!ids('const re = /console\\.log\\(/;').includes('alone/debug-leftover'));
  // Division is not a regex, and the code after it still counts.
  assert.ok(ids('const r = a / b; eval(payload);').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval the division-then-eval snippet the pack must still flag
});

// The direction ts used to lose: build tooling and SSR assemble code as
// strings, and a sink assembled there runs exactly as written.
test('sees sinks assembled inside string and template literals', () => {
  assert.ok(ids('render(`el.innerHTML = "${raw}"`)').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the sink assembled inside a template literal
  assert.ok(ids('emit("document.write(" + payload + ")")').includes('safe/xss-sink'));  // procoder: literal safe/xss-sink the sink assembled by string concatenation
  assert.ok(ids('db.query("SELECT * FROM t WHERE a = " + a)').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the query assembled by string concatenation
});

// A ternary is an operator of the grammar: it cannot occur inside a literal,
// so this rule alone keeps stripping strings as well as comments.
test('a ternary-shaped string is not a nested ternary', () => {
  assert.ok(!ids('const label = "a ? b ? 1 : 2 : 3";').includes('obvious/nested-ternary'));
  assert.ok(ids('const x = a ? b ? 1 : 2 : 3;').includes('obvious/nested-ternary'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'ts');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.ts'), 'utf8'),
    { relPath: 'clean.ts', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.ts'), 'utf8'),
    { relPath: 'dirty.ts', config }).length >= 6);
});

// Perf guard — see tests/perf-guard.js for the bound and why it is relative.
// The last two units are the taint scan's own adversarial shapes: a statement
// stream of assignments and sinks, and an assignment whose right-hand side is
// a concatenation. The word runs are the ones FUNCTION_SIGNATURE used to retry
// its `\w+` branch over from every offset (54ms at 8KB, 9,041ms at 100KB), and
// that the Go pack's taint assignment pattern later cost 4,887ms on.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check,
    relPath: 'x.ts',
    config,
    baseline: 'let a = 1; ',
    units: [
      'spawn(',
      'a ? b ? c ',
      '?',
      'query(`x ',
      'exec("x" ',
      'var x = f(a, b) + "s" + c; ',
      'const q = "SELECT " + id; db.query(q); ',
      'const a = "x" + b; ',
    ],
    sources: [
      'x'.repeat(100 * 1024), '$a'.repeat(50 * 1024), 'x'.repeat(100 * 1024) + '(a){',
      // Nested calls that DO close, so paramSpans records a span for each:
      // the shape a slice-per-span implementation of spans.js would be
      // quadratic on. 14,000 levels deep at 100KB.
      'spawn('.repeat(14000) + ')'.repeat(14000),
    ],
  });
});

// The scan still has to find the signatures a per-line anchor would lose: a
// method whose name is preceded by `$`, and one that is not the first match on
// its line.
test('finds signatures whatever the identifier is preceded by', () => {
  assert.ok(ids('class C { $sink(a, b, c, d, e, f, g) {\n  return a;\n} }').includes('obvious/too-many-params'));
  assert.ok(ids('const f = () => { g(a, b, c, d, e, f) {\n  return a;\n} }').includes('obvious/too-many-params'));
});

// Local taint: the assign-then-use form, at least as common as the inline one.
// Reported at the sink, naming the line the value was built on.
test('tracks a string built by concatenation from its assignment to a sink', () => {
  const src = 'function f(db, id) {\n  const q = "SELECT * FROM t WHERE id=" + id;\n  db.query(q);\n}';
  const sql = check(src, { relPath: 'x.ts', config });
  const hit = sql.find((f) => f.id === 'safe/sql-injection');
  assert.ok(hit, `no safe/sql-injection: ${sql.map((f) => f.id).join(', ')}`);
  assert.strictEqual(hit.line, 3, 'reported at the sink, not the assignment');
  assert.match(hit.message, /line 2/);

  assert.ok(ids('function f(id) {\n  const q = `SELECT * FROM t WHERE id=${id}`;\n  db.query(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('function f(dir) {\n  const cmd = "ls " + dir;\n  exec(cmd);\n}')
    .includes('safe/shell-injection'));
  assert.ok(ids('function f(dir) {\n  const cmd = `ls ${dir}`;\n  execSync(cmd);\n}')
    .includes('safe/shell-injection'));
});

test('a parameterized or literal-only variable reaching a sink stays silent', () => {
  assert.ok(!ids('function f(db, id) {\n  const q = "SELECT * FROM t WHERE id = ?";\n  db.query(q, [id]);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('function f(db) {\n  const q = "SELECT " + "a, b" + " FROM t";\n  db.query(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('function f() {\n  const cmd = "ls -la";\n  exec(cmd);\n}')
    .includes('safe/shell-injection'));
});

test('taint clears on a literal reassignment and does not leave its block', () => {
  assert.ok(!ids('function f(db, id) {\n  let q = "SELECT t WHERE id=" + id;\n  q = "SELECT * FROM t";\n  db.query(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('function a(id) {\n  const q = "SELECT " + id;\n}\nfunction b(db, q) {\n  db.query(q);\n}')
    .includes('safe/sql-injection'));
});

// The 500-character span ceilings the SAFE rules used to carry: a sink whose
// interpolation sits further than that from the call was missed entirely, and
// a long literal is exactly where a stray concatenation hides.
const PAD = 'a'.repeat(600);

test('sees a sink whose interpolation is more than 500 characters from the call', () => {
  assert.ok(ids(`db.query("SELECT ${PAD} WHERE id = " + id)`).includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the over-500-character scanner input for that rule, not an instance of it
  assert.ok(ids('db.query(`SELECT ' + PAD + ' WHERE id = ${id}`)').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the over-500-character scanner input for that rule, not an instance of it
  assert.ok(ids(`execSync("rm -rf ${PAD}/" + dir)`).includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection the over-500-character scanner input for that rule, not an instance of it
  assert.ok(ids(`spawn('sh', ['${PAD}', cmd], { shell: true })`).includes('safe/shell-injection'));  // procoder: literal safe/shell-injection the over-500-character scanner input for that rule, not an instance of it
});

test('the safe forms stay silent however long the arguments are', () => {
  assert.ok(!ids(`db.query("SELECT ${PAD} WHERE id = ?", [id])`).includes('safe/sql-injection'));
  assert.ok(!ids(`spawn('ls', ['${PAD}', dir])`).includes('safe/shell-injection'));
  assert.ok(!ids(`execFile('git', ['log', '${PAD}', branch])`).includes('safe/shell-injection'));
});
