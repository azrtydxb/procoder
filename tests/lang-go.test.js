// tests/lang-go.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/go');
const { DEFAULTS } = require('../hooks/checks/config');
const { assertLinear } = require('./perf-guard');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.go', config }).map((f) => f.id);

test('owns the .go extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.go']);
});

test('flags discarded errors', () => {
  assert.ok(ids('result, _ := doWork()').includes('true/ignored-error'));
  assert.ok(!ids('result, err := doWork()\nif err != nil {\n\treturn err\n}').includes('true/ignored-error'));
});

test('does not flag idiomatic range loops that discard the value', () => {
  assert.ok(!ids('for i, _ := range xs {\n\tuse(i)\n}').includes('true/ignored-error'));
  assert.ok(!ids('for k, _ := range m {\n\tuse(k)\n}').includes('true/ignored-error'));
});

test('flags SQL interpolation', () => {
  assert.ok(ids('db.Query(fmt.Sprintf("SELECT * FROM t WHERE id = %s", id))').includes('safe/sql-injection'));
  assert.ok(!ids('db.Query("SELECT * FROM t WHERE id = $1", id)').includes('safe/sql-injection'));
});

test('flags command injection and disabled TLS', () => {
  assert.ok(ids('exec.Command("sh", "-c", userInput)').includes('safe/shell-injection'));
  assert.ok(ids('InsecureSkipVerify: true').includes('safe/tls-disabled'));
  assert.ok(!ids('exec.Command("ls", dir)').includes('safe/shell-injection'));
});

test('flags weak hashing and math/rand for secrets', () => {
  assert.ok(ids('h := md5.New()').includes('safe/weak-hash'));
  assert.ok(ids('token := rand.Int63()').includes('safe/weak-random'));
  assert.ok(!ids('h := sha256.New()').includes('safe/weak-hash'));
});

test('flags panic in library code and missing Close', () => {
  assert.ok(ids('panic("unreachable")').includes('true/panic-in-library'));
  assert.ok(ids('resp, err := http.Get(url)').includes('true/unclosed-resource'));
});

test('does not flag a resource followed by a nearby defer Close', () => {
  const withDefer = 'resp, err := http.Get(url)\nif err != nil {\n\treturn err\n}\ndefer resp.Body.Close()\n';
  assert.ok(!ids(withDefer).includes('true/unclosed-resource'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('fmt.Println("here")').includes('alone/debug-leftover'));
  assert.ok(!ids('log.Info("started")').includes('alone/debug-leftover'));
});

// Both directions of the shared principle: a rule sees code, not prose, and
// the string literals a sink is assembled from are code.
test('ignores rules named in comments, not the code beside them', () => {
  assert.ok(!ids('// never db.Query(fmt.Sprintf("SELECT %s", col))').includes('safe/sql-injection'));
  assert.ok(!ids('// never exec.Command("sh", "-c", userInput)').includes('safe/shell-injection'));
  assert.ok(!ids('// InsecureSkipVerify: true is never acceptable').includes('safe/tls-disabled'));
  assert.ok(!ids('// h := md5.New() is not a password hash').includes('safe/weak-hash'));
  assert.ok(!ids('// result, _ := doWork() discards the error').includes('true/ignored-error'));
  assert.ok(!ids('// fmt.Println("here") was removed').includes('alone/debug-leftover'));
  assert.ok(!ids('/* token := rand.Int63() is not a secret */').includes('safe/weak-random'));

  assert.ok(ids('db.Query(fmt.Sprintf("SELECT %s", col)) // never do this')
    .includes('safe/sql-injection'));
  assert.ok(ids('exec.Command("sh", "-c", userInput) // bad').includes('safe/shell-injection'));
  assert.ok(ids('cfg := &tls.Config{InsecureSkipVerify: true} // bad').includes('safe/tls-disabled'));
  assert.ok(ids('h := md5.New() // bad').includes('safe/weak-hash'));
  assert.ok(ids('result, _ := doWork() // bad').includes('true/ignored-error'));
  assert.ok(ids('fmt.Println("here") // bad').includes('alone/debug-leftover'));
});

// A comment is not a Close: prose promising cleanup must not discharge the
// rule, or the discharge becomes the easiest way to silence it.
test('a defer Close in a comment does not discharge the resource rule', () => {
  const commented = 'resp, err := http.Get(url)\n// defer resp.Body.Close()\n';
  assert.ok(ids(commented).includes('true/unclosed-resource'));
});

test('keeps seeing sinks built inside string literals', () => {
  assert.ok(ids('db.Query("SELECT * FROM t WHERE id = " + id)').includes('safe/sql-injection'));  // procoder: literal safe/sql-injection the Go snippet handed to the pack as input
  assert.ok(ids('exec.Command("bash", "-c", "rm "+dir)').includes('safe/shell-injection'));
});

// The structural shapes taint.js closed, each with the safe twin that must
// stay silent. Every one of these reported nothing before the statement model,
// the block-aware merge, the path bindings and the return pass went in.
const SHAPES = [
  ['a field',
    's.q = "SELECT id=" + id\ndb.Query(s.q)',
    's.q = "SELECT * FROM t"\ndb.Query(s.q)'],
  ['a helper\'s return value',
    'func build(id string) string { return "SELECT id=" + id }\nq := build(x)\ndb.Query(q)',
    'func build() string { return "SELECT * FROM t" }\nq := build()\ndb.Query(q)'],
  ['a return straight into the sink',
    'func b(id string) string { return "SELECT id=" + id }\ndb.Query(b(x))',
    'func b() string { return "SELECT * FROM t" }\ndb.Query(b())'],
  ['a binding made inside a branch',
    'q := "SELECT"\nif x {\n\tq = "SELECT id=" + id\n}\ndb.Query(q)',
    'q := "SELECT 1"\nif x {\n\tq = "SELECT 2"\n}\ndb.Query(q)'],
  ['a value built in a loop',
    'q := "SELECT"\nfor _, p := range ps {\n\tq = q + p\n}\ndb.Query(q)',
    'q := "SELECT"\nfor _, p := range ps {\n\tlog(q + p)\n}\ndb.Query(q)'],
  ['a wrapped right-hand side',
    'q :=\n\t"SELECT id=" + id\ndb.Query(q)',
    'q :=\n\t"SELECT " + "* FROM t"\ndb.Query(q)'],
  ['a container',
    'parts := []string{"SELECT id=", id}\nq := strings.Join(parts, "")\ndb.Query(q)',
    'parts := []string{"SELECT ", "* FROM t"}\nq := strings.Join(parts, "")\ndb.Query(q)'],
  ['an inner binding of the same name',
    'q := "SELECT id=" + id\nfunc g() {\n\tq := "SELECT 1"\n\t_ = q\n}\ndb.Query(q)',
    'q := "SELECT 1"\nfunc g() {\n\tq := "SELECT id=" + id\n\t_ = q\n}\ndb.Query(q)'],
];

for (const [what, unsafe, safe] of SHAPES) {
  test(`taint follows ${what}, and its safe twin stays silent`, () => {
    assert.ok(ids(unsafe).includes('safe/sql-injection'), `unsafe form of ${what} went unreported`);  // procoder: literal safe/sql-injection the id the shape must report
    assert.ok(!ids(safe).includes('safe/sql-injection'), `safe form of ${what} was reported`);
  });
}

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'go');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.go'), 'utf8'),
    { relPath: 'clean.go', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.go'), 'utf8'),
    { relPath: 'dirty.go', config }).length >= 5);
});

