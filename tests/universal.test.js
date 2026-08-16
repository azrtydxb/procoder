// tests/universal.test.js
const test = require('node:test');
const assert = require('node:assert');
const { checkUniversal } = require('../hooks/checks/universal');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const run = (src) => checkUniversal(src, { relPath: 'x.js', config });
const ids = (src) => run(src).map((f) => f.id);

test('flags hardcoded secrets of several shapes', () => {
  assert.ok(ids('const k = "AKIAIOSFODNN7EXAMPLE";').includes('safe/hardcoded-secret'));  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  assert.ok(ids('token = "ghp_aBcD1234567890aBcD1234567890aBcD12"').includes('safe/hardcoded-secret'));  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  assert.ok(ids('-----BEGIN RSA PRIVATE KEY-----').includes('safe/hardcoded-secret'));  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  assert.ok(ids('const password = "hunter2correcthorse";').includes('safe/hardcoded-secret'));  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
});

test('does not flag secrets read from the environment or a manager', () => {
  assert.deepStrictEqual(ids('const key = process.env.API_KEY;'), []);
  assert.deepStrictEqual(ids('password = os.environ["DB_PASSWORD"]'), []);
  assert.deepStrictEqual(ids('const token = await secrets.get("stripe");'), []);
  assert.deepStrictEqual(ids('password = "" # set at startup'), []);
});

test('flags secrets and PII reaching log calls', () => {
  assert.ok(ids('logger.info(`auth ${token}`)').includes('safe/secret-in-log'));  // procoder: literal safe/secret-in-log scanner input for that rule, not an instance of it
  assert.ok(ids('console.log("authorization: " + req.headers.authorization)').includes('safe/secret-in-log'));  // procoder: literal safe/secret-in-log, alone/debug-leftover scanner input for that rule, not an instance of it
  assert.ok(ids('log.debug(`session ${sessionId} rotated`)').includes('safe/secret-in-log'));  // procoder: literal safe/secret-in-log scanner input for that rule, not an instance of it
  assert.ok(ids('log.debug(f"user email {user.email}")').includes('safe/pii-in-log'));  // procoder: literal safe/pii-in-log scanner input for that rule, not an instance of it
  assert.deepStrictEqual(ids('logger.info(`user ${user.id} logged in`)'), []);
});

test('does not flag a sensitive word in the log message when it is not what is interpolated', () => {
  assert.deepStrictEqual(ids('log.info(`password validation ran for ${user.id}`)'), []);
  assert.deepStrictEqual(ids('log.info(`credential check passed for ${user.id}`)'), []);
  assert.deepStrictEqual(ids('logger.info(`user ${user.id} logged in`)'), []);
  assert.deepStrictEqual(ids('logger.warn("password reset requested for " + user.id)'), []);
});

test('still flags when the sensitive word is part of the interpolated expression itself', () => {
  assert.ok(ids('log.info(`${user.password}`)').includes('safe/secret-in-log'));  // procoder: literal safe/secret-in-log scanner input for that rule, not an instance of it
  assert.ok(ids('log.info(`${config.apiKey}`)').includes('safe/secret-in-log'));  // procoder: literal safe/secret-in-log scanner input for that rule, not an instance of it
});

test('flags a block of commented-out code but not prose comments', () => {
  const commented = [
    '// const x = compute(a, b);',
    '// if (x > 3) {',
    '//   send(x);',
    '// }',
  ].join('\n');
  assert.ok(ids(commented).includes('alone/commented-code'));

  const prose = [
    '// This exists because the upstream API returns 200 on failure.',
    '// See INFRA-4821 for the vendor ticket.',
    '// Remove once they ship the fix.',
  ].join('\n');
  assert.ok(!ids(prose).includes('alone/commented-code'));
});

test('still flags commented-out code that has no leading keyword', () => {
  const commented = [
    '// const q = `SELECT * FROM t`;',
    '// db.query(q);',
    '// return q;',
  ].join('\n');
  assert.ok(ids(commented).includes('alone/commented-code'));
});

