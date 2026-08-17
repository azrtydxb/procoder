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

// The structural shapes taint.js closed, each with the safe twin that must
// stay silent. Every one of these reported nothing before the statement model,
// the block-aware merge, the path bindings and the return pass went in.
const SHAPES = [
  ['a field',
    'this.q = "SELECT id=" + id;\ncmd.CommandText = this.q;',
    'this.q = "SELECT * FROM t";\ncmd.CommandText = this.q;'],
  ['a helper\'s return value',
    'string Build(string id) { return "SELECT id=" + id; }\nvar q = Build(x);\ncmd.CommandText = q;',
    'string Build() { return "SELECT * FROM t"; }\nvar q = Build();\ncmd.CommandText = q;'],
  ['a return straight into the sink',
    'string B(string id) { return "SELECT id=" + id; }\ncmd.CommandText = B(x);',
    'string B() { return "SELECT * FROM t"; }\ncmd.CommandText = B();'],
  ['a binding made inside a branch',
    'var q = "SELECT";\nif (x) { q = "SELECT id=" + id; }\ncmd.CommandText = q;',
    'var q = "SELECT 1";\nif (x) { q = "SELECT 2"; }\ncmd.CommandText = q;'],
  ['a value built in a loop',
    'var q = "SELECT";\nforeach (var p in ps) { q = q + p; }\ncmd.CommandText = q;',
    'var q = "SELECT";\nforeach (var p in ps) { Log(q + p); }\ncmd.CommandText = q;'],
  ['a wrapped right-hand side',
    'var q =\n    "SELECT id=" + id;\ncmd.CommandText = q;',
    'var q =\n    "SELECT " + "* FROM t";\ncmd.CommandText = q;'],
  ['a transformation at the sink',
    'var q = "SELECT id=" + id;\ncmd.CommandText = q.Trim();',
    'var q = "SELECT * FROM t";\ncmd.CommandText = q.Trim();'],
  ['a container',
    'var parts = new[] { "SELECT id=", id };\nvar q = string.Join("", parts);\ncmd.CommandText = q;',
    'var parts = new[] { "SELECT ", "* FROM t" };\nvar q = string.Join("", parts);\ncmd.CommandText = q;'],
  ['an inner binding of the same name',
    'var q = "SELECT id=" + id;\nvoid G() { var q = "SELECT 1"; }\ncmd.CommandText = q;',
    'var q = "SELECT 1";\nvoid G() { var q = "SELECT id=" + id; }\ncmd.CommandText = q;'],
];

for (const [what, unsafe, safe] of SHAPES) {
  test(`taint follows ${what}, and its safe twin stays silent`, () => {
    assert.ok(ids(unsafe).includes('safe/sql-injection'), `unsafe form of ${what} went unreported`);  // procoder: literal safe/sql-injection the id the shape must report
    assert.ok(!ids(safe).includes('safe/sql-injection'), `safe form of ${what} was reported`);
  });
}

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
    sources: [
      'x'.repeat(100 * 1024),
      // Nested calls that DO close — see lang-ts.test.js.
      'Process.Start('.repeat(7000) + ')'.repeat(7000),
      'ServerCertificateValidationCallback = a '.repeat(2500),
    ],
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

// The 500-character span ceilings the SAFE rules used to carry: a sink whose
// interpolation sits further than that from the call was missed entirely.
const PAD = 'a'.repeat(600);

test('sees a sink whose interpolation is more than 500 characters from the call', () => {
  assert.ok(ids(`Process.Start("${PAD}", $"git log {branch}");`).includes('safe/shell-injection'));
  assert.ok(ids(`var token = Helper("${PAD}") + new Random().Next();`).includes('safe/weak-random'));
  assert.ok(ids(`ServerCertificateValidationCallback = Wrap("${PAD}", (a, b, c, d) => true);`).includes('safe/tls-disabled'));
});