// Perf guard — see tests/perf-guard.js for the bound and why it is relative.
// The unbroken word run is not decoration: the taint assignment pattern's
// first draft nested `\w*` inside an optional `[\w*.[\]]+`, and two quantifiers
// over the same characters cost 4,887ms on 100KB of it and 75,263ms at 400KB.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check,
    relPath: 'x.go',
    config,
    baseline: 'return  ',
    units: [
      'func f(a) x ',
      'Query("x" ',
      'func (a, ',
      'x := f(a, b) + "s" + c; ',
      'q := "SELECT " + id ',
      'db.Query(q) ',
    ],
    sources: ['x'.repeat(100 * 1024), 'x'.repeat(100 * 1024) + ' := 1'],
  });
});

// Local taint: the assign-then-use form, at least as common as the inline one.
// Reported at the sink, naming the line the value was built on.
test('tracks a Sprintf or concatenated string from its assignment to a sink', () => {
  const src = 'func f(db *sql.DB, id string) {\n\tq := "SELECT * FROM t WHERE id=" + id\n\tdb.Query(q)\n}';
  const hit = check(src, { relPath: 'x.go', config }).find((f) => f.id === 'safe/sql-injection');
  assert.ok(hit, 'no safe/sql-injection for the assign-then-use form');
  assert.strictEqual(hit.line, 3, 'reported at the sink, not the assignment');
  assert.match(hit.message, /line 2/);

  assert.ok(ids('func f(db *sql.DB, id string) {\n\tq := fmt.Sprintf("SELECT * FROM t WHERE id=%s", id)\n\tdb.Exec(q)\n}')
    .includes('safe/sql-injection'));
});

