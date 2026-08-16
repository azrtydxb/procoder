// tests/lang-go.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/go');
const { DEFAULTS } = require('../hooks/checks/config');

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

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'go');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.go'), 'utf8'),
    { relPath: 'clean.go', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.go'), 'utf8'),
    { relPath: 'dirty.go', config }).length >= 5);
});

// Perf guard: every rule here must stay linear in line length. Each unit below
// is an adversarial prefix — repeated, it makes any unbounded span in a rule
// re-scan to end of line from every offset, which is the quadratic shape that
// took .NET's safe/shell-injection rule 4.7s on this input. Linear runs finish
// in ~10ms, so 1s separates "linear" from "regression" with plenty of slack for
// a loaded CI machine; the hook's whole budget is 2s.
test('stays linear on a very long single line', () => {
  const units = [
    "func f(a) x ",
    "Query(\"x\" ",
    "func (a, ",
    "x := f(a, b) + \"s\" + c; ",
  ];
  for (const unit of units) {
    const line = unit.repeat(Math.ceil((100 * 1024) / unit.length)).slice(0, 100 * 1024);
    const started = Date.now();
    check(line, { relPath: 'x.go', config });
    const elapsed = Date.now() - started;
    assert.ok(elapsed < 1000, `100KB line took ${elapsed}ms for unit ${JSON.stringify(unit)}`);
  }
});