test('reads measured explanations as prose, not commented-out code', () => {
  // The exact comment procoder's own self-scan used to flag: a why-comment that
  // records the measurements behind a threshold. Rung 3 asks for this.
  const measured = [
    '// Total-size skip. Cost of everything that survives the line guard below is',
    '// linear in file size: measured end to end on many-short-line files, 1MB = 34ms,',
    '// 4MB = 137ms, 8MB = 279ms. 4MB is ~7% of the budget and larger than any file a',
    '// human edits, so that is where the skip sits — not the old 256KB, which was',
    '// inherited from when a long line could blow the budget and threw away real',
    '// findings on ordinary large sources.',
  ].join('\n');
  assert.deepStrictEqual(ids(measured), []);

  const formula = [
    '// The ranking is deliberately linear, so a reviewer can predict it:',
    '// score = weight * signal, normalised so that 1.0 = perfect',
    '// Anything cleverer was impossible to explain in review.',
  ].join('\n');
  assert.deepStrictEqual(ids(formula), []);

  const versions = [
    '// The shim exists only for the broken release window.',
    '// Upstream fixed this in v2.4 = 2025-11-03; remove the shim after',
    '// every consumer has moved past that release.',
  ].join('\n');
  assert.deepStrictEqual(ids(versions), []);
});

test('flags TODOs without an owner or ticket', () => {
  assert.ok(ids('// TODO: fix this later').includes('alone/orphan-todo'));  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
  assert.ok(!ids('// TODO(pascal): drop the shim').includes('alone/orphan-todo'));
  assert.ok(!ids('// TODO INFRA-4821: drop the shim').includes('alone/orphan-todo'));
});

test('flags blanket suppressions but not narrow, named, explained ones', () => {
  // File-wide or unnamed: silences everything at that location, including future findings.
  assert.ok(ids('/* eslint-disable */').includes('alone/blanket-suppression'));  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it
  assert.ok(ids('// eslint-disable-next-line').includes('alone/blanket-suppression'));  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it
  assert.ok(ids('x = compute()  # noqa').includes('alone/blanket-suppression'));  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it
  assert.ok(ids('y = cast(v)  # type: ignore').includes('alone/blanket-suppression'));  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it
  assert.ok(ids('//nolint').includes('alone/blanket-suppression'));  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it
  assert.ok(ids('@SuppressWarnings("all")').includes('alone/blanket-suppression'));  // procoder: literal alone/blanket-suppression scanner input for that rule, not an instance of it

  // Named + scoped + explained: the sanctioned form, must stay silent.
  assert.deepStrictEqual(
    ids('// eslint-disable-next-line no-eval -- sandboxed evaluator, input is a literal'), []);
  assert.deepStrictEqual(ids('x = compute()  # noqa: E501 - URL cannot be wrapped'), []);
  assert.deepStrictEqual(
    ids('//nolint:errcheck // Close() on a read-only handle cannot fail'), []);
});

test('flags a named suppression that gives no reason', () => {
  assert.ok(ids('// eslint-disable-next-line no-eval').includes('alone/unexplained-suppression'));  // procoder: literal alone/unexplained-suppression scanner input for that rule, not an instance of it
  assert.ok(ids('#pragma warning disable CS0618').includes('alone/unexplained-suppression'));  // procoder: literal alone/unexplained-suppression scanner input for that rule, not an instance of it
});

test('flags a suppression with a reason expressed as trailing prose, no separator', () => {
  assert.deepStrictEqual(
    ids('# noqa: E501 this line intentionally exceeds the limit for readability'), []);
});

test('flags deprecations with no removal trigger', () => {
  assert.ok(ids('// @deprecated use createUser instead').includes('alone/deprecated-no-trigger'));  // procoder: literal alone/deprecated-no-trigger scanner input for that rule, not an instance of it
  assert.ok(!ids('// @deprecated remove after v3.0').includes('alone/deprecated-no-trigger'));
  assert.ok(!ids('// procoder: remove after the 2026-10 migration').includes('alone/deprecated-no-trigger'));
});

test('the clean fixture produces no findings and the dirty one produces several', () => {
  const fs = require('fs');
  const path = require('path');
  const dir = path.join(__dirname, 'fixtures', 'universal');
  assert.strictEqual(
    checkUniversal(fs.readFileSync(path.join(dir, 'clean.txt'), 'utf8'),
      { relPath: 'clean.txt', config }).length, 0);
  assert.ok(
    checkUniversal(fs.readFileSync(path.join(dir, 'dirty.txt'), 'utf8'),
      { relPath: 'dirty.txt', config }).length >= 5);
});

