// tests/lang-py.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/py');
const { DEFAULTS } = require('../hooks/checks/config');
const { assertLinear } = require('./perf-guard');

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
  assert.ok(ids('eval(user_input)').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
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

// The rule used to be a line pattern whose parameter span was bounded to 500
// characters, so a `def` with more parameters than that ahead of the mutable
// one produced nothing — and a long signature is exactly where a stray `[]`
// hides. Being a *line* pattern, it also never saw a wrapped `def`, which is
// what black produces for a signature that long.
test('a mutable default is found however long the parameter list is', () => {
  // Past 500 characters of parameters, which is where the old span gave up.
  const wide = Array.from({ length: 60 }, (unused, i) => `parameter_number_${i}`).join(', ');
  assert.ok(ids(`def add(${wide}, into=[]):\n    return into\n`)
    .includes('true/mutable-default'), 'lost past the old 500-character span');
  assert.ok(!ids(`def add(${wide}, into=None):\n    return into\n`)
    .includes('true/mutable-default'), 'a sound default must stay sound');

  const wrapped = ['def add(', '    item,', '    into=[],', '):', '    return into'];
  assert.ok(ids(wrapped.join('\n')).includes('true/mutable-default'),
    'a wrapped def was never read past its first line');
});

test('flags leftover debugging', () => {
  assert.ok(ids('print("here")').includes('alone/debug-leftover'));
  assert.ok(ids('breakpoint()').includes('alone/debug-leftover'));
  assert.ok(!ids('logger.info("started")').includes('alone/debug-leftover'));
});

// Both directions of the shared principle: a rule sees code, not prose, and
// the string literals a sink is assembled from are code.
test('ignores rules named in comments and docstrings, not the code beside them', () => {
  // procoder: literal safe/dynamic-eval the eval( is the assertion input, not a call
  assert.ok(!ids('# never use eval(user_input) here').includes('safe/dynamic-eval'));
  assert.ok(!ids('run(cmd)  # do not pass shell=True').includes('safe/shell-injection'));
  assert.ok(!ids('# cursor.execute(f"SELECT {uid}") is how not to do it')
    .includes('safe/sql-injection'));
  assert.ok(!ids('def f():\n    """Never call pickle.loads(payload) on user bytes."""\n')
    .includes('safe/unsafe-deserialize'));
  assert.ok(!ids('# print("here") was removed').includes('alone/debug-leftover'));
  assert.ok(!ids('# except: is never acceptable').includes('true/bare-except'));

  // procoder: literal safe/dynamic-eval the eval( is the assertion input, not a call
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

// The structural shapes taint.js closed, each with the safe twin that must
// stay silent. Every one of these reported nothing before the statement model,
// the block-aware merge, the path bindings and the return pass went in.
const SHAPES = [
  ['a field',
    'self.q = "SELECT id=" + user_id\ncur.execute(self.q)',
    'self.q = "SELECT * FROM t"\ncur.execute(self.q)'],
  ['a helper\'s return value',
    'def build(uid):\n    return "SELECT id=" + uid\nq = build(x)\ncur.execute(q)',
    'def build():\n    return "SELECT * FROM t"\nq = build()\ncur.execute(q)'],
  ['a return straight into the sink',
    'def b(uid):\n    return "SELECT id=" + uid\ncur.execute(b(1))',
    'def b():\n    return "SELECT * FROM t"\ncur.execute(b())'],
  ['a binding made inside a branch',
    'def f(cur, uid, x):\n    q = "SELECT"\n    if x:\n        q = "SELECT id=" + uid\n    cur.execute(q)',
    'def f(cur, uid, x):\n    q = "SELECT 1"\n    if x:\n        q = "SELECT 2"\n    cur.execute(q)'],
  ['a value built in a loop',
    'def f(cur, ps):\n    q = "SELECT"\n    for p in ps:\n        q = q + p\n    cur.execute(q)',
    'def f(cur, ps):\n    q = "SELECT"\n    for p in ps:\n        log(q + p)\n    cur.execute(q)'],
  ['a wrapped right-hand side',
    'q = (\n    "SELECT id=" + uid)\ncur.execute(q)',
    'q = (\n    "SELECT " + "* FROM t")\ncur.execute(q)'],
  ['a transformation at the sink',
    'q = "SELECT id=" + uid\ncur.execute(q.strip())',
    'q = "SELECT * FROM t"\ncur.execute(q.strip())'],
  ['a container',
    'parts = ["SELECT id=", uid]\nq = "".join(parts)\ncur.execute(q)',
    'parts = ["SELECT ", "* FROM t"]\nq = "".join(parts)\ncur.execute(q)'],
  ['an inner binding of the same name',
    'q = "SELECT id=" + uid\ndef g():\n    q = "SELECT 1"\ncur.execute(q)',
    'q = "SELECT 1"\ndef g():\n    q = "SELECT id=" + uid\ncur.execute(q)'],
];

for (const [what, unsafe, safe] of SHAPES) {
  test(`taint follows ${what}, and its safe twin stays silent`, () => {
    assert.ok(ids(unsafe).includes('safe/sql-injection'), `unsafe form of ${what} went unreported`);  // procoder: literal safe/sql-injection the id the shape must report
    assert.ok(!ids(safe).includes('safe/sql-injection'), `safe form of ${what} was reported`);
  });
}

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'py');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.py'), 'utf8'),
    { relPath: 'clean.py', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.py'), 'utf8'),
    { relPath: 'dirty.py', config }).length >= 6);
});