// Defect: a sink whose arguments are wholly constant carries nothing
// untrusted, and a rung-1 rule that fires on obviously-safe code is what gets
// the whole pack turned off. An identifier, a nested call or an interpolation
// is data, and those still report.
test('a data sink whose arguments are wholly constant stays silent', () => {
  assert.ok(!ids('Process.Start("cmd.exe", "/c " + "dir");').includes('safe/shell-injection'));
  assert.ok(!ids('var psi = new ProcessStartInfo { FileName = "cmd.exe", Arguments = $"/c dir", UseShellExecute = true };').includes('safe/shell-injection'));
});

test('the same sinks still report anything that is not a constant', () => {
  assert.ok(ids('Process.Start($"git log {branch}");').includes('safe/shell-injection'));
  assert.ok(ids('Process.Start("cmd.exe", "/c " + cmd);').includes('safe/shell-injection'));
  assert.ok(ids('Process.Start("cmd.exe", "/c " + Build(dir));').includes('safe/shell-injection'));
});

// Defect: `q = q + id` cleared the taint at the moment it should have been
// introduced — the right-hand side is two names with no literal, so the source
// patterns missed it and the clear-on-any-other-assignment rule fired.
test('an assignment whose right-hand side is a tainted name carries the taint', () => {
  assert.ok(ids('void F(SqlConnection c, string id) {\n  var q = "SELECT id=";\n  q = q + id;\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void F(SqlConnection c, string id) {\n  var b = $"x{id}";\n  var a = b;\n  var cmd = new SqlCommand(a, c);\n}')
    .includes('safe/sql-injection'));
});

test('an append carries the taint of the whole value it builds', () => {
  assert.ok(ids('void F(SqlConnection c, string id) {\n  var q = "SELECT id=";\n  q += id;\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void F(SqlConnection c, string id) {\n  var q = $"SELECT {id}";\n  q += " LIMIT 1";\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void F(SqlConnection c) {\n  var q = "SELECT a";\n  q += " FROM t";\n  var cmd = new SqlCommand(q, c);\n}')
    .includes('safe/sql-injection'));
});

// Defect: a parameter is a fresh binding. One that happens to share a name
// with a tainted variable in an enclosing scope must start clean.
test('a parameter shadowing a tainted name starts clean', () => {
  assert.ok(!ids('void F(SqlConnection c, string id) {\n  var q = "SELECT " + id;\n  void G(string q) {\n    var cmd = new SqlCommand(q, c);\n  }\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void F(SqlConnection c, string id) {\n  var q = "SELECT " + id;\n  void G(string other) {\n    var cmd = new SqlCommand(q, c);\n  }\n}')
    .includes('safe/sql-injection'));
});

test('the safe forms stay silent however long the arguments are', () => {
  assert.ok(!ids(`Process.Start("git", "${PAD}");`).includes('safe/shell-injection'));
  assert.ok(!ids(`var token = Helper("${PAD}") + RandomNumberGenerator.GetInt32(9);`).includes('safe/weak-random'));
  assert.ok(!ids(`ServerCertificateValidationCallback = Wrap("${PAD}", (a, b, c, d) => Validate(a));`).includes('safe/tls-disabled'));
});

// ---------------------------------------------------------------------------
// Round 3. Item 4: `CommandText = $"… @tenant"` is an interpolated string with
// *zero holes* — a fully parameterized query — and the rule carried no
// `dataSink` mark, so the constant discharge never ran on it.
test("an interpolated string with no holes stays silent", () => {
  assert.ok(!ids('cmd.CommandText = $"SELECT * FROM t WHERE tenant = @tenant";\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

// Item 1: a constant column spliced into an otherwise parameterized query.
test("a constant fragment in a parameterized query stays silent", () => {
  assert.ok(!ids('const string col = "created_at";\ncmd.CommandText = $"SELECT * FROM t WHERE id = @id ORDER BY {col}";\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

test("the same shapes report when the fragment is not provably constant", () => {
  assert.ok(ids('cmd.CommandText = $"SELECT * FROM t WHERE id = {userId}";\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('string col = "created_at";\ncol = Request.Query["c"];\ncmd.CommandText = $"SELECT * FROM t ORDER BY {col}";\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});
