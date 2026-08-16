const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/jvm');
const { DEFAULTS } = require('../hooks/checks/config');

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
  assert.ok(ids('Runtime.getRuntime().exec("git log " + branch);').includes('safe/shell-injection'));
  assert.ok(ids('new ProcessBuilder("sh", "-c", cmd).start();').includes('safe/shell-injection'));
  assert.ok(!ids('Runtime.getRuntime().exec(new String[]{"git", "log", branch});').includes('safe/shell-injection'));
  assert.ok(!ids('new ProcessBuilder("git", "log", cmd).start();').includes('safe/shell-injection'));
});

test('flags swallowed exceptions', () => {
  assert.ok(ids('try { go(); } catch (Exception e) { }').includes('true/swallowed-error'));
  assert.ok(ids('catch (IOException e) { e.printStackTrace(); }').includes('true/printstacktrace'));
  assert.ok(!ids('catch (IOException e) { log.error("read failed", e); throw e; }').includes('true/swallowed-error'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('System.out.println("here");').includes('alone/debug-leftover'));
  assert.ok(!ids('log.info("started");').includes('alone/debug-leftover'));
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

// Perf guard: every rule here must stay linear in line length. Each unit below
// is an adversarial prefix — repeated, it makes any unbounded span in a rule
// re-scan to end of line from every offset, which is the quadratic shape that
// took .NET's safe/shell-injection rule 4.7s on this input. Linear runs finish
// in ~10ms, so 1s separates "linear" from "regression" with plenty of slack for
// a loaded CI machine; the hook's whole budget is 2s.
test('stays linear on a very long single line', () => {
  const units = [
    "new ProcessBuilder(\"sh\", ",
    "Runtime.getRuntime().exec(a ",
    "String token = ",
    "checkServerTrusted(a ",
    "var x = f(a, b) + \"s\" + c; ",
  ];
  for (const unit of units) {
    const line = unit.repeat(Math.ceil((100 * 1024) / unit.length)).slice(0, 100 * 1024);
    const started = Date.now();
    check(line, { relPath: 'X.java', config });
    const elapsed = Date.now() - started;
    assert.ok(elapsed < 1000, `100KB line took ${elapsed}ms for unit ${JSON.stringify(unit)}`);
  }
});