// --- wrapped `def` signatures ----------------------------------------------
//
// black wraps a signature one parameter per line as soon as it passes the line
// width, so in formatted Python a many-parameter function is *always* wrapped —
// exactly the shape obvious/too-many-params exists to catch. Reading the
// parameters off the `def` line alone reported 0 for every one of them.
const paramFinding = (src) => check(src, { relPath: 'x.py', config })
  .find((f) => f.id === 'obvious/too-many-params');

test('a wrapped def is measured, and reported at its def line', () => {
  const src = [
    'def process_user_records(',   // 1
    '    records,',
    '    options,',
    '    logger,',
    '    clock,',
    '    store,',
    '    ctx,',                    // trailing comma is not a parameter
    '):',
    '    return len(records)',
    '',
  ].join('\n');
  const found = paramFinding(src);
  assert.ok(found, 'a wrapped def was not measured at all');
  assert.strictEqual(found.message, '6 parameters (limit 4)');
  assert.strictEqual(found.line, 1);
});

test('async def, decorators above, and commas inside defaults and annotations', () => {
  const wrapped = (params) => [
    '@retry(times=3)',
    '@traced',
    'async def go(',
    ...params.map((p) => `    ${p},`),
    ') -> Dict[str, int]:',
    '    return {}',
    '',
  ].join('\n');

  // Five parameters, three of them carrying a comma of their own.
  assert.strictEqual(
    paramFinding(wrapped(['a', 'b=(1, 2)', 'seen: Dict[str, int]', '*args', '**kwargs'])).message,
    '5 parameters (limit 4)');
  // Four of the same: under the limit, and the inner commas must not inflate it.
  assert.strictEqual(paramFinding(wrapped(['b=(1, 2)', 'seen: Dict[str, int]', '*args', '**kwargs'])),
    undefined);
});

// `self`/`cls` is the receiver, not an argument the caller passes. A method
// written with five callable parameters is the same burden as a function with
// five, and counting the receiver would make the budget one tighter for methods
// than for functions — and tighter than for the brace packs, where the receiver
// is implicit `this` and was never counted.
test('self and cls do not count toward the parameter budget', () => {
  const method = (receiver, params) => [
    'class Store:',
    `    def save(${receiver}${params.map((p) => `, ${p}`).join('')}):`,
    '        return 1',
    '',
  ].join('\n');
  assert.strictEqual(paramFinding(method('self', ['a', 'b', 'c', 'd'])), undefined);
  assert.strictEqual(paramFinding(method('cls', ['a', 'b', 'c', 'd'])), undefined);
  assert.strictEqual(paramFinding(method('self', ['a', 'b', 'c', 'd', 'e'])).message,
    '5 parameters (limit 4)');
  // A plain function keeps counting all five.
  assert.strictEqual(paramFinding('def save(a, b, c, d, e):\n    return 1\n').message,
    '5 parameters (limit 4)');
});

// Perf guard: every rule here must stay linear in line length. Each unit below
// is an adversarial prefix — repeated, it makes any unbounded span in a rule
// re-scan to end of line from every offset, which is the quadratic shape that
// took .NET's safe/shell-injection rule 4.7s on this input, and this pack's
// true/mutable-default 653ms.
//
// The bound, the baseline it is measured against and the rationale for both
// now live in tests/perf-guard.js, where the other five packs share them:
// this pack had the relative bound and the other five still had absolute
// ones, which is five copies of one property at two different answers.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check,
    relPath: 'x.py',
    config,
    comment: 'py',
    baseline: 'pass  ',
    units: [
      'yaml.load(',
      'execute("x" ',
      'def f(a=1, ',
      'x = f(a, b) + "s" + c; ',
      'q = f"SELECT {a}" ',
      'cur.execute(q) ',
    ],
    sources: ['x'.repeat(100 * 1024)],
  });
});

