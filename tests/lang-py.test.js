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
