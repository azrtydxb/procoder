const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/dotnet');
const { DEFAULTS } = require('../hooks/checks/config');
const { assertLinear } = require('./perf-guard');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'X.cs', config }).map((f) => f.id);

test('owns the .cs extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.cs']);
});

test('flags SQL string building', () => {
  assert.ok(ids('new SqlCommand($"SELECT * FROM t WHERE id = {id}", conn);').includes('safe/sql-injection'));
  assert.ok(ids('cmd.CommandText = "SELECT * FROM t WHERE id = " + id;').includes('safe/sql-injection'));
  assert.ok(!ids('cmd.Parameters.AddWithValue("@id", id);').includes('safe/sql-injection'));
});

test('does not flag FromSqlInterpolated, the safe EF Core API', () => {
  // FromSqlInterpolated takes a C# interpolated string by design and
  // parameterizes it internally — it must not be confused with the unsafe
  // FromSqlRaw/string-concatenation path.
  assert.ok(!ids('context.Users.FromSqlInterpolated($"SELECT * FROM Users WHERE Id = {id}");').includes('safe/sql-injection'));
});

test('flags unsafe deserialization and weak crypto', () => {
  assert.ok(ids('var f = new BinaryFormatter();').includes('safe/unsafe-deserialize'));
  assert.ok(ids('MD5.Create()').includes('safe/weak-hash'));
  assert.ok(ids('var token = new Random().Next().ToString();').includes('safe/weak-random'));
  assert.ok(!ids('SHA256.Create()').includes('safe/weak-hash'));
});

test('flags disabled certificate validation', () => {
  assert.ok(ids('ServerCertificateValidationCallback = (a, b, c, d) => true;').includes('safe/tls-disabled'));
});

test('flags shell injection', () => {
  assert.ok(ids('Process.Start($"git log {branch}");').includes('safe/shell-injection'));
  assert.ok(ids('Process.Start("cmd.exe", "/c " + cmd);').includes('safe/shell-injection'));
  assert.ok(ids('var psi = new ProcessStartInfo { FileName = "cmd.exe", Arguments = $"/c {cmd}", UseShellExecute = true };').includes('safe/shell-injection'));
  assert.ok(!ids('Process.Start("git", "log");').includes('safe/shell-injection'));
  assert.ok(!ids('var psi = new ProcessStartInfo { FileName = "git", ArgumentList = { "log", branch }, UseShellExecute = false };').includes('safe/shell-injection'));
});

test('flags swallowed exceptions and leftover debugging', () => {
  assert.ok(ids('try { Go(); } catch (Exception) { }').includes('true/swallowed-error'));  // procoder: literal true/swallowed-error the C# snippet handed to the pack as input
  assert.ok(ids('Console.WriteLine("here");').includes('alone/debug-leftover'));
  assert.ok(!ids('_logger.LogInformation("started");').includes('alone/debug-leftover'));
});

// Both directions of the shared principle: a rule sees code, not prose, and
// the string literals a sink is assembled from are code.
test('ignores rules named in comments, not the code beside them', () => {
  assert.ok(!ids('// never context.Users.FromSqlRaw($"SELECT * FROM Users WHERE Id = {id}")')
    .includes('safe/sql-injection'));
  assert.ok(!ids('// never new BinaryFormatter()').includes('safe/unsafe-deserialize'));
  assert.ok(!ids('/* MD5.Create() is not a password hash */').includes('safe/weak-hash'));
  assert.ok(!ids('// var token = new Random().Next(); is predictable').includes('safe/weak-random'));
  assert.ok(!ids('// never Process.Start($"git log {branch}");').includes('safe/shell-injection'));
  assert.ok(!ids('// ServerCertificateValidationCallback = (a, b, c, d) => true; is never ok')
    .includes('safe/tls-disabled'));
  assert.ok(!ids('// Console.WriteLine("here") was removed').includes('alone/debug-leftover'));

  assert.ok(ids('context.Users.FromSqlRaw($"SELECT * FROM Users WHERE Id = {id}"); // never do this')
    .includes('safe/sql-injection'));
  assert.ok(ids('var f = new BinaryFormatter(); // bad').includes('safe/unsafe-deserialize'));
  assert.ok(ids('MD5.Create(); // bad').includes('safe/weak-hash'));
  assert.ok(ids('var token = new Random().Next().ToString(); // bad').includes('safe/weak-random'));
  assert.ok(ids('Process.Start($"git log {branch}"); // bad').includes('safe/shell-injection'));
  assert.ok(ids('ServerCertificateValidationCallback = (a, b, c, d) => true; // bad')
    .includes('safe/tls-disabled'));
  assert.ok(ids('Console.WriteLine("here"); // bad').includes('alone/debug-leftover'));
});

