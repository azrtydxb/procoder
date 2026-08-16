// tests/lang-comments.test.js
//
// The rule-matching principle the six packs share: comments (and regex
// bodies) are not code, string literals are.
const test = require('node:test');
const assert = require('node:assert');
const { stripComments } = require('../hooks/checks/lang/comments');

// The blanked stand-in for a piece of source: same width, so offsets and line
// numbers on either side of it are unchanged.
const gone = (text) => text.replace(/[^\n]/g, ' ');

test('blanks line and block comments, keeping line numbers', () => {
  assert.strictEqual(stripComments('a();\n// never eval(x)\nb();\n', 'c'),  // procoder: literal safe/dynamic-eval the source given to stripComments, where the eval sits in a comment
    `a();\n${gone('// never eval(x)')}\nb();\n`);  // procoder: literal safe/dynamic-eval the expected output of that same source
  assert.strictEqual(stripComments('a(); /* eval(x)\nmore */ b();', 'c'),  // procoder: literal safe/dynamic-eval the block-comment variant of the same input
    `a(); ${gone('/* eval(x)')}\n${gone('more */')} b();`);  // procoder: literal safe/dynamic-eval the expected output of the block-comment variant
});

test('keeps string literals — a sink can be assembled in one', () => {
  assert.strictEqual(stripComments('run("rm -rf " + dir); // gone', 'c'),
    `run("rm -rf " + dir);${gone(' // gone')}`);
  assert.strictEqual(stripComments('const q = `SELECT ${x}`; // gone', 'js'),
    `const q = \`SELECT \${x}\`;${gone(' // gone')}`);
});

test('a comment marker inside a string is not a comment', () => {
  assert.strictEqual(stripComments('u = "http://host/a"; // gone', 'js'),
    `u = "http://host/a";${gone(' // gone')}`);
  assert.strictEqual(stripComments('s = "# not a comment"  # gone', 'py'),
    `s = "# not a comment"  ${gone('# gone')}`);
});

test('blanks python comments and statement-position docstrings', () => {
  assert.strictEqual(stripComments('x = 1  # never eval(y)\n', 'py'),  // procoder: literal safe/dynamic-eval the Python source given to stripComments, eval inside a # comment
    `x = 1  ${gone('# never eval(y)')}\n`);  // procoder: literal safe/dynamic-eval the expected output of that Python source
  assert.strictEqual(stripComments('def f():\n    """never eval(y)"""\n', 'py'),  // procoder: literal safe/dynamic-eval the docstring variant of the same Python input
    `def f():\n    ${gone('"""never eval(y)"""')}\n`);  // procoder: literal safe/dynamic-eval the expected output of the docstring variant
  // A triple-quoted string used as a value is data, not documentation.
  assert.strictEqual(stripComments('t = """SELECT 1"""\n', 'py'), 't = """SELECT 1"""\n');
});

test('blanks regex bodies in js only, and never mistakes division for one', () => {
  assert.strictEqual(stripComments('const re = /eval\\(/;', 'js'), 'const re = /      /;');
  assert.strictEqual(stripComments('x = a/b/c;', 'js'), 'x = a/b/c;');
  assert.strictEqual(stripComments('x = a / b; eval(y)', 'js'), 'x = a / b; eval(y)');  // procoder: literal safe/dynamic-eval input and expected output for a division that is not a regex
  // Other grammars have no regex literal — `/` is only ever division there.
  assert.strictEqual(stripComments('x := a/b/c', 'c'), 'x := a/b/c');
});

test('an unterminated quote stops at the line end', () => {
  // A Rust lifetime must not swallow the file; the worst case is that this
  // line keeps its comment, which only ever costs a finding already reported.
  assert.strictEqual(stripComments("fn f<'a>(s: &'a str) {}\n// gone\n", 'c'),
    `fn f<'a>(s: &'a str) {}\n${gone('// gone')}\n`);
});

test('never throws on odd input', () => {
  for (const src of [null, undefined, '', '"', '`', '/*', '/', "'''", '#']) {
    assert.doesNotThrow(() => stripComments(src, 'js'));
    assert.doesNotThrow(() => stripComments(src, 'py'));
  }
});

test('stays linear on a very long single line', () => {
  for (const unit of ['a = "x"; // c ', 'b = /re/ + `t${x}` ', '/* c */ q("s" + v) ']) {
    const line = unit.repeat(Math.ceil((400 * 1024) / unit.length)).slice(0, 400 * 1024);
    const started = Date.now();
    stripComments(line, 'js');
    const elapsed = Date.now() - started;
    assert.ok(elapsed < 1000, `400KB line took ${elapsed}ms for ${JSON.stringify(unit)}`);
  }
});