test('every finding carries a line number and a fix', () => {
  for (const f of run('const k = "AKIAIOSFODNN7EXAMPLE";\n// TODO: later\n')) {  // procoder: literal safe/hardcoded-secret, alone/orphan-todo scanner input for that rule, not an instance of it
    assert.ok(f.line > 0, `${f.id} has no line`);
    assert.ok(f.fix.length > 0, `${f.id} has no fix`);
  }
});

test('stays cheap on a long comment run of long near-code lines', () => {
  // Worst case for the assignment arm: every line starts with something that
  // walks the member-expression loop before failing to reach an `=`.
  const line = '// ' + 'a.b[0].c'.repeat(500) + ' measured at 1MB = 34ms';
  const start = Date.now();
  run(Array.from({ length: 2000 }, () => line).join('\n'));
  const ms = Date.now() - start;
  assert.ok(ms < 500, `took ${ms}ms on pathological comment run`);
});

test('does not catastrophically backtrack on a long line', () => {
  const long = 'x = ' + 'a'.repeat(20000) + ';  // eslint-disable-next-line no-eval no reason here but long';
  const start = Date.now();
  run(long);
  assert.ok(Date.now() - start < 500, 'took too long on pathological input');
});

// A 300KB minified bundle on one line — the file the pack is exempted from the
// line-length guard in order to keep scanning for credentials. Each shape below
// used to drive an unbounded `[^x]*` span to end-of-line from every start
// position: 300KB cost ~36s, against a 2s hook budget.
//
// Bound: 500ms for all three, measured at ~2ms each. The margin is deliberately
// enormous — a loaded CI runner can lose a factor of fifty and still pass, while
// any return of the quadratic behaviour costs seconds to tens of seconds and
// cannot slip under it. The point of the assertion is the growth rate, not the
// constant, so it is not tightened to the measurement.
test('stays linear on a single 300KB line', () => {
  const rep = (unit) => unit.repeat(Math.ceil((300 * 1024) / unit.length));
  const shapes = {
    'word run in a comment': '// ' + rep('a'),
    'unclosed parens in a comment': '// ' + rep('a('),
    'unclosed interpolation in a log call': 'console.log("' + rep('${') + '")',  // procoder: literal alone/debug-leftover scanner input for that rule, not an instance of it
  };
  for (const [what, source] of Object.entries(shapes)) {
    const start = Date.now();
    run(source);
    const ms = Date.now() - start;
    assert.ok(ms < 500, `${what}: took ${ms}ms on a 300KB line`);
  }
});

// A credential on such a line is exactly what the exemption exists to catch, so
// speed must not have been bought by skipping the line.
test('finds a credential on a 300KB minified line', () => {
  const filler = 'function e(t){return t.x?f(t,1):g(t)};var n=[1,2,3];'.repeat(3000);
  const findings = run(`${filler};var q="AKIAIOSFODNN7EXAMPLE";${filler}`);  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  assert.ok(findings.some((f) => f.id === 'safe/hardcoded-secret'),
    'a hardcoded AWS key on a 300KB line went unreported');
});

// Speed used to be bought with a 200-character ceiling on every `[^x]*` span,
// which is a real blind spot rather than a safe approximation: an interpolated
// expression or an argument list longer than that fell out of the check
// entirely. The spans are walked left to right now — one pass, nothing capped —
// so length is no longer what decides whether a line is examined.
test('a long interpolated expression in a log call is still seen', () => {
  const expr = 'user.' + 'profile.'.repeat(40) + 'password';
  assert.ok(ids('logger.info(`auth ${' + expr + '}`)').includes('safe/secret-in-log'),  // procoder: literal safe/secret-in-log scanner input for that rule, not an instance of it
    `a ${expr.length}-character interpolated credential went unreported`);
});

test('a commented-out call with a long argument list is still seen', () => {
  const args = 'argument, '.repeat(30);
  const block = [`// send(${args}payload)`, `// retry(${args}payload)`, '// note'].join('\n');
  assert.ok(ids(block).includes('alone/commented-code'),
    `commented-out calls with ${args.length}-character argument lists went unreported`);
});

