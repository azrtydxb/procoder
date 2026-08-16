// tests/lang-py.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/py');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.py', config }).map((f) => f.id);

test('owns the .py extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.py']);
});

test('flags SQL built with f-strings, % or concatenation', () => {
  assert.ok(ids('cursor.execute(f"SELECT * FROM t WHERE id = {uid}")').includes('safe/sql-injection'));
  assert.ok(ids('cursor.execute("SELECT * FROM t WHERE id = %s" % uid)').includes('safe/sql-injection'));
  assert.ok(!ids('cursor.execute("SELECT * FROM t WHERE id = %s", (uid,))').includes('safe/sql-injection'));
});

test('flags shell and dynamic execution risks', () => {
  assert.ok(ids('subprocess.run(cmd, shell=True)').includes('safe/shell-injection'));
  assert.ok(ids('os.system("rm " + target)').includes('safe/shell-injection'));
  assert.ok(ids('eval(user_input)').includes('safe/dynamic-eval'));
  assert.ok(!ids('subprocess.run(["ls", target])').includes('safe/shell-injection'));
});

test('flags unsafe deserialization and weak hashing', () => {
  assert.ok(ids('data = pickle.loads(payload)').includes('safe/unsafe-deserialize'));
  assert.ok(ids('yaml.load(text)').includes('safe/unsafe-deserialize'));
  assert.ok(ids('hashlib.md5(password.encode())').includes('safe/weak-hash'));
  assert.ok(!ids('yaml.safe_load(text)').includes('safe/unsafe-deserialize'));
});

test('flags bare and silent exception handling', () => {
  assert.ok(ids('try:\n    go()\nexcept:\n    pass\n').includes('true/bare-except'));
  assert.ok(ids('try:\n    go()\nexcept Exception:\n    pass\n').includes('true/swallowed-error'));
  assert.ok(!ids('try:\n    go()\nexcept ValueError as e:\n    logger.exception(e)\n    raise\n').includes('true/bare-except'));
});

test('flags mutable default arguments', () => {
  assert.ok(ids('def add(item, into=[]):').includes('true/mutable-default'));
  assert.ok(ids('def add(item, opts={}):').includes('true/mutable-default'));
  assert.ok(!ids('def add(item, into=None):').includes('true/mutable-default'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('print("here")').includes('alone/debug-leftover'));
  assert.ok(ids('breakpoint()').includes('alone/debug-leftover'));
  assert.ok(!ids('logger.info("started")').includes('alone/debug-leftover'));
});

// Both directions of the shared principle: a rule sees code, not prose, and
// the string literals a sink is assembled from are code.
test('ignores rules named in comments and docstrings, not the code beside them', () => {
  assert.ok(!ids('# never use eval(user_input) here').includes('safe/dynamic-eval'));
  assert.ok(!ids('run(cmd)  # do not pass shell=True').includes('safe/shell-injection'));
  assert.ok(!ids('# cursor.execute(f"SELECT {uid}") is how not to do it')
    .includes('safe/sql-injection'));
  assert.ok(!ids('def f():\n    """Never call pickle.loads(payload) on user bytes."""\n')
    .includes('safe/unsafe-deserialize'));
  assert.ok(!ids('# print("here") was removed').includes('alone/debug-leftover'));
  assert.ok(!ids('# except: is never acceptable').includes('true/bare-except'));

  assert.ok(ids('eval(user_input)  # never use eval here').includes('safe/dynamic-eval'));
  assert.ok(ids('run(cmd, shell=True)  # do not pass shell=True').includes('safe/shell-injection'));
  assert.ok(ids('cursor.execute(f"SELECT {uid}")  # bad').includes('safe/sql-injection'));
  assert.ok(ids('pickle.loads(payload)  # bad').includes('safe/unsafe-deserialize'));
  assert.ok(ids('print("here")  # bad').includes('alone/debug-leftover'));
  assert.ok(ids('try:\n    go()\nexcept:  # bad\n    pass\n').includes('true/bare-except'));
});

test('keeps seeing sinks built inside string literals', () => {
  assert.ok(ids('cursor.execute("SELECT * FROM t WHERE id = %s" % uid)')
    .includes('safe/sql-injection'));
  assert.ok(ids('os.system("rm " + target)').includes('safe/shell-injection'));
});

test('flags shape violations using indentation depth', () => {
  const deep = 'def a():\n    if x:\n        for y in z:\n            while w:\n                go()\n';
  assert.ok(ids(deep).includes('obvious/nesting-depth'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'py');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.py'), 'utf8'),
    { relPath: 'clean.py', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.py'), 'utf8'),
    { relPath: 'dirty.py', config }).length >= 6);
});

// Perf guard: every rule here must stay linear in line length. Each unit below
// is an adversarial prefix — repeated, it makes any unbounded span in a rule
// re-scan to end of line from every offset, which is the quadratic shape that
// took .NET's safe/shell-injection rule 4.7s on this input. Linear runs finish
// in ~10ms, so 1s separates "linear" from "regression" with plenty of slack for
// a loaded CI machine; the hook's whole budget is 2s.
test('stays linear on a very long single line', () => {
  const units = [
    "yaml.load(",
    "execute(\"x\" ",
    "def f(a=1, ",
    "x = f(a, b) + \"s\" + c; ",
  ];
  for (const unit of units) {
    const line = unit.repeat(Math.ceil((100 * 1024) / unit.length)).slice(0, 100 * 1024);
    const started = Date.now();
    check(line, { relPath: 'x.py', config });
    const elapsed = Date.now() - started;
    assert.ok(elapsed < 1000, `100KB line took ${elapsed}ms for unit ${JSON.stringify(unit)}`);
  }
});
