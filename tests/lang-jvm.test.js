const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/jvm');
const { DEFAULTS } = require('../hooks/checks/config');
const { assertLinear } = require('./perf-guard');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'X.java', config }).map((f) => f.id);

test('owns the JVM extensions', () => {
  assert.ok(EXTENSIONS.includes('.java') && EXTENSIONS.includes('.kt'));
});

test('flags SQL string building', () => {
  assert.ok(ids('stmt.executeQuery("SELECT * FROM t WHERE id = " + id);').includes('safe/sql-injection'));
  assert.ok(ids('String q = String.format("SELECT * FROM t WHERE id = %s", id);\nstmt.executeQuery(q);').includes('safe/sql-injection'));
  assert.ok(!ids('PreparedStatement ps = conn.prepareStatement("SELECT * FROM t WHERE id = ?");').includes('safe/sql-injection'));
});

test('flags unsafe deserialization and XXE-prone parsing', () => {
  assert.ok(ids('ObjectInputStream in = new ObjectInputStream(payload);').includes('safe/unsafe-deserialize'));
  assert.ok(ids('DocumentBuilderFactory.newInstance()').includes('safe/xxe-risk'));
});

test('flags weak hashing and predictable randomness', () => {
  assert.ok(ids('MessageDigest.getInstance("MD5")').includes('safe/weak-hash'));
  assert.ok(ids('String token = String.valueOf(new Random().nextLong());').includes('safe/weak-random'));
  assert.ok(!ids('MessageDigest.getInstance("SHA-256")').includes('safe/weak-hash'));
});