// Nothing above may cost what the ceiling was bought to prevent. Same bound and
// same reasoning as the 300KB test: the assertion is the growth rate.
test('stays linear on a single 400KB line with unbounded spans in it', () => {
  const rep = (unit) => unit.repeat(Math.ceil((400 * 1024) / unit.length));
  const shapes = {
    'unclosed interpolation': 'console.log(`token ${' + rep('a') + '`)',  // procoder: literal alone/debug-leftover, safe/secret-in-log scanner input for that rule, not an instance of it
    'one huge closed argument list in a comment': '// send(' + rep('a,') + 'x)',
    'many opens, one close': '// f(' + rep('a(') + ')',
  };
  for (const [what, source] of Object.entries(shapes)) {
    const start = Date.now();
    run(source);
    const ms = Date.now() - start;
    assert.ok(ms < 500, `${what}: took ${ms}ms on a 400KB line`);
  }
});

// ---------------------------------------------------------------------------
// The literal marker: text that describes a pattern, marked as such.
//
// MARK is assembled at runtime rather than written out, so that the lines of
// this section are not themselves markers — the tests below feed marker text
// to the pack as *data*, and a real marker in the test source would silence
// the very findings these tests assert on. Likewise the AWS key: split, so it
// is a credential only once the pack sees it.
const MARK = 'procoder' + ': literal ';
const KEY = 'AKIA' + 'IOSFODNN7EXAMPLE';

test('a trailing literal marker silences the rules it names, on its line only', () => {
  assert.deepStrictEqual(
    ids(`const k = "${KEY}"; // ${MARK}safe/hardcoded-secret sample key in a doc`), []);

  // Trailing, so it reaches no further than the line it sits on.
  assert.ok(ids(`const a = 1; // ${MARK}safe/hardcoded-secret sample key in a doc\nconst k = "${KEY}";`)
    .includes('safe/hardcoded-secret'));
});

test('a standalone literal marker reaches the following line, and no further', () => {
  assert.deepStrictEqual(
    ids(`// ${MARK}safe/hardcoded-secret the key below is documentation\nconst k = "${KEY}";`), []);

  assert.ok(ids(`// ${MARK}safe/hardcoded-secret the key below is documentation\nconst a = 1;\nconst k = "${KEY}";`)
    .includes('safe/hardcoded-secret'));
});

test('a literal marker silences nothing it did not name', () => {
  assert.ok(ids(`const k = "${KEY}"; // ${MARK}alone/orphan-todo wrong rule named here`)
    .includes('safe/hardcoded-secret'));
  assert.deepStrictEqual(
    ids(`// TODO: later ${MARK}alone/orphan-todo describes an unowned marker`), []);  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it
});

test('a literal marker may name several rules', () => {
  assert.deepStrictEqual(
    ids(`// TODO: later, deprecated too // ${MARK}alone/orphan-todo, alone/deprecated-no-trigger both described here`),  // procoder: literal alone/orphan-todo, alone/deprecated-no-trigger scanner input for that rule, not an instance of it
    []);
});

test('a bare literal marker is a blanket suppression and suppresses nothing', () => {
  const found = ids(`const k = "${KEY}"; // procoder` + ': literal');
  assert.ok(found.includes('alone/blanket-suppression'), 'a marker naming no rule must be reported');
  assert.ok(found.includes('safe/hardcoded-secret'), 'a marker naming no rule must silence nothing');
});

test('a literal marker with no reason is unexplained and suppresses nothing', () => {
  const found = ids(`const k = "${KEY}"; // ${MARK}safe/hardcoded-secret`);
  assert.ok(found.includes('alone/unexplained-suppression'), 'a marker with no reason must be reported');
  assert.ok(found.includes('safe/hardcoded-secret'), 'a marker with no reason must silence nothing');
});

test('a well-formed literal marker is not itself reported as a suppression', () => {
  assert.deepStrictEqual(
    ids(`// ${MARK}alone/orphan-todo this line is a marker and nothing else`), []);
});