// Local taint: the assign-then-use form, at least as common as the inline one.
// Reported at the sink, naming the line the value was built on.
test('tracks an f-string, % or format value from its assignment to a sink', () => {
  const src = 'def f(cur, uid):\n    q = f"SELECT * FROM t WHERE id={uid}"\n    cur.execute(q)\n';
  const hit = check(src, { relPath: 'x.py', config }).find((f) => f.id === 'safe/sql-injection');
  assert.ok(hit, 'no safe/sql-injection for the f-string form');
  assert.strictEqual(hit.line, 3, 'reported at the sink, not the assignment');
  assert.match(hit.message, /line 2/);

  assert.ok(ids('def f(cur, uid):\n    q = "SELECT * FROM t WHERE id=%s" % uid\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def f(cur, uid):\n    q = "SELECT * FROM t WHERE id=" + uid\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def f(cur, uid):\n    q = "SELECT * FROM t WHERE id={}".format(uid)\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
});

test('a parameterized or literal-only variable reaching a sink stays silent', () => {
  assert.ok(!ids('def f(cur, uid):\n    q = "SELECT * FROM t WHERE id = %s"\n    cur.execute(q, (uid,))\n')
    .includes('safe/sql-injection'));
  assert.ok(!ids('def f(cur):\n    q = "SELECT " + "a, b" + " FROM t"\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
});

test('taint clears on a literal reassignment and does not leave its block', () => {
  assert.ok(!ids('def f(cur, uid):\n    q = "SELECT * FROM t WHERE id=" + uid\n    q = "SELECT * FROM t"\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(!ids('def a(uid):\n    q = "SELECT " + uid\n\n\ndef b(cur, q):\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
});

// Defect: `os.system("ls /tmp")` reported safe/shell-injection. Nothing
// untrusted is in it — the command is a literal — and a rung-1 rule that fires
// on obviously-safe code is what gets the whole pack turned off. A call whose
// arguments are wholly constant carries no data, so the finding is discharged;
// an identifier, a nested call, an f-string or a concatenation is data and the
// finding stands.
test('a data sink whose arguments are wholly constant stays silent', () => {
  assert.ok(!ids('os.system("ls /tmp")').includes('safe/shell-injection'));
  assert.ok(!ids('os.popen("ls -la")').includes('safe/shell-injection'));
  assert.ok(!ids('subprocess.run("ls -la", shell=True)').includes('safe/shell-injection'));
  assert.ok(!ids('eval("1 + 1")').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.ok(!ids('pickle.loads(b"payload")').includes('safe/unsafe-deserialize'));
});

test('the same sinks still report anything that is not a constant', () => {
  assert.ok(ids('os.system("rm " + target)').includes('safe/shell-injection'));
  assert.ok(ids('os.system(cmd)').includes('safe/shell-injection'));
  assert.ok(ids('os.system(f"rm {target}")').includes('safe/shell-injection'));
  assert.ok(ids('os.system(build_command())').includes('safe/shell-injection'));
  assert.ok(ids('subprocess.run(cmd, shell=True)').includes('safe/shell-injection'));
  assert.ok(ids('eval(user_input)').includes('safe/dynamic-eval'));  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.ok(ids('pickle.loads(payload)').includes('safe/unsafe-deserialize'));
});

// Defect: `q = q + uid` cleared the taint at the moment it should have been
// introduced — the right-hand side is two names with no literal, so the source
// patterns missed it and the clear-on-any-other-assignment rule fired.
test('an assignment whose right-hand side is a tainted name carries the taint', () => {
  assert.ok(ids('def f(cur, uid):\n    q = "SELECT id="\n    q = q + uid\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def f(cur, uid):\n    b = f"x{uid}"\n    a = b\n    cur.execute(a)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def f(cur, uid):\n    q = f"SELECT {uid}"\n    q = q + " LIMIT 1"\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
});

test('an append carries the taint of the whole value it builds', () => {
  assert.ok(ids('def f(cur, uid):\n    q = "SELECT id="\n    q += uid\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def f(cur, uid):\n    q = f"SELECT {uid}"\n    q += " LIMIT 1"\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(!ids('def f(cur):\n    q = "SELECT a"\n    q += " FROM t"\n    cur.execute(q)\n')
    .includes('safe/sql-injection'));
});

// Defect: a parameter is a fresh binding. One that happens to share a name
// with a tainted variable in an enclosing scope must start clean.
test('a parameter shadowing a tainted name starts clean', () => {
  assert.ok(!ids('def outer(uid):\n    q = f"SELECT {uid}"\n    def inner(q):\n        cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def outer(uid):\n    q = f"SELECT {uid}"\n    def inner(other):\n        cur.execute(q)\n')
    .includes('safe/sql-injection'));
  assert.ok(ids('def outer(uid):\n    q = "SELECT 1"\n    def inner(q):\n        q = q + uid\n        cur.execute(q)\n')
    .includes('safe/sql-injection'));
});

// ---------------------------------------------------------------------------
// Round 3. Every case has its unsafe twin beside it: a fix that silences a
// false positive by weakening the rule is not a fix.

// Item 1. A constant column list spliced into an otherwise parameterized query.
test("a constant fragment in a parameterized query stays silent", () => {
  assert.ok(!ids('COLS = "id, name"\ncur.execute(f"SELECT {COLS} FROM t WHERE id = %s", (id,))\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(!ids('TABLE = "users"\ncur.execute("SELECT * FROM " + TABLE)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

test("the same shapes report when the fragment is not user-independent", () => {
  assert.ok(ids('def h(cur, user_id):\n    cur.execute(f"SELECT * FROM t WHERE id = {user_id}")\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('COLS = request.args["c"]\ncur.execute(f"SELECT {COLS} FROM t WHERE id = %s", (id,))\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

// Item 2. `execute` is an ordinary method name — a Command pattern, a job
// runner — and a SQL finding in a file with no database in it is always wrong.
test("a generic execute in a file with no database in it stays silent", () => {
  assert.ok(!ids('def run(job, n):\n    label = f"job {n}"\n    return job.execute(label)\n')
    .includes("safe/sql-injection"));
});

test("the same call reports as soon as the file does talk to a database", () => {
  assert.ok(ids('def run(cur, n):\n    label = f"job {n}"\n    return cur.execute(label)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

// Item 6. `usedforsecurity=False` is the standard, explicit way to say this
// hash is not a security primitive. The rule is about the algorithm, and this
// flag is the one argument that answers it.
test("a hash explicitly declared non-security stays silent", () => {
  assert.ok(!ids('h = hashlib.md5(data, usedforsecurity=False).hexdigest()\n')  // procoder: literal safe/weak-hash scanner input for that rule, not an instance of it
    .includes("safe/weak-hash"));
  assert.ok(!ids('d = hashlib.sha1(blob, usedforsecurity = False)\n')  // procoder: literal safe/weak-hash scanner input for that rule, not an instance of it
    .includes("safe/weak-hash"));
});

test("weak hashing still reports without that flag", () => {
  assert.ok(ids('digest = hashlib.md5(password.encode()).hexdigest()\n')  // procoder: literal safe/weak-hash scanner input for that rule, not an instance of it
    .includes("safe/weak-hash"));
  assert.ok(ids('digest = hashlib.md5(pw, usedforsecurity=True).hexdigest()\n')  // procoder: literal safe/weak-hash scanner input for that rule, not an instance of it
    .includes("safe/weak-hash"));
});

// Item 7, a false negative. The f-string source excluded both quote characters
// from the literal body, so an interpolation inside single quotes inside a
// double-quoted f-string — the most common Python injection spelling there is —
// was never seen at all.
test("an f-string interpolating inside the other quote is a source", () => {
  assert.ok(ids('def q(cur, name):\n    sql = f"SELECT * FROM users WHERE name = \'{name}\'"\n    cur.execute(sql)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('def q(cur, name):\n    sql = "SELECT * FROM users WHERE name = \'%s\'" % name\n    cur.execute(sql)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(!ids('def q(cur, name):\n    sql = "SELECT * FROM users WHERE name = %s"\n    cur.execute(sql, (name,))\n')
    .includes("safe/sql-injection"));
});

// A type annotation between the name and the `=` defeated the binding
// recogniser outright, so an annotated binding established no taint at all.
test("an annotated binding still establishes taint", () => {
  assert.ok(ids('def h(cur, user_id):\n    q: str = "SELECT * FROM t WHERE id = " + user_id\n    cur.execute(q)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(!ids('def h(cur, user_id):\n    q: str = "SELECT * FROM t WHERE id = %s"\n    cur.execute(q, (user_id,))\n')
    .includes("safe/sql-injection"));
});

// Round 4. The file-level database gate was too coarse — see lang-ts.test.js.
// A driver import is the file-level tie-break for a call whose receiver shape
// and method form both say nothing.
test("an injection in a file with no SQL vocabulary still reports", () => {
  assert.ok(ids('import asyncpg\nfrom .statements import BASE\n\nasync def find(client, name):\n    text = BASE + "@" + name\n    return await client.execute(text)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(ids('import peewee\n\ndef find(client, name):\n    text = BASE + name\n    return client.execute(text)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
});

test("the executemany call form is database evidence on its own", () => {
  assert.ok(ids('def find(client, name):\n    text = BASE + name\n    return client.executemany(text)\n')  // procoder: literal safe/sql-injection scanner input for that rule, not an instance of it
    .includes("safe/sql-injection"));
  assert.ok(!ids('def run(job, n):\n    label = BASE + n\n    return job.execute(label)\n')
    .includes("safe/sql-injection"));
});