test('flags shell injection', () => {
  assert.ok(ids('Runtime.getRuntime().exec("git log " + branch);').includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
  assert.ok(ids('new ProcessBuilder("sh", "-c", cmd).start();').includes('safe/shell-injection'));
  assert.ok(!ids('Runtime.getRuntime().exec(new String[]{"git", "log", branch});').includes('safe/shell-injection'));
  assert.ok(!ids('new ProcessBuilder("git", "log", cmd).start();').includes('safe/shell-injection'));
});

test('flags swallowed exceptions', () => {
  assert.ok(ids('try { go(); } catch (Exception e) { }').includes('true/swallowed-error'));  // procoder: literal true/swallowed-error the Java snippet handed to the pack as input
  assert.ok(ids('catch (IOException e) { e.printStackTrace(); }').includes('true/printstacktrace'));
  assert.ok(!ids('catch (IOException e) { log.error("read failed", e); throw e; }').includes('true/swallowed-error'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('System.out.println("here");').includes('alone/debug-leftover'));
  assert.ok(!ids('log.info("started");').includes('alone/debug-leftover'));
});

// Both directions of the shared principle: a rule sees code, not prose, and
// the string literals a sink is assembled from are code.
test('ignores rules named in comments, not the code beside them', () => {
  assert.ok(!ids('// never stmt.executeQuery("SELECT * FROM t WHERE id = " + id);')
    .includes('safe/sql-injection'));
  assert.ok(!ids('// never new ObjectInputStream(payload)').includes('safe/unsafe-deserialize'));
  assert.ok(!ids('/* MessageDigest.getInstance("MD5") is not a password hash */')
    .includes('safe/weak-hash'));
  assert.ok(!ids('// never Runtime.getRuntime().exec("git log " + branch);')  // procoder: literal safe/shell-injection, safe/sql-injection the commented Java snippet the pack must NOT flag
    .includes('safe/shell-injection'));
  assert.ok(!ids('// e.printStackTrace() is not error handling').includes('true/printstacktrace'));
  assert.ok(!ids('// System.out.println("here") was removed').includes('alone/debug-leftover'));
  assert.ok(!ids('// DocumentBuilderFactory.newInstance() needs hardening').includes('safe/xxe-risk'));

  assert.ok(ids('stmt.executeQuery("SELECT * FROM t WHERE id = " + id); // never do this')
    .includes('safe/sql-injection'));
  assert.ok(ids('var in = new ObjectInputStream(payload); // bad').includes('safe/unsafe-deserialize'));
  assert.ok(ids('MessageDigest.getInstance("MD5"); // bad').includes('safe/weak-hash'));
  assert.ok(ids('Runtime.getRuntime().exec("git log " + branch); // bad')  // procoder: literal safe/shell-injection, safe/sql-injection the same Java snippet uncommented, which the pack must flag
    .includes('safe/shell-injection'));
  assert.ok(ids('e.printStackTrace(); // bad').includes('true/printstacktrace'));
  assert.ok(ids('System.out.println("here"); // bad').includes('alone/debug-leftover'));
  assert.ok(ids('DocumentBuilderFactory.newInstance(); // bad').includes('safe/xxe-risk'));
});

// Hardening promised in a comment is not hardening: prose must not discharge
// a SAFE rule, or the discharge becomes the cheapest way to silence it.
test('a commented-out hardening call does not discharge xxe-risk', () => {
  const commented = [
    'DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();',
    '// dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);',
  ].join('\n');
  assert.ok(ids(commented).includes('safe/xxe-risk'));
});

test('keeps seeing sinks built inside string literals', () => {
  assert.ok(ids('stmt.executeQuery("SELECT * FROM t WHERE id = " + id);')
    .includes('safe/sql-injection'));
  assert.ok(ids('new ProcessBuilder("sh", "-c", "rm " + dir).start();')
    .includes('safe/shell-injection'));
  // A URL inside a string is not the start of a comment.
  assert.ok(ids('String u = "http://h/x"; System.out.println(u);').includes('alone/debug-leftover'));
});

test('the signature regex does not treat if/catch as methods', () => {
  const src = [
    'class X {',
    '    void run() {',
    '        if (ready()) {',
    '            doThing();',
    '        }',
    '        try {',
    '            doThing();',
    '        } catch (Exception e) {',
    '            log.error("failed", e);',
    '            throw e;',
    '        }',
    '    }',
    '}',
  ].join('\n');
  // Must not crash, and must not report a shape finding rooted at the
  // if/catch lines as if they were methods with their own signature — the
  // whole snippet is short and simple, so a correct scan reports nothing.
  const findings = check(src, { relPath: 'X.java', config });
  assert.deepStrictEqual(findings, []);
});

test('xxe-risk stays silent when the very next lines disable DOCTYPE', () => {
  const hardened = [
    'DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();',
    'dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);',
  ].join('\n');
  assert.ok(!ids(hardened).includes('safe/xxe-risk'));

  const bare = 'DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();';
  assert.ok(ids(bare).includes('safe/xxe-risk'));

  // The lookahead is bounded (a handful of lines) — hardening far away in
  // the same method does not retroactively excuse a bare factory.
  const farAway = [
    'DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();',
    'log.info("building parser");',
    'log.info("step 2");',
    'log.info("step 3");',
    'log.info("step 4");',
    'log.info("step 5");',
    'log.info("step 6");',
    'dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);',
  ].join('\n');
  assert.ok(ids(farAway).includes('safe/xxe-risk'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'jvm');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.java'), 'utf8'),
    { relPath: 'clean.java', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.java'), 'utf8'),
    { relPath: 'dirty.java', config }).length >= 5);
});

// Multi-param generics are idiomatic Java; a space after the comma must not
// hide the method from shape measurement.
test('measures methods whose generic return type contains a space', () => {
  const method = (returnType) => [
    `public ${returnType} run(String a, String b, String c, String d, String e) {`,
    '    return null;',
    '}',
  ].join('\n');
  assert.ok(ids(method('Map<String, List<Integer>>')).includes('obvious/too-many-params'));
  assert.ok(ids(method('Map<String,List<Integer>>')).includes('obvious/too-many-params'));
});

// Perf guard — see tests/perf-guard.js for the bound and why it is relative.
test('stays linear on a very long single line', () => {
  assertLinear({
    assert,
    check,
    relPath: 'X.java',
    config,
    baseline: 'return;  ',
    units: [
      'new ProcessBuilder("sh", ',
      'Runtime.getRuntime().exec(a ',
      'String token = ',
      'checkServerTrusted(a ',
      'var x = f(a, b) + "s" + c; ',
      'String q = "SELECT " + id; ',
      'stmt.executeQuery(q); ',
      'Runtime.getRuntime().exec(cmd); ',
    ],
    sources: [
      'x'.repeat(100 * 1024),
      // Nested calls that DO close, so paramSpans records a span for each —
      // see lang-ts.test.js for why this shape is the one to guard.
      'checkServerTrusted('.repeat(5000) + ')'.repeat(5000),
      'new ProcessBuilder('.repeat(5000) + ')'.repeat(5000),
    ],
  });
});

// Local taint: the assign-then-use form, at least as common as the inline one.
// Reported at the sink, naming the line the value was built on.
test('tracks a concatenated or formatted string from its assignment to a sink', () => {
  const src = 'void f(Statement stmt, String id) {\n  String q = "SELECT * FROM t WHERE id=" + id;\n  stmt.executeQuery(q);\n}';
  const hit = check(src, { relPath: 'X.java', config }).find((f) => f.id === 'safe/sql-injection');
  assert.ok(hit, 'no safe/sql-injection for the assign-then-use form');
  assert.strictEqual(hit.line, 3, 'reported at the sink, not the assignment');
  assert.match(hit.message, /line 2/);

  assert.ok(ids('void f(Statement stmt, String id) {\n  String q = String.format("SELECT * FROM t WHERE id=%s", id);\n  stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void f(String dir) {\n  String cmd = "ls " + dir;\n  Runtime.getRuntime().exec(cmd);\n}')
    .includes('safe/shell-injection'));
});

test('a bound-parameter or literal-only variable reaching a sink stays silent', () => {
  assert.ok(!ids('void f(PreparedStatement stmt, String id) {\n  String q = "SELECT * FROM t WHERE id = ?";\n  stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void f(Statement stmt) {\n  String q = "SELECT " + "a, b" + " FROM t";\n  stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void f() {\n  String cmd = "ls -la";\n  Runtime.getRuntime().exec(cmd);\n}')
    .includes('safe/shell-injection'));
});

test('taint clears on a literal reassignment and does not leave its block', () => {
  assert.ok(!ids('void f(Statement stmt, String id) {\n  String q = "SELECT t WHERE id=" + id;\n  q = "SELECT * FROM t";\n  stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void a(String id) {\n  String q = "SELECT " + id;\n}\nvoid b(Statement stmt, String q) {\n  stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
});

// The 500-character span ceilings the SAFE rules used to carry: a sink whose
// interpolation sits further than that from the call was missed entirely.
const PAD = 'a'.repeat(600);

test('sees a sink whose interpolation is more than 500 characters from the call', () => {
  assert.ok(ids(`stmt.executeQuery("SELECT ${PAD} WHERE id = " + id);`).includes('safe/sql-injection'));
  assert.ok(ids(`Runtime.getRuntime().exec("git log ${PAD} " + branch);`).includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection the over-500-character scanner input for that rule, not an instance of it
  assert.ok(ids(`new ProcessBuilder("sh", "${PAD}", "-c", cmd).start();`).includes('safe/shell-injection'));
  assert.ok(ids(`String token = helper("${PAD}") + new Random().nextLong();`).includes('safe/weak-random'));
  assert.ok(ids(`public void checkServerTrusted(X509Certificate[] chain, String ${PAD}) {}`).includes('safe/tls-disabled'));
});

// Defect: a sink whose arguments are wholly constant carries nothing
// untrusted, and a rung-1 rule that fires on obviously-safe code is what gets
// the whole pack turned off. An identifier, a nested call, a concatenation or
// a Kotlin interpolation is data, and those still report.
test('a data sink whose arguments are wholly constant stays silent', () => {
  assert.ok(!ids('new ProcessBuilder("sh", "-c", "ls /tmp").start();').includes('safe/shell-injection'));
  assert.ok(!ids('Runtime.getRuntime().exec("ls " + "/tmp");').includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
});

test('the same sinks still report anything that is not a constant', () => {
  assert.ok(ids('new ProcessBuilder("sh", "-c", cmd).start();').includes('safe/shell-injection'));
  assert.ok(ids('new ProcessBuilder("sh", "-c", build(dir)).start();').includes('safe/shell-injection'));
  assert.ok(ids('new ProcessBuilder("sh", "-c", "rm $dir").start();').includes('safe/shell-injection'));
  assert.ok(ids('Runtime.getRuntime().exec("git log " + branch);').includes('safe/shell-injection'));  // procoder: literal safe/sql-injection, safe/shell-injection scanner input for that rule, not an instance of it
});

// Defect: `q = q + id` cleared the taint at the moment it should have been
// introduced — the right-hand side is two names with no literal, so the source
// patterns missed it and the clear-on-any-other-assignment rule fired.
test('an assignment whose right-hand side is a tainted name carries the taint', () => {
  assert.ok(ids('void f(String id) {\n    String q = "SELECT id=";\n    q = q + id;\n    stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void f(String id) {\n    String b = String.format("x%s", id);\n    String a = b;\n    stmt.executeQuery(a);\n}')
    .includes('safe/sql-injection'));
});

test('an append carries the taint of the whole value it builds', () => {
  assert.ok(ids('void f(String id) {\n    String q = "SELECT id=";\n    q += id;\n    stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void f(String id) {\n    String q = String.format("SELECT %s", id);\n    q += " LIMIT 1";\n    stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
  assert.ok(!ids('void f() {\n    String q = "SELECT a";\n    q += " FROM t";\n    stmt.executeQuery(q);\n}')
    .includes('safe/sql-injection'));
});

// Defect: a parameter is a fresh binding. One that happens to share a name
// with a tainted variable in an enclosing scope must start clean.
test('a parameter shadowing a tainted name starts clean', () => {
  assert.ok(!ids('void f(String id) {\n    String q = "SELECT " + id;\n    Runnable r = new Runnable() {\n        void run(String q) {\n            stmt.executeQuery(q);\n        }\n    };\n}')
    .includes('safe/sql-injection'));
  assert.ok(ids('void f(String id) {\n    String q = "SELECT " + id;\n    Runnable r = new Runnable() {\n        void run(String other) {\n            stmt.executeQuery(q);\n        }\n    };\n}')
    .includes('safe/sql-injection'));
});

test('the safe forms stay silent however long the arguments are', () => {
  assert.ok(!ids(`stmt.executeQuery("SELECT ${PAD} WHERE id = ?");`).includes('safe/sql-injection'));
  assert.ok(!ids(`new ProcessBuilder("git", "${PAD}", "log", branch).start();`).includes('safe/shell-injection'));
  assert.ok(!ids(`String token = helper("${PAD}") + secureRandom.nextLong();`).includes('safe/weak-random'));
  assert.ok(!ids(`public void checkServerTrusted(X509Certificate[] chain, String ${PAD}) { verify(chain); }`).includes('safe/tls-disabled'));
});