// The marker is only useful if it reaches the language packs too: most of what
// a test file or a doctrine page describes is an injection sink or a debug
// statement, not a credential. checkFile applies this to every finding it has.
test('filterMarkedLiterals applies the same marker to findings from any pack', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `const q = "SELECT 1"; // ${MARK}safe/sql-injection the query above is a doc example\nconst r = "SELECT 2";`;
  const findings = [
    { id: 'safe/sql-injection', line: 1 },
    { id: 'safe/sql-injection', line: 2 },
    { id: 'safe/xss-sink', line: 1 },
  ];
  assert.deepStrictEqual(
    filterMarkedLiterals(source, findings).map((f) => `${f.id}:${f.line}`),
    ['safe/sql-injection:2', 'safe/xss-sink:1']);
});

// A configured linter's findings carry the tool's own rule id — `true/eslint:
// no-eval`, `true/ruff:E501` — so a check id may contain a colon, an uppercase
// letter, a digit, and (for a scoped eslint plugin) a second slash and an `@`.
// The marker's rule-id list matched `[a-z]+/[a-z-]+` only, so a marker naming
// one of those parsed as a *bare* marker: reported as a blanket suppression,
// silencing nothing, which is the same defect .procoder.toml's rule exclusions
// were already fixed for by splitting on the first colon rather than the last.
test('a literal marker can name an external linter rule id', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `evil(); // ${MARK}true/eslint:no-eval the rule id above is documentation`;
  assert.deepStrictEqual(
    filterMarkedLiterals(source, [{ id: 'true/eslint:no-eval', line: 1 }]), []);

  const scoped = `x(); // ${MARK}true/eslint:@typescript-eslint/no-explicit-any, true/ruff:E501 both described`;
  assert.deepStrictEqual(
    filterMarkedLiterals(scoped, [
      { id: 'true/eslint:@typescript-eslint/no-explicit-any', line: 1 },
      { id: 'true/ruff:E501', line: 1 },
    ]), []);

  // And it is a well-formed suppression, not a bare one.
  assert.deepStrictEqual(ids(source), []);
});

// A marker naming an id the engine cannot produce — a typo, or an id renamed
// since — silenced nothing and said nothing, so the author believed a line was
// marked when it was not. Two choices made here: the complaint goes to stderr,
// because the PostToolUse hook's stdout carries a JSON protocol and stderr is
// where config.js already reports an exclusion it is dropping; and the unknown
// id suppresses nothing, so the underlying finding still appears — the safe
// direction, since a marker that does not do what its author thinks must not
// also hide the thing they were looking at.
function withStderr(work) {
  const written = [];
  const real = process.stderr.write;
  process.stderr.write = (chunk) => { written.push(String(chunk)); return true; };
  try {
    return { result: work(), stderr: written.join('') };
  } finally {
    process.stderr.write = real;
  }
}

test('a literal marker naming an unknown rule id warns and suppresses nothing', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const findings = [{ id: 'alone/no-such-rule-here', line: 1 }];
  const { result, stderr } = withStderr(() => filterMarkedLiterals(
    `x(); // ${MARK}alone/no-such-rule-here a plausible id that does not exist`, findings));

  assert.deepStrictEqual(result, findings, 'an unknown id must silence nothing');
  assert.ok(stderr.includes('alone/no-such-rule-here'),
    `the unknown id was swallowed rather than reported: ${JSON.stringify(stderr)}`);
});

test('the known ids in a marker still apply when it also names an unknown one', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `const k = "${KEY}"; // ${MARK}safe/hardcoded-secret, safe/hardcoded-secrets one real id and one typo`;
  const { result, stderr } = withStderr(() => filterMarkedLiterals(source, [
    { id: 'safe/hardcoded-secret', line: 1 },
    { id: 'alone/debug-leftover', line: 1 },
  ]));

  assert.deepStrictEqual(result.map((f) => f.id), ['alone/debug-leftover']);
  assert.ok(stderr.includes('safe/hardcoded-secrets'), 'the typo went unreported');
});