test('keeps seeing sinks built inside string literals', () => {
  assert.ok(ids('cmd.CommandText = "SELECT * FROM t WHERE id = " + id;').includes('safe/sql-injection'));
  assert.ok(ids('Process.Start("cmd.exe", "/c " + cmd);').includes('safe/shell-injection'));
  // A URL inside a string is not the start of a comment.
  assert.ok(ids('var u = "http://h/x"; Console.WriteLine(u);').includes('alone/debug-leftover'));
});

test('the signature regex does not treat if/catch as methods', () => {
  const src = [
    'class X {',
    '    void Run() {',
    '        if (Ready()) {',
    '            DoThing();',
    '        }',
    '        try {',
    '            DoThing();',
    '        } catch (Exception e) {',
    '            _logger.LogError(e, "failed");',
    '            throw;',
    '        }',
    '    }',
    '}',
  ].join('\n');
  assert.deepStrictEqual(check(src, { relPath: 'X.cs', config }), []);
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'dotnet');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.cs'), 'utf8'),
    { relPath: 'clean.cs', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.cs'), 'utf8'),
    { relPath: 'dirty.cs', config }).length >= 4);
});

// Dictionary<string, int> is idiomatic C#; the space after the comma must not
// hide the method from shape measurement.
test('measures methods whose generic return type contains a space', () => {
  const method = (returnType) => [
    `public ${returnType} Run(string a, string b, string c, string d, string e) {`,
    '    return null;',
    '}',
  ].join('\n');
  assert.ok(ids(method('Dictionary<string, int>')).includes('obvious/too-many-params'));
  assert.ok(ids(method('Dictionary<string,int>')).includes('obvious/too-many-params'));
});

test('flags the ProcessStartInfo pair in either order', () => {
  // The two halves are a conjunction on one line; anchoring the rule at `^`
  // to make it linear must not make it order-sensitive.
  assert.ok(ids('var psi = new ProcessStartInfo { Arguments = $"/c {cmd}", UseShellExecute = true };').includes('safe/shell-injection'));
  assert.ok(ids('var psi = new ProcessStartInfo { UseShellExecute = true, Arguments = $"/c {cmd}" };').includes('safe/shell-injection'));
  assert.ok(!ids('var psi = new ProcessStartInfo { Arguments = $"/c {cmd}", UseShellExecute = false };').includes('safe/shell-injection'));
});

// Perf guard: every rule here must stay linear in line length. These lines are
// the adversarial shapes for the SAFE rules — a prefix that matches, repeated,
// so a rule with an unbounded span retries that span from every offset. The
// bound is deliberately loose: linear runs take ~10ms for 100KB, and the
// quadratic version of safe/shell-injection took 4,700ms on the same input, so
// 1s separates "linear" from "regression" with room for a loaded CI machine.
// The hook's whole budget is 2s.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check,
    relPath: 'X.cs',
    config,
    baseline: 'return;  ',
    units: [
      'Process.Start("a" ',
      'var psi = new ProcessStartInfo { UseShellExecute = true, Name = "a", ',
      'var token = ',
      'ServerCertificateValidationCallback = ',
      'var x = foo(a, b) + "s" + bar; ',
      'var q = "SELECT " + id; ',
      'var cmd = new SqlCommand(q, c); ',
      'Process.Start(arg); ',
    ],
    sources: ['x'.repeat(100 * 1024)],
  });
});

// Local taint: the assign-then-use form, at least as common as the inline one.
// Reported at the sink, naming the line the value was built on.
test('tracks an interpolated or concatenated string from its assignment to a sink', () => {
  const src = 'void F(SqlConnection c, string id) {\n  var q = "SELECT * FROM t WHERE id=" + id;\n  var cmd = new SqlCommand(q, c);\n}';
  const hit = check(src, { relPath: 'X.cs', config }).find((f) => f.id === 'safe/sql-injection');
  assert.ok(hit, 'no safe/sql-injection for the assign-then-use form');
  assert.strictEqual(hit.line, 3, 'reported at the sink, not the assignment');
  assert.match(hit.message, /line 2/);

  assert.ok(ids('void F(DbContext db, string id) {\n  var q = $"SELECT * FROM t WHERE id={id}";\n  db.Database.ExecuteSqlRaw(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void F(string dir) {\n  var cmd = "ls " + dir;\n  Process.Start(cmd);\n}')
    .includes('safe/shell-injection'));
});

test('a parameterized or literal-only variable reaching a sink stays silent', () => {
  assert.ok(!ids('void F(SqlConnection c, string id) {\n  var q = "SELECT * FROM t WHERE id = @id";\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void F(SqlConnection c) {\n  var q = "SELECT " + "a, b" + " FROM t";\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void F() {\n  var cmd = "notepad.exe";\n  Process.Start(cmd);\n}')
    .includes('safe/shell-injection'));
});

test('taint clears on a literal reassignment and does not leave its block', () => {
  assert.ok(!ids('void F(SqlConnection c, string id) {\n  var q = "SELECT t WHERE id=" + id;\n  q = "SELECT * FROM t";\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void A(string id) {\n  var q = "SELECT " + id;\n}\nvoid B(SqlConnection c, string q) {\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
});
