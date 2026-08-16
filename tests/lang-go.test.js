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

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'go');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.go'), 'utf8'),
    { relPath: 'clean.go', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.go'), 'utf8'),
    { relPath: 'dirty.go', config }).length >= 5);
});