// An external tool's rule ids are the tool's own and cannot be enumerated, so
// they are accepted on shape. That must not become a hole a typo slips through:
// a plain built-in id is still checked against the real set.
test('an external linter rule id is accepted without a registry of its rules', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `evil(); // ${MARK}true/eslint:no-such-eslint-rule the id above is documentation`;
  const { result, stderr } = withStderr(() => filterMarkedLiterals(
    source, [{ id: 'true/eslint:no-such-eslint-rule', line: 1 }]));

  assert.deepStrictEqual(result, []);
  assert.strictEqual(stderr, '');
});

// The set is a list, and a list rots. Every check id the engine spells anywhere
// under hooks/ must be in it, or a marker naming a genuine id would be reported
// as a typo — the failure this fix exists to prevent, inverted.
// The colon form was accepted on shape alone: `<rung>/<word>:<anything>`. The
// rule half genuinely cannot be enumerated — it is the tool's — but the tool
// half is a closed set, the same four registry.js can invoke, so half the id
// can be checked and a misspelt tool is no longer the one marker mistake that
// fails silently.
test('a literal marker naming an unknown tool in a colon id warns', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const findings = [{ id: 'true/eslint:no-eval', line: 1 }];
  const { result, stderr } = withStderr(() => filterMarkedLiterals(
    `evil(); // ${MARK}true/eslnit:no-eval the tool name above is a typo`, findings));

  assert.deepStrictEqual(result, findings, 'a misspelt tool must silence nothing');
  assert.ok(stderr.includes('true/eslnit:no-eval'), `the misspelt tool went unreported: ${JSON.stringify(stderr)}`);
});

test('a colon id on a rung no external tool reports on warns', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const findings = [{ id: 'safe/hardcoded-secret', line: 1 }];
  const { result, stderr } = withStderr(() => filterMarkedLiterals(
    `const k = "${KEY}"; // ${MARK}safe/hardcoded-secret:x a colon id no pack can produce`, findings));

  assert.deepStrictEqual(result, findings);
  assert.ok(stderr.includes('safe/hardcoded-secret:x'), 'an unproducible colon id went unreported');
});

// The de-duplication key held the id and the line number and no file, so the
// same typo on the same line of a second file was silently swallowed — and the
// message never said which file to go and fix.
test('the same unknown id on the same line of two files warns for each, by name', () => {
  const source = `const k = "${KEY}"; // ${MARK}alone/no-such-rule-at-all a plausible id that does not exist`;
  const { stderr } = withStderr(() => {
    checkUniversal(source, { relPath: 'first.js', config });
    checkUniversal(source, { relPath: 'second.js', config });
  });

  assert.ok(stderr.includes('first.js'), `the first file was not named: ${JSON.stringify(stderr)}`);
  assert.ok(stderr.includes('second.js'), `the second file was not warned about: ${JSON.stringify(stderr)}`);
  assert.strictEqual(stderr.split('\n').filter((l) => l).length, 2, 'one warning per file, no more');
});

// Two passes over the same file — the pack's own, then run.js's over every
// pack's findings — must still warn once.
test('the two passes over one file warn once between them', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `const k = "${KEY}"; // ${MARK}alone/no-such-rule-twice a plausible id that does not exist`;
  const { stderr } = withStderr(() => {
    const own = checkUniversal(source, { relPath: 'once.js', config });
    assert.ok(own.length, 'the pack must produce the finding the second pass then filters');
    filterMarkedLiterals(source, own);
  });
  assert.strictEqual(stderr.split('\n').filter((l) => l).length, 1, `warned twice: ${JSON.stringify(stderr)}`);
});

test('two different unknown ids on one line are both named', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = `x(); // ${MARK}alone/typo-one, alone/typo-two two distinct typos here`;
  const { stderr } = withStderr(() => filterMarkedLiterals(source, [{ id: 'alone/debug-leftover', line: 1 }]));
  assert.ok(stderr.includes('alone/typo-one') && stderr.includes('alone/typo-two'), stderr);
});