test('a placeholder or literal-only query variable stays silent', () => {
  assert.ok(!ids('func f(db *sql.DB, id string) {\n\tq := "SELECT * FROM t WHERE id = $1"\n\tdb.Query(q, id)\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('func f(db *sql.DB) {\n\tq := "SELECT " + "a, b" + " FROM t"\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
});

test('taint clears on a literal reassignment and does not leave its block', () => {
  assert.ok(!ids('func f(db *sql.DB, id string) {\n\tq := "SELECT * FROM t WHERE id=" + id\n\tq = "SELECT * FROM t"\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('func a(id string) {\n\tq := "SELECT " + id\n\t_ = q\n}\nfunc b(db *sql.DB, q string) {\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
});

// Defect: a sink whose arguments are wholly constant carries nothing
// untrusted, and a rung-1 rule that fires on obviously-safe code is what gets
// the whole pack turned off. An identifier, a nested call or a Sprintf is
// data, and those still report.
test('a data sink whose arguments are wholly constant stays silent', () => {
  assert.ok(!ids('exec.Command("sh", "-c", "ls /tmp")').includes('safe/shell-injection'));
  assert.ok(!ids('exec.Command("bash", "-c", "ls -la")').includes('safe/shell-injection'));
});

test('the same sink still reports anything that is not a constant', () => {
  assert.ok(ids('exec.Command("sh", "-c", userInput)').includes('safe/shell-injection'));
  assert.ok(ids('exec.Command("sh", "-c", "rm "+dir)').includes('safe/shell-injection'));
  assert.ok(ids('exec.Command("sh", "-c", fmt.Sprintf("rm %s", dir))').includes('safe/shell-injection'));
});

// Defect: `q = q + id` cleared the taint at the moment it should have been
// introduced — the right-hand side is two names with no literal, so the source
// patterns missed it and the clear-on-any-other-assignment rule fired.
test('an assignment whose right-hand side is a tainted name carries the taint', () => {
  assert.ok(ids('func f(db *sql.DB, id string) {\n\tq := "SELECT id="\n\tq = q + id\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('func f(db *sql.DB, id string) {\n\tb := fmt.Sprintf("x%s", id)\n\ta := b\n\tdb.Query(a)\n}')
    .includes('safe/sql-injection'));
});

test('an append carries the taint of the whole value it builds', () => {
  assert.ok(ids('func f(db *sql.DB, id string) {\n\tq := "SELECT id="\n\tq += id\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('func f(db *sql.DB, id string) {\n\tq := fmt.Sprintf("SELECT %s", id)\n\tq += " LIMIT 1"\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('func f(db *sql.DB) {\n\tq := "SELECT a"\n\tq += " FROM t"\n\tdb.Query(q)\n}')
    .includes('safe/sql-injection'));
});

// Defect: a parameter is a fresh binding. One that happens to share a name
// with a tainted variable in an enclosing scope must start clean.
test('a parameter shadowing a tainted name starts clean', () => {
  assert.ok(!ids('func f(db *sql.DB, id string) {\n\tq := "SELECT " + id\n\tg := func(q string) {\n\t\tdb.Query(q)\n\t}\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('func f(db *sql.DB, id string) {\n\tq := "SELECT " + id\n\tg := func(other string) {\n\t\tdb.Query(q)\n\t}\n}')
    .includes('safe/sql-injection'));
});

// ---------------------------------------------------------------------------
// Round 3. Every case has its unsafe twin beside it.

// Item 1. A constant table name spliced into an otherwise parameterized query
// is the recommended way to write a query with a dynamic table, and it is
// correct: the value is not user data. `const` was also missing from the
// binding recogniser, so `const table = "users"` bound the name `const`.
test("a constant fragment in a parameterized query stays silent", () => {
  assert.ok(!ids('const table = "users"\nrows, err := db.Query(fmt.Sprintf("SELECT * FROM %s WHERE id = $1", table), id)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(!ids('const TABLE = "users"\nrows, err := db.Query("SELECT * FROM " + TABLE)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

test("the same shapes report when the fragment is not provably constant", () => {
  assert.ok(ids('table := r.URL.Query().Get("t")\nrows, err := db.Query(fmt.Sprintf("SELECT * FROM %s WHERE id = $1", table), id)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('rows, err := db.Query(fmt.Sprintf("SELECT * FROM t WHERE id = %s", userID))\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

// Item 2. `Query` is an ordinary method name, and a SQL finding in a file
// with no database in it is always wrong.
test("a generic Query in a file with no database in it stays silent", () => {
  assert.ok(!ids('func Ask(a Agent, t string) string {\n\tphrase := fmt.Sprintf("find %s", t)\n\treturn a.Query(phrase)\n}\n')
    .includes("safe/sql-injection"));
});

test("the same call reports as soon as the file does talk to a database", () => {
  assert.ok(ids('func Ask(db *sql.DB, t string) string {\n\tphrase := fmt.Sprintf("find %s", t)\n\treturn db.Query(phrase)\n}\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

// Item 7. A single class excluding every quote never saw a literal that holds
// the other quote character — which is exactly how SQL gets written.
test("concatenation where the literal holds the other quote is still seen", () => {
  assert.ok(ids('rows, err := db.Query("SELECT * FROM u WHERE n = \x27" + name + "\x27")\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

// Round 4. The file-level database gate was too coarse — see lang-ts.test.js.
// `QueryRowContext` is a database call form; a bare `Exec` is not.
test("a database-only call form is evidence without any SQL vocabulary", () => {
  assert.ok(ids('func find(h Handle, name string) error {\n\ttext := base + "@" + name\n\treturn h.QueryRowContext(ctx, text)\n}\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('import "github.com/jackc/pgx/v5"\n\nfunc find(h Handle, name string) error {\n\ttext := base + "@" + name\n\treturn h.Exec(text)\n}\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

test("a generic Exec with no evidence anywhere stays silent", () => {
  assert.ok(!ids('func run(s Step, n string) error {\n\tlabel := base + "@" + n\n\treturn s.Exec(label)\n}\n')
    .includes("safe/sql-injection"));
});