test('every check id the engine can produce is in the known-id set', () => {
  const fs = require('fs');
  const path = require('path');
  const { BUILTIN_RULE_IDS } = require('../hooks/checks/patterns/markers');
  const known = new Set(BUILTIN_RULE_IDS);

  const walk = (dir) => fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const full = path.join(dir, e.name);
    return e.isDirectory() ? walk(full) : (full.endsWith('.js') ? [full] : []);
  });

  const missing = new Set();
  for (const file of walk(path.join(__dirname, '..', 'hooks'))) {
    const text = fs.readFileSync(file, 'utf8');
    for (const m of text.matchAll(/'((?:safe|true|obvious|alone)\/[a-z][a-z-]*)'/g)) {
      if (!known.has(m[1])) missing.add(`${m[1]} (${path.basename(file)})`);
    }
  }
  assert.deepStrictEqual([...missing], [], 'check ids missing from BUILTIN_RULE_IDS');
});

// ---------------------------------------------------------------------------
// Compound credential names.
//
// The pattern demanded a word boundary in FRONT of the credential word, so it
// only ever saw the bare spelling — `password = "..."`. Every ordinary
// identifier a real codebase uses (`dbPassword`, `api_secret`, `myApiKey`) put
// a word character there and went unreported, which is the majority spelling,
// not the exception.
const COMPOUND = [
  'const dbPassword = "hunter2correcthorse";',  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  'let userPassword = "hunter2correcthorse";',  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  'const apiSecret = "s3cr3tvaluehere";',  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  'api_secret = "s3cr3tvaluehere"',  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  'const authToken = "abcdef1234567890";',  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
  'const myApiKey = "abcdef1234567890";',  // procoder: literal safe/hardcoded-secret scanner input for that rule, not an instance of it
];

test('a credential literal is reported whatever the identifier is prefixed with', () => {
  for (const line of COMPOUND) {
    assert.ok(ids(line).includes('safe/hardcoded-secret'), `missed: ${line}`);
  }
});

// The other direction, and it matters more: this rule gates rung 1, so a
// loosened credential pattern is the classic false-positive generator. None of
// these is a credential, and none of them may report.
const NOT_CREDENTIALS = [
  'const passwordPolicy = "min-length-twelve";',
  'const tokenizer = "whitespace-splitter";',
  'const keyboard = "qwerty-us-layout";',
  'const monkeypatch = "replace-the-method";',
  'const names = keys("some-long-string");',
  'const v = secretsManager.get("production-db");',
  'const password = process.env.DB_PASSWORD;',
  'password = os.environ["DB_PASSWORD"]',
  'const secretsCache = new Map("seed-value-here");',
];

test('compound spellings do not turn ordinary identifiers into credentials', () => {
  for (const line of NOT_CREDENTIALS) {
    assert.deepStrictEqual(ids(line), [], `false positive: ${line}`);
  }
});

// ---------------------------------------------------------------------------
// A cross-line finding is reported at its sink and names the line the value was
// built on. A marker written at that build line — where the author sees the
// real subject of the finding — has to reach it, or an author who marks the
// concatenation gets no effect and no explanation.
const CROSS_LINE = [{
  rung: 'SAFE',
  id: 'safe/sql-injection',
  line: 4,
  message: 'query built from untrusted input, built at line 2',
  fix: 'parameterize',
}];

const crossLineSource = (rule) => [
  'function f(id) {',
  `  const q = "SELECT " + id; // ${MARK}${rule} the doctrine example, not a live query`,
  '  // ...',
  '  db.query(q);',
].join('\n');

test('a literal marker on the line a cross-line finding was built at silences it', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  assert.deepStrictEqual(
    filterMarkedLiterals(crossLineSource('safe/sql-injection'), CROSS_LINE), []);
});

test('the build-line marker still silences nothing it did not name', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  assert.deepStrictEqual(
    filterMarkedLiterals(crossLineSource('safe/xss-sink'), CROSS_LINE), CROSS_LINE);
});

test('marker scanning is cheap on a large marker-free file', () => {
  const { filterMarkedLiterals } = require('../hooks/checks/universal');
  const source = Array.from({ length: 20000 }, (_, i) => `const a${i} = compute(${i});`).join('\n');
  const findings = [{ id: 'safe/hardcoded-secret', line: 1 }];
  const start = Date.now();
  for (let i = 0; i < 20; i += 1) filterMarkedLiterals(source, findings);
  const ms = Date.now() - start;
  assert.ok(ms < 200, `marker scan cost ${ms}ms over 20 passes of a 20k-line file`);
});
