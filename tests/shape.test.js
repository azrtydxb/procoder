// tests/shape.test.js
const test = require('node:test');
const assert = require('node:assert');
const {
  analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
  measureFunctions, signaturesFrom, stripNoise,
} = require('../hooks/checks/shape');
const { DEFAULTS } = require('../hooks/checks/config');

// CPU milliseconds for the fastest of three runs — the same guard the language
// packs use, imported rather than re-declared here, because the copy that used
// to live in this file measured wall-clock and flaked under the concurrent
// suite. See tests/perf-guard.js for why CPU time is what these bounds mean.
const { bestOf } = require('./perf-guard');

// Quadrupling the input costs ~4x when the scan is linear and ~16x when it is
// quadratic, so 8 sits an equal distance from both. The same bound the two
// ratio guards further down already use, for one property with one answer.
const LINEAR_RATIO = 8;

// A pathological input against benign code of the *same size*, which is
// tests/perf-guard.js's shape: same function, same byte count, so every linear
// cost is on both sides and only the blow-up is on one. 40x is that file's
// PERF_MULTIPLE, and means the same thing here.
const PERF_MULTIPLE = 40;

// Why these are ratios and not millisecond bounds.
//
// `bestOf` already measures CPU time rather than wall clock, which is what
// stopped a *loaded* machine flaking these. It does nothing about a *slow* one:
// a host at a third of the speed spends three times the CPU on the same work,
// so an absolute bound is a bound on the runner. `signaturesFrom stays linear
// on a huge single-line file` asserted `< 500ms`, cost 108ms of CPU here, and
// failed on the macOS CI runner at 548ms — red for the project's whole history,
// on a scan that is linear and always was. Widening the number would only move
// the next runner that trips it.
//
// A ratio between two input sizes is speed-free by construction: both points
// are measured on whatever host is running, and only the *shape* of the growth
// survives the division. That was tried once during the CPU-time fix and
// rejected, because it read 4.6-15.4x under load — but it was measured with a
// 100KB small point costing ~1ms, where per-run overhead and a single GC pause
// are most of the number. Both points here are tens to hundreds of
// milliseconds, and the ratio holds: measured on a 10-core box at 0, 16 and 48
// competing spinners (4.8x oversubscription), signaturesFrom read 3.4, 3.2 and
// 3.2, and analyzeBraces plus measureFunctions read 3.7, 3.3 and 3.3 — against
// the 16 a quadratic returns.

test('analyzeBraces reports nesting depth and block spans', () => {
  const src = [
    'function a() {',      // 1
    '  if (x) {',          // 2
    '    while (y) {',     // 3
    '      go();',
    '    }',
    '  }',
    '}',
  ].join('\n');
  const out = analyzeBraces(src);
  assert.strictEqual(out.maxDepth, 3);
  assert.ok(out.blocks.some((b) => b.startLine === 1 && b.endLine === 7));
});

test('analyzeBraces ignores braces inside strings and comments', () => {
  const src = 'const s = "{{{";\n// }}}\nfunction a() {\n  go();\n}\n';
  assert.strictEqual(analyzeBraces(src).maxDepth, 1);
});

// A `/*` inside a string literal is not a comment opener. Blanking from there
// to the next `*/` fused two brace blocks into one and invented an 88-line
// function; any glob, regex or URL carrying `/*` in a string could do it, and
// the corruption runs as far as the next `*/` in the file.
test('a /* inside a string does not open a block comment', () => {
  const src = [
    'function a() {',                                        // 1
    '  const cfg = { toml: "paths = [\\"**/*.generated.ts\\"]" };',
    '}',                                                     // 3
    'function b() {',                                        // 4
    '  return 1; /* a real comment */',
    '}',                                                     // 6
  ].join('\n');
  const { blocks } = analyzeBraces(src);
  assert.ok(blocks.some((b) => b.startLine === 1 && b.endLine === 3),
    'the string swallowed the first function');
  assert.ok(blocks.some((b) => b.startLine === 4 && b.endLine === 6),
    `two blocks were fused into one: ${JSON.stringify(blocks)}`);
});

test('stripNoise blanks strings, comments and regexes without moving a line', () => {
  const src = [
    'const u = "http://host/*path";',
    'const re = /a{2,3}|b/g;',
    "// a comment with an apostrophe: don't",
    '/* block { } */',
    'go();',
  ].join('\n');
  const out = stripNoise(src).split('\n');
  assert.strictEqual(out.length, 5);
  assert.strictEqual(out[0], 'const u = "' + ' '.repeat('http://host/*path'.length) + '";');
  assert.strictEqual(out[1], 'const re = /' + ' '.repeat('a{2,3}|b'.length) + '/g;');
  assert.strictEqual(out[2].trim(), '');
  assert.strictEqual(out[3].trim(), '');
  assert.strictEqual(out[4], 'go();');
});

test('a quote inside a comment does not swallow the code below it', () => {
  const src = [
    "// don't count these: { { {",
    'function a() {',
    '  go();',
    '}',
  ].join('\n');
  assert.strictEqual(analyzeBraces(src).maxDepth, 1);
  assert.ok(analyzeBraces(src).blocks.some((b) => b.startLine === 2 && b.endLine === 4));
});

test('analyzeIndent reports depth for indentation languages', () => {
  const src = [
    'def a():',
    '    if x:',
    '        while y:',
    '            go()',
  ].join('\n');
  assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 3);
});

test('countParams handles defaults, generics, and destructuring', () => {
  assert.strictEqual(countParams('(a, b, c)'), 3);
  assert.strictEqual(countParams('()'), 0);
  assert.strictEqual(countParams('(a: Map<string, number>, b)'), 2);
  assert.strictEqual(countParams('({ a, b }, c = [1, 2])'), 2);
});

test('estimateComplexity counts branches, loops and logical operators', () => {
  assert.strictEqual(estimateComplexity('return 1;'), 1);
  assert.strictEqual(estimateComplexity('if (a) {} else if (b) {}'), 3);
  assert.strictEqual(estimateComplexity('if (a && b || c) {}'), 4);
  assert.ok(estimateComplexity('for (;;) { if (a) { while (b) {} } }') >= 4);
});

test('shapeFindings fires only above the configured thresholds', () => {
  const thresholds = DEFAULTS.thresholds;
  const none = shapeFindings({
    blocks: [{ startLine: 1, endLine: 10, length: 10, params: 2, complexity: 3 }],
    maxDepth: 2, thresholds, kind: 'function',
  });
  assert.deepStrictEqual(none, []);

  const ids = shapeFindings({
    blocks: [{ startLine: 1, endLine: 95, length: 95, params: 7, complexity: 22 }],
    maxDepth: 6, thresholds, kind: 'function',
  }).map((f) => f.id);
  assert.ok(ids.includes('obvious/function-too-long'));
  assert.ok(ids.includes('obvious/too-many-params'));
  assert.ok(ids.includes('obvious/complexity'));
  assert.ok(ids.includes('obvious/nesting-depth'));
});

test('a block-scoped switch case counts as a block, not a data literal', () => {
  const src = [
    'function a() {',        // 1
    '  switch (x) {',        // 2
    '    case 1: {',         // 3
    '      if (y) {',        // 4
    '        go();',
    '      }',
    '    }',
    '    default: {',
    '      stop();',
    '    }',
    '  }',
    '}',
  ].join('\n');
  assert.strictEqual(analyzeBraces(src).maxDepth, 4);
});

test('a pure data literal still does not count as nesting', () => {
  const src = [
    'const cfg = {',
    '  a: {',
    '    b: {',
    '      c: {',
    '        d: { e: 1 },',
    '      },',
    '    },',
    '  },',
    '};',
  ].join('\n');
  assert.strictEqual(analyzeBraces(src).maxDepth, 0);
});

test('a chain of control-flow blocks still counts as nesting', () => {
  const src = [
    'function a() {',
    '  for (;;) {',
    '    if (x) {',
    '      while (y) {',
    '        if (z) {',
    '          go();',
    '        }',
    '      }',
    '    }',
    '  }',
    '}',
  ].join('\n');
  assert.strictEqual(analyzeBraces(src).maxDepth, 5);
});

// A minified single-line file. Line numbering used to re-slice the whole source
// per match, so cost was quadratic: ~1.3s at 2MB, over the 2s whole-file hook
// budget on its own. Linear line indexing runs 4MB in a quarter of a second, so
// the quadratic term is what the two sizes are here to expose.
test('signaturesFrom stays linear on a huge single-line file', () => {
  const unit = 'function f(a,b){return a+b}';
  const re = { head: /function\s+\w*\(/g, tail: /\s*\{/ };
  const cost = (mb) => {
    const stripped = stripNoise(unit.repeat(Math.ceil((mb * 1024 * 1024) / unit.length)));
    return bestOf(3, () => signaturesFrom(stripped, re));
  };

  const small = cost(1);
  const large = cost(4);
  assert.ok(large / small < LINEAR_RATIO,
    `4x the file cost ${(large / small).toFixed(1)}x the time (${small}ms → ${large}ms)`);
});

// A minified line at 400KB and 1.6MB. At 400KB the two used to cost 2.0s and
// 44.8s respectively — each on its own over the 2s whole-file hook budget.
test('analyzeBraces and measureFunctions stay linear in line length', () => {
  const unit = 'function f(a,b){if(a&&b){return a+b}else{return 0}}';
  const re = { head: /function\s+\w*\(/g, tail: /\s*\{/ };
  const cost = (kb) => {
    const line = unit.repeat(Math.ceil((kb * 1024) / unit.length));
    return bestOf(3, () => {
      const { blocks } = analyzeBraces(line);
      measureFunctions([line], blocks, signaturesFrom(stripNoise(line), re));
    });
  };

  const small = cost(400);
  const large = cost(1600);
  assert.ok(large / small < LINEAR_RATIO,
    `4x the line length cost ${(large / small).toFixed(1)}x the time (${small}ms → ${large}ms)`);
});

// --- signatures that wrap across lines -------------------------------------
//
// A brace block was only measured when its opening line matched the pack's
// signature pattern, so a signature wrapped over several lines — what every
// prevailing formatter produces once the line width is exceeded — dropped the
// function out of measurement entirely. The parameter count is what suffers
// most: a wrapped signature usually means many parameters, so the functions
// obvious/too-many-params exists to catch were the ones it could not see.
// These go through the packs themselves, because the shapes are the packs'
// patterns applied to real formatter output.
const packs = {
  ts: require('../hooks/checks/lang/ts'),
  py: require('../hooks/checks/lang/py'),
  go: require('../hooks/checks/lang/go'),
  rust: require('../hooks/checks/lang/rust'),
  jvm: require('../hooks/checks/lang/jvm'),
  dotnet: require('../hooks/checks/lang/dotnet'),
};

function findings(pack, src, relPath) {
  return packs[pack].check(src, { relPath, config: { thresholds: DEFAULTS.thresholds } });
}

function paramFinding(pack, src, relPath) {
  return findings(pack, src, relPath).find((f) => f.id === 'obvious/too-many-params');
}

test('a wrapped six-parameter function is measured, and reported at its first line', () => {
  const src = [
    'export async function processUserRecords(',  // 1
    '  records: UserRecord[],',
    '  options: ProcessOptions,',
    '  logger: Logger,',
    '  clock: Clock,',
    '  retries: number,',
    '  tag: string,',
    '): Promise<Result> {',                       // 8
    '  return records.length;',
    '}',
  ].join('\n');

  const found = paramFinding('ts', src, 'a.ts');
  assert.ok(found, 'a wrapped signature was not measured at all');
  assert.strictEqual(found.message, '6 parameters (limit 4)');
  assert.strictEqual(found.line, 1, 'reported at the brace instead of the signature');
});

// One five-parameter function per shape a formatter actually produces, keyed
// by pack, path and a name for the failure message.
const WRAPPED_SHAPES = [
    ['ts', 'a.ts', 'arrow function assigned to a const', [
      'const buildReport = (',
      '  rows: Row[], header: string, footer: string, width: number, height: number,',
      ') => {',
      '  return rows.length;',
      '};',
    ]],
    ['ts', 'a.ts', 'object-literal method', [
      'const api = {',
      '  fetchEverything(',
      '    url: string, token: string, timeout: number, retries: number, signal: Signal,',
      '  ) {',
      '    return url;',
      '  },',
      '};',
    ]],
    ['jvm', 'X.java', 'annotation above, throws clause after', [
      'class X {',
      '    @Override',
      '    public Result process(',
      '            List<Record> records,',
      '            Options options,',
      '            Logger logger,',
      '            Clock clock,',
      '            int retries) throws IOException {',
      '        return null;',
      '    }',
      '}',
    ]],
    ['dotnet', 'X.cs', 'attribute above, brace on its own line', [
      'class X',
      '{',
      '    [Obsolete]',
      '    public async Task<Result> Process(',
      '        List<Record> records,',
      '        Options options,',
      '        ILogger logger,',
      '        IClock clock,',
      '        int retries)',
      '    {',
      '        return null;',
      '    }',
      '}',
    ]],
    ['go', 'a.go', 'multi-line params with named returns', [
      'func process(',
      '\trecords []Record,',
      '\toptions Options,',
      '\tlogger Logger,',
      '\tclock Clock,',
      '\tretries int,',
      ') (out int, err error) {',
      '\treturn len(records), nil',
      '}',
    ]],
    ['go', 'a.go', 'method with a receiver', [
      'func (s *Store) save(',
      '\tctx context.Context,',
      '\tid string,',
      '\tbody []byte,',
      '\ttags []string,',
      '\tforce bool,',
      ') error {',
      '\treturn nil',
      '}',
    ]],
    ['rust', 'a.rs', 'where clause on its own line', [
      'fn process<T>(',
      '    records: Vec<T>,',
      '    options: Options,',
      '    logger: Logger,',
      '    clock: Clock,',
      '    retries: usize,',
      ') -> usize',
      'where',
      '    T: Clone,',
      '{',
      '    records.len()',
      '}',
    ]],
];

test('wrapped signatures are measured in every shape the packs meet', () => {
  for (const [pack, relPath, shape, lines] of WRAPPED_SHAPES) {
    const found = paramFinding(pack, lines.join('\n'), relPath);
    assert.ok(found, `not measured: ${shape}`);
    assert.strictEqual(found.message, '5 parameters (limit 4)', shape);
    assert.strictEqual(found.line, lines.findIndex((l) => l.includes('(')) + 1, shape);
  }
});

// A wrapped signature carries a trailing comma under every formatter that
// wraps it, and counting that comma as a parameter would report one too many —
// turning a five-parameter function into a six-parameter finding.
test('a trailing comma is not a parameter', () => {
  assert.strictEqual(countParams('(a, b, c,)'), 3);
  assert.strictEqual(countParams('(a, b, c, )'), 3);
  assert.strictEqual(countParams('(,)'), 0);
});

// --- parameter lists longer than any span ceiling --------------------------
//
// The packs used to capture the parameter list as a span bounded to 500
// characters, so a signature whose parameters ran past that matched no pattern
// at all: a 60-parameter function on one line produced no finding while a
// 6-parameter one did. The failure was silent and inverted — the worse the
// code, the less likely it was reported — and it cost the whole block, not
// just the parameter count, since an unmatched signature is not measured for
// length or complexity either.
//
// One five-, one sixty- and one four-parameter function per pack, in that
// pack's own syntax, all on a single line.
const NAMES = (count) => Array.from({ length: count }, (unused, i) => `p${i}`);
const ONE_LINE = [
  ['ts', 'a.ts', (n) => `function wide(${NAMES(n).map((p) => `${p}: string`).join(', ')}) {\n  return 1;\n}`],
  ['py', 'a.py', (n) => `def wide(${NAMES(n).join(', ')}):\n    return 1`],
  ['go', 'a.go', (n) => `func wide(${NAMES(n).map((p) => `${p} string`).join(', ')}) error {\n\treturn nil\n}`],
  ['rust', 'a.rs', (n) => `fn wide(${NAMES(n).map((p) => `${p}: String`).join(', ')}) -> usize {\n    1\n}`],
  ['jvm', 'X.java', (n) => `class X {\n    public int wide(${NAMES(n).map((p) => `String ${p}`).join(', ')}) {\n        return 1;\n    }\n}`],
  ['dotnet', 'X.cs', (n) => `class X {\n    public int Wide(${NAMES(n).map((p) => `string ${p}`).join(', ')}) {\n        return 1;\n    }\n}`],
];

test('a parameter list is counted however long it is, in every pack', () => {
  for (const [pack, relPath, source] of ONE_LINE) {
    const wide = paramFinding(pack, source(60), relPath);
    assert.ok(wide, `${pack}: a 60-parameter function on one line was not measured at all`);
    assert.strictEqual(wide.message, '60 parameters (limit 4)', pack);

    const six = paramFinding(pack, source(6), relPath);
    assert.ok(six, `${pack}: a 6-parameter function stopped being reported`);
    assert.strictEqual(six.message, '6 parameters (limit 4)', pack);

    assert.strictEqual(paramFinding(pack, source(4), relPath), undefined,
      `${pack}: a 4-parameter function is inside the limit`);
  }
});

// The other two shape rules ride on the same match, so the ceiling hid them
// too: a long-signature function was measured for neither length nor
// complexity.
test('a function past the old ceiling is measured for length and complexity too', () => {
  const params = NAMES(60).map((p) => `${p}: string`).join(', ');
  const body = Array.from({ length: 60 }, (unused, i) => `  if (p${i}) { return p${i}; }`);
  const ids = findings('ts', [`function wide(${params}) {`, ...body, '}'].join('\n'), 'a.ts')
    .map((f) => f.id);
  assert.ok(ids.includes('obvious/too-many-params'));
  assert.ok(ids.includes('obvious/function-too-long'));
  assert.ok(ids.includes('obvious/complexity'));
});

// Commas that do not separate parameters: inside a default value, inside a
// subscripted annotation, inside nested generics. Counting them would report a
// three-parameter function as five and put a finding on correct code.
test('commas inside defaults, annotations and generics are not separators', () => {
  assert.strictEqual(countParams('(a, b=(1, 2), c: Dict[str, int])'), 3);
  assert.strictEqual(countParams('(a: Map<string, Map<string, number>>, b: number)'), 2);
  assert.strictEqual(countParams('(a = [1, 2, 3], b = { x: 1, y: 2 })'), 2);

  const src = 'def f(a, b=(1, 2), c: Dict[str, int]):\n    return a\n';
  const found = findings('py', src, 'a.py').find((f) => f.id === 'obvious/too-many-params');
  assert.strictEqual(found, undefined, 'three parameters were counted as more than four');

  // The same, past the old 500-character ceiling: the nesting has to be read
  // over a list long enough that the ceiling used to discard the signature.
  const padding = NAMES(60).map((p) => `${p}: Map<string, number>`);
  const wide = `function f(${['a', 'b = [1, 2]', ...padding].join(', ')}) {\n  return a;\n}`;
  assert.strictEqual(paramFinding('ts', wide, 'a.ts').message, '62 parameters (limit 4)');
});

// The span ceiling existed because an unbounded `[^)]*` capture is retried
// from every signature start and, with no `)` ahead, runs to end of file each
// time — quadratic, and once 75 seconds on a 400KB line. Counting commas at
// bracket depth zero in one pass replaces it, and must not bring that back.
//
// Expressed as the ratio between two input sizes rather than as an absolute
// millisecond bound: an absolute bound flakes under load on a loaded machine,
// while quadratic scaling shows up as a ratio no amount of load can produce.
// Quadrupling the input costs ~4x when linear and ~16x when quadratic; 8x
// leaves a wide margin either way.
test('parameter counting stays linear in line length', () => {
  const shapes = {
    'balanced signatures': 'function f(a,b){if(a&&b){return a+b}else{return 0}}',
    // No `)` anywhere: the shape on which a greedy unbounded capture scans to
    // end of file from every one of its starts.
    'no closing paren': 'function f(a,b,',
    // Every start nested inside the last, so each parameter list spans the
    // rest of the file.
    'nested parameter lists': 'function f(',
  };

  for (const [shape, unit] of Object.entries(shapes)) {
    const cost = (kb) => {
      const line = unit.repeat(Math.ceil((kb * 1024) / unit.length));
      return bestOf(3, () => packs.ts.check(line, {
        relPath: 'a.ts', config: { thresholds: DEFAULTS.thresholds },
      }));
    };
    const small = Math.max(1, cost(100));
    const large = cost(400);
    assert.ok(large / small < 8,
      `${shape}: 4x the input cost ${(large / small).toFixed(1)}x the time (${small}ms → ${large}ms)`);
  }
});

// Python's blocks come from indentation, and a wrapped `def` already starts its
// block on the `def` line: the continuation lines are indented past it and the
// body closes it as usual. So analyzeIndent has no equivalent gap to close.
// (Counting the wrapped def's parameters is py.js's DEF_LINE, not shape.js.)
test('a wrapped def opens its block at the def line', () => {
  const src = [
    'def process(',
    '    records,',
    '    options,',
    '):',
    '    if records:',
    '        return 1',
    '    return 0',
  ].join('\n');
  const { blocks } = analyzeIndent(src, { tabWidth: 4 });
  assert.ok(blocks.some((b) => b.startLine === 1 && b.endLine === 7));
});

// The lookback must not turn every brace into a function. An `else` brace is
// preceded by an `if` whose parameter list closed long before it, and the
// signature has to be the one this very brace opens.
//
// The `if` itself is no longer measured either — a control-flow head is not a
// signature, see the keyword sweep at the foot of this file — so the only
// function here is `outer`. This used to assert [1, 2]: the `if` block was
// measured as a function of its own, which is the duplicate-report defect.
test('the lookback does not attribute an else block to the if above it', () => {
  const src = [
    'function outer(a) {',
    '  if (a) {',
    '    go();',
    '  } else {',
    '    stop();',
    '  }',
    '}',
  ].join('\n');
  const re = { head: /(?:function\s+\w*|(?<!\w)\w+\s*)\(/g, tail: /\s*\{/ };
  const { blocks } = analyzeBraces(src);
  const measured = measureFunctions(
    src.split('\n'), blocks, signaturesFrom(stripNoise(src), re));
  assert.deepStrictEqual(measured.map((b) => b.startLine).sort(), [1]);
});

// The lookback used to be bounded at ten lines, which is the same inverted
// failure the 500-character parameter span had: one parameter per line is what
// every formatter produces, so the functions wrapped furthest are the ones with
// the most parameters — exactly what obvious/too-many-params exists to report.
// Nothing about the wrap is bounded any more: the parameter list's `(` is
// matched to its `)` by the one paramSpans pass over the file, however many
// lines apart they are.
const wrapped = (count) => [
  'function wide(',
  ...Array.from({ length: count }, (unused, i) => `  p${i},`),
  ') {',
  '  return 1;',
  '}',
].join('\n');

test('a signature wrapped over 25 lines is measured', () => {
  for (const count of [8, 25, 40, 200]) {
    const found = paramFinding('ts', wrapped(count), 'a.ts');
    assert.ok(found, `a ${count}-line wrap was not measured at all`);
    assert.strictEqual(found.message, `${count} parameters (limit 4)`);
    assert.strictEqual(found.line, 1, 'reported at the brace instead of the signature');
  }
});

// The other two shape rules ride on the same attribution, and the length is
// counted from the signature's first line, not from its brace.
test('a signature wrapped past the old bound is measured for length too', () => {
  const long = findings('ts', wrapped(60), 'a.ts')
    .find((f) => f.id === 'obvious/function-too-long');
  assert.ok(long, 'a wrapped function was not measured for length');
  assert.strictEqual(long.line, 1, 'length counted from the brace, not the signature');
  assert.strictEqual(long.message, 'function is 64 lines (limit 40)');
});

// Every pack that keys a function off its block-opening brace, on a wrap far
// past the old ten-line bound: the two whose head is scanned over the whole
// file (ts, go, rust) and the two whose head is anchored to the start of a line
// and so must be met line by line (jvm, dotnet).
const DEEP_WRAPS = [
  ['ts', 'a.ts', (n) => ['function wide(',
    ...Array.from({ length: n }, (unused, i) => `  p${i}: string,`), ') {', '  return 1;', '}']],
  ['go', 'a.go', (n) => ['func wide(',
    ...Array.from({ length: n }, (unused, i) => `\tp${i} string,`), ') error {', '\treturn nil', '}']],
  ['rust', 'a.rs', (n) => ['fn wide(',
    ...Array.from({ length: n }, (unused, i) => `    p${i}: String,`), ') -> usize', 'where', '    T: Clone,', '{', '    1', '}']],
  ['jvm', 'X.java', (n) => ['class X {', '    public int wide(',
    ...Array.from({ length: n }, (unused, i) => `            String p${i},`),
    '            int last) throws IOException {', '        return 1;', '    }', '}']],
  ['dotnet', 'X.cs', (n) => ['class X', '{', '    public int Wide(',
    ...Array.from({ length: n }, (unused, i) => `        string p${i},`),
    '        int last)', '    {', '        return 1;', '    }', '}']],
];

test('a wrap far past the old bound is measured in every brace pack', () => {
  for (const [pack, relPath, shape] of DEEP_WRAPS) {
    const lines = shape(25);
    const found = paramFinding(pack, lines.join('\n'), relPath);
    assert.ok(found, `${pack}: a 25-line wrap was not measured at all`);
    // jvm and dotnet carry one more parameter on the closing line.
    const expected = pack === 'jvm' || pack === 'dotnet' ? 26 : 25;
    assert.strictEqual(found.message, `${expected} parameters (limit 4)`, pack);
    assert.strictEqual(found.line, lines.findIndex((l) => l.includes('(')) + 1, pack);
  }
});

// Removing the bound must not buy the quadratic scan back. Expressed as a ratio
// between two input sizes rather than an absolute millisecond bound, for the
// reason the parameter-counting guard above gives: load moves an absolute
// bound, and cannot manufacture a ratio. The shapes are the ones the
// attribution pass actually walks — a file of blocks that are not functions,
// where every candidate has to be rejected, and a file of nothing but wrapped
// signatures, where every candidate is accepted.
test('signature attribution stays linear in line length and in signature count', () => {
  const re = { head: /function\s+\w*\(/g, tail: /\s*\{/ };
  const rawShape = (src) => {
    const lines = src.split('\n');
    const { blocks } = analyzeBraces(src);
    measureFunctions(lines, blocks, signaturesFrom(stripNoise(src), re));
  };

  const oneLine = (kb) => {
    const unit = 'if(a&&b){return a+b}else{return 0}';
    const src = 'const x = 1;\n' + unit.repeat(Math.ceil((kb * 1024) / unit.length));
    return Math.max(1, bestOf(3, () => rawShape(src)));
  };
  const ratio = oneLine(400) / oneLine(100);
  assert.ok(ratio < 8, `4x the line length cost ${ratio.toFixed(1)}x the time`);

  const manyWraps = (count) => {
    const src = Array.from({ length: count }, (unused, i) =>
      `function w${i}(\n  a,\n  b,\n  c,\n) {\n  return a;\n}`).join('\n');
    return Math.max(1, bestOf(3, () => rawShape(src)));
  };
  const wrapRatio = manyWraps(4000) / manyWraps(1000);
  assert.ok(wrapRatio < 8, `4x the signatures cost ${wrapRatio.toFixed(1)}x the time`);
});

// --- mixed tab/space indentation -------------------------------------------
//
// The tab-width conversion buckets an indent by width, so a two-space level and
// a tab level land in the same bucket and two real levels report as one. Depth
// under-reported is a nesting violation that passes.
test('mixed tabs and spaces do not collapse two indentation levels into one', () => {
  const src = [
    'def f():',
    '  if a:',
    '\tif b:',
    '\t  if c:',
    '\t\tgo()',
  ].join('\n');
  assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 4);
});

test('consistent indentation is read exactly as before', () => {
  const spaces = 'def f():\n    if a:\n        if b:\n            go()';
  const tabs = 'def f():\n\tif a:\n\t\tif b:\n\t\t\tgo()';
  assert.strictEqual(analyzeIndent(spaces, { tabWidth: 4 }).maxDepth, 3);
  assert.strictEqual(analyzeIndent(tabs, { tabWidth: 4 }).maxDepth, 3);
});

// --- indentation width read from the file ----------------------------------
//
// Depth was `column / tabWidth` with tabWidth fixed at 4, so a file indented
// two spaces per level reported half its depth: seven real levels measured
// three and passed a limit of three. Two-space Python is ordinary — black is
// not universal, and plenty of code predates it — and under-reported depth is
// the wrong direction of error, a nesting violation that passes.
//
// tabWidth keeps its own meaning, what a tab character is worth, so the packs
// need no change: py.js still passes 4 and a tab still expands to four columns.
// What one *level* is worth is read from the file instead.
const nest = (unit, levels) => [
  'def f():',
  ...Array.from({ length: levels }, (unused, i) => `${unit.repeat(i + 1)}if a${i}:`),
  `${unit.repeat(levels + 1)}go()`,
].join('\n');

test('a 2-space and a 4-space file with identical structure report one depth', () => {
  for (const levels of [1, 2, 3, 5, 7]) {
    const four = analyzeIndent(nest('    ', levels), { tabWidth: 4 }).maxDepth;
    const two = analyzeIndent(nest('  ', levels), { tabWidth: 4 }).maxDepth;
    const tabs = analyzeIndent(nest('\t', levels), { tabWidth: 4 }).maxDepth;
    assert.strictEqual(four, levels + 1, `4-space, ${levels} levels`);
    assert.strictEqual(two, four, `2-space read as ${two}, 4-space as ${four}`);
    assert.strictEqual(tabs, four, `tabs read as ${tabs}, 4-space as ${four}`);
  }
});

// The three files an inference can trip over: nothing to infer from, one
// sample to infer from, and a first sample that is not the step.
test('indent width inference survives the files with nothing to infer from', () => {
  assert.strictEqual(analyzeIndent('x = 1\ny = 2\n', { tabWidth: 4 }).maxDepth, 0);
  assert.strictEqual(analyzeIndent('', { tabWidth: 4 }).maxDepth, 0);
  assert.strictEqual(analyzeIndent('def f():\n  return 1\n', { tabWidth: 4 }).maxDepth, 1);
  assert.strictEqual(analyzeIndent('def f():\n        return 1\n', { tabWidth: 4 }).maxDepth, 1);

  // First indent unusual — a hanging block, then the file's real step. The
  // step is what the file does most, not what it did first.
  const src = [
    'if a:',
    '        odd()',
    'def f():',
    '    if b:',
    '        if c:',
    '            go()',
  ].join('\n');
  assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 3);
});

// Inference adds a pass over the file, so it has to stay a pass: a ratio
// between two input sizes, for the reason the guards above give.
test('reading the indent width off the file stays linear', () => {
  const body = ['def f(a):', '    if a:', '        for i in a:', '            go(i)', ''].join('\n');
  const cost = (kb) => Math.max(1, bestOf(3, () => analyzeIndent(
    body.repeat(Math.ceil((kb * 1024) / body.length)), { tabWidth: 4 })));
  const ratio = cost(400) / cost(100);
  assert.ok(ratio < 8, `4x the file cost ${ratio.toFixed(1)}x the time`);
});

// End to end through the pack that owns the caller, which passes tabWidth 4 and
// is not this change's to edit: a genuinely over-nested two-space function has
// to be reported like its four-space twin.
test('an over-nested 2-space Python function is reported', () => {
  const deep = (unit) => [
    'def f():',
    `${unit}if a:`,
    `${unit.repeat(2)}if b:`,
    `${unit.repeat(3)}if c:`,
    `${unit.repeat(4)}go()`,
  ].join('\n');
  const depthFinding = (src) => findings('py', src, 'a.py')
    .find((f) => f.id === 'obvious/nesting-depth');

  assert.strictEqual(depthFinding(deep('    ')).message, 'nesting depth 4 (limit 3)');
  assert.strictEqual(depthFinding(deep('  ')).message, 'nesting depth 4 (limit 3)');
});

// Against the same byte count of ordinary code rather than against a
// millisecond number, for the reason given at the head of this file: the
// millisecond number bounded the runner. Backtracking is superlinear in the run
// it backtracks over, so it cannot hide inside a multiple of the linear scan —
// the pathological line measured 1.0-1.7x the benign one at 0 and 48 competing
// spinners, and it is in fact usually the *cheaper* of the two, having no
// braces and no branches in it.
test('does not catastrophically backtrack on a long line', () => {
  const long = 'function a() { ' + 'x'.repeat(20000) + ' }';
  const unit = 'function f(a,b){if(a&&b){return a+b}else{return 0}}';
  const benign = unit.repeat(Math.ceil(long.length / unit.length)).slice(0, long.length);
  const cost = (src) => bestOf(3, () => {
    analyzeBraces(src);
    estimateComplexity(src);
  });

  const ms = cost(long);
  const budget = cost(benign) * PERF_MULTIPLE;
  assert.ok(ms < budget, `took ${ms}ms on pathological input (budget ${budget}ms)`);
});

// A destructuring pattern is data, not a block. LITERAL_BRACE only catches
// literals in expression position (right of `=`), so a binding pattern — which
// sits left of it — used to count as a level of nesting and invented rung-3
// findings on flat code. The guards below must hold in both directions: real
// nesting still counts, data still does not.
test('binding patterns are data, not nesting', () => {
  assert.strictEqual(analyzeBraces('function f() {\n  const { a } = obj;\n}\n').maxDepth, 1);
  assert.strictEqual(analyzeBraces('import { x } from "y";\nfunction f() {\n  go();\n}\n').maxDepth, 1);
  assert.strictEqual(analyzeBraces('function f() {\n  let { a, b } = obj;\n}\n').maxDepth, 1);

  // Still counted: real blocks, object literals, and switch cases.
  assert.strictEqual(analyzeBraces('function f() {\n  if (a) {\n    go();\n  }\n}\n').maxDepth, 2);
  assert.strictEqual(analyzeBraces('function f() {\n  const a = { b: 1 };\n}\n').maxDepth, 1);
  assert.strictEqual(
    analyzeBraces('function f(){\nswitch(x){\ncase 1: {\nif(a){\ngo();\n}}}}\n').maxDepth, 4);
});

// --- `#` opens a comment only where the language says so --------------------
//
// stripNoise blanked from any `#` to end of line whatever the language. In
// JS/TS `#` opens a private class member, so `#wide(a, b, c, d, e, f) {` lost
// its parameter list AND its block-opening brace: the method got no block, no
// signature and so no finding at all, while the identical public method
// reported six parameters. Hiding a violation is the worse direction of error,
// and private members are ordinary modern JS.
test('a private class method is measured like a public one', () => {
  const re = { head: /(?:async\s+)?(?<!\w)\w+\s*\(/g, tail: /\s*\{/ };
  const measure = (src) => {
    const { blocks } = analyzeBraces(src);
    return measureFunctions(src.split('\n'), blocks, signaturesFrom(stripNoise(src), re));
  };
  const src = (name) => [
    'class Cache {',
    `  ${name}(a, b, c, d, e, f) {`,
    '    return a;',
    '  }',
    '}',
  ].join('\n');

  const priv = measure(src('#wide')).find((b) => b.startLine === 2);
  assert.ok(priv, 'the private method was measured as no block at all');
  assert.strictEqual(priv.params, 6);
  assert.strictEqual(priv.params, measure(src('wide')).find((b) => b.startLine === 2).params);
});

test('stripNoise blanks # comments for the languages that have them', () => {
  // Python, Ruby, shell, TOML and YAML: there `#` genuinely opens a comment.
  assert.strictEqual(stripNoise('x = 1  # note\n', 'py'), 'x = 1        \n');
  // JS/TS: it does not.
  assert.strictEqual(stripNoise('this.#n = go(1);'), 'this.#n = go(1);');
});

// --- a control-flow head is not a function signature -------------------------
//
// `switch (kind) {`, `if (ok) {`, `while (more) {` — every control-flow keyword
// that takes a parenthesised head has exactly the shape the packs' signature
// heads look for, `<name>(args) {`. A function whose body opened with one was
// measured twice: once at its real signature line, once at the keyword on the
// line below, so obvious/complexity reported the same function at two lines.
// Found by an adversarial false-positive hunt on a `switch`; every sibling
// keyword had it too.
const CONTROL_BODIES = {
  switch: ['  switch (kind) {', "    case 'a': return 1;", '    default: return 0;', '  }'],
  if: ['  if (kind) {', '    return 1;', '  }'],
  while: ['  while (kind) {', '    kind = next(kind);', '  }'],
  for: ['  for (let i = 0; i < 3; i += 1) {', '    go(i);', '  }'],
  catch: ['  try {', '    go();', '  } catch (err) {', '    report(err);', '  }'],
  'do…while': ['  do {', '    kind = next(kind);', '  } while (kind);'],
};

// The ts pack's own pair, because an arrow's `=>` sits between the parameter
// list and the brace and the simplified pattern the older tests use cannot
// reach past it.
const measureTs = (src) => {
  const re = {
    head: /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?(?<!\w)(?:\w+\s*)?)\(/g,
    tail: /\s*(?::[^{=]{1,500})?(?:=>)?\s*\{/,
  };
  const { blocks } = analyzeBraces(src);
  return measureFunctions(src.split('\n'), blocks, signaturesFrom(stripNoise(src), re));
};

test('a body opening with a control-flow keyword is measured once, at its own line', () => {
  for (const [keyword, body] of Object.entries(CONTROL_BODIES)) {
    const src = ['function route(kind) {', ...body, '  return 0;', '}'].join('\n');
    assert.deepStrictEqual(measureTs(src).map((b) => b.startLine), [1],
      `${keyword}: measured as a function of its own`);
  }
});

// The same defect end to end, on the input the hunt reported: one function, one
// finding, at line 1.
test('a switch-bodied function reports complexity once, at the signature', () => {
  const cases = Array.from({ length: 14 }, (unused, i) => `    case '${i}': return ${i};`);
  const src = ['function route(kind) {', '  switch (kind) {', ...cases,
    '    default: return 0;', '  }', '}'].join('\n');
  const found = findings('ts', src, 'a.ts').filter((f) => f.id === 'obvious/complexity');
  assert.strictEqual(found.length, 1, `reported at lines ${found.map((f) => f.line)}`);
  assert.strictEqual(found[0].line, 1);
});

// `else if (…) {` is the same shape to the line-anchored heads, which want two
// identifiers before the parens and find `else` and `if`.
test('an else-if branch is not measured as a method', () => {
  const jvm = ['class X {', '    public int route(int k) {', '        if (k == 0) {',
    '            return 0;', '        }', '        else if (k == 1) {', '            return 1;',
    '        }', '        return 2;', '    }', '}'].join('\n');
  const { blocks } = analyzeBraces(jvm);
  const measured = measureFunctions(jvm.split('\n'), blocks,
    signaturesFrom(stripNoise(jvm), {
      head: /^\s*(?:(?:public|private|protected|static|final)\s+)*[\w<>[\],.]+(?:\s*<[\w\s<>[\],.]*>)?\s+\w+\s*\(/,
      tail: /\s*(?:throws\s+[\w,.\s]+)?\{\s*$/,
    }));
  assert.deepStrictEqual(measured.map((b) => b.startLine), [2]);
});

// Coverage the sweep must not take with it: a keyword after `.` is a method,
// and the callback it takes is a function like any other.
test('a .catch callback is still measured', () => {
  const src = ['run().catch((err, ctx, a, b, c, d) => {', '  report(err);', '});'].join('\n');
  const measured = measureTs(src);
  assert.strictEqual(measured.length, 1);
  assert.strictEqual(measured[0].params, 6);
});

// A genuinely over-threshold function still reports, once, at its own line.
test('a genuinely complex function still reports once at its signature', () => {
  const src = ['function tangle(a, b) {',
    ...Array.from({ length: 12 }, (unused, i) => `  if (a === ${i} && b) return ${i};`),
    '  return 0;', '}'].join('\n');
  const found = findings('ts', src, 'a.ts').filter((f) => f.id === 'obvious/complexity');
  assert.strictEqual(found.length, 1, `reported at lines ${found.map((f) => f.line)}`);
  assert.strictEqual(found[0].line, 1);
});

// --- one measurement per function, structurally ------------------------------
//
// The keyword sweep removes one way for a function to be measured twice; it
// does not remove the last. A block is attributed to a signature by the line
// its brace opens on, and more than one block can open on that line: `function
// outer(a) { if (a) {` gives the signature two blocks, and both reported. Any
// future head that matches something extra on a signature's own line does the
// same. Two measurements of one function must not be able to reach output at
// all, so measureFunctions keeps one span per signature line — the widest,
// which is the function's own block rather than something nested inside it.
test('two blocks opening on one signature line are measured once', () => {
  const body = Array.from({ length: 45 }, (unused, i) => `    step${i}();`);
  const src = ['function outer(a) { if (a) {', ...body, '  }', '}'].join('\n');

  const measured = measureTs(src);
  assert.strictEqual(measured.length, 1, 'the same function was measured twice');
  assert.strictEqual(measured[0].startLine, 1);
  assert.strictEqual(measured[0].length, 48, 'kept the inner block over the function');

  const found = findings('ts', src, 'a.ts').filter((f) => f.id === 'obvious/function-too-long');
  assert.strictEqual(found.length, 1, `reported ${found.length} times for one function`);
});

// Removing the `if` signature is not enough on its own: a bare parenthesised
// expression is a head to the ts pattern, and that pattern's tail — a return
// type of up to 500 characters, newlines included — reaches down to the brace
// of an `if` four lines below it. The `if`'s own signature used to mask that;
// with the sweep above it surfaces, and the finding moves from the `if` to an
// expression that is not a function at all. A tail is a return type, a
// `throws`, a `where` clause — never a statement — so a control-flow keyword
// inside one says the brace belongs to the keyword. Found in the differential
// against a 486-file TypeScript tree, not by unit test.
test('a tail does not reach past a control-flow keyword to its brace', () => {
  const src = ['export function render(obj: Payload) {',
    '  const message = typeof obj === "object"',
    '    ? (obj.a || obj.b || obj.c || obj.d || JSON.stringify(obj, null, 2))',
    '    : String(obj)',
    '',
    '  if (message.startsWith("task.")) {',
    '    return 1;',
    '  }',
    '  return 0;',
    '}'].join('\n');
  assert.deepStrictEqual(measureTs(src).map((b) => b.startLine), [1]);
});

// --- a body on the signature's own line is still a body ----------------------
//
// The other half of the same assumption: the recogniser expected the block to
// open at the end of the signature's line and the body to follow below it, so a
// declaration that opened and closed on one line fell out of the scan entirely.
// Java's and C#'s tails are `$`-anchored, which is exactly that assumption
// written down, and `public int size() { return n; }` matched no tail, got no
// signature, and was measured for nothing at all — length, nesting, parameters
// and complexity alike. braceLineAfter cuts its window at the brace now instead
// of at the line end.
const SAME_LINE = [
  ['jvm', 'X.java', (n) => ['class X {',
    `    public int wide(${Array.from({ length: n }, (u, i) => `int p${i}`).join(', ')}) { return 1; }`,
    '}']],
  ['dotnet', 'X.cs', (n) => ['class X {',
    `    public int Wide(${Array.from({ length: n }, (u, i) => `int p${i}`).join(', ')}) { return 1; }`,
    '}']],
  ['ts', 'a.ts', (n) => [
    `function wide(${Array.from({ length: n }, (u, i) => `p${i}`).join(', ')}) { return 1; }`]],
  ['go', 'a.go', (n) => [
    `func wide(${Array.from({ length: n }, (u, i) => `p${i} int`).join(', ')}) int { return 1 }`]],
  ['rust', 'a.rs', (n) => [
    `fn wide(${Array.from({ length: n }, (u, i) => `p${i}: usize`).join(', ')}) -> usize { 1 }`]],
];

test('a method whose body opens on its own line is measured', () => {
  for (const [pack, relPath, shape] of SAME_LINE) {
    const lines = shape(6);
    const found = paramFinding(pack, lines.join('\n'), relPath);
    assert.ok(found, `${pack}: a same-line body was not measured at all`);
    assert.strictEqual(found.message, '6 parameters (limit 4)', pack);
    assert.strictEqual(found.line, lines.findIndex((l) => l.includes('(')) + 1, pack);
  }
});

test('an empty same-line body is measured too', () => {
  const src = ['class X {', '    public void noop(int a, int b, int c, int d, int e, int f) {}', '}'];
  const found = paramFinding('jvm', src.join('\n'), 'X.java');
  assert.ok(found, 'an empty same-line body was not measured');
  assert.strictEqual(found.line, 2);
});

// An expression body has no brace at all, so analyzeBraces hands it nothing:
// measured directly, a C# method with 300 parameters reported nothing.
test('an expression-bodied method is measured for its parameters', () => {
  const params = Array.from({ length: 300 }, (unused, i) => `int p${i}`).join(', ');
  const src = ['class X', '{', `    public int Wide(${params}) => n;`, '}'].join('\n');
  const found = paramFinding('dotnet', src, 'X.cs');
  assert.ok(found, 'an expression-bodied method was not measured at all');
  assert.strictEqual(found.message, '300 parameters (limit 4)');
  assert.strictEqual(found.line, 3);
});

// The synthesised span must not turn every callback into a function. A `=>`
// after a parameter list is an arrow anywhere in an expression, and hanging a
// one-line function on one would put the whole line's complexity on it — the
// false positive this pass exists to remove. So only a declaration that starts
// its line and ends the statement on it counts.
test('an arrow inside an expression is not a declaration', () => {
  const src = ['const totals = rows.map((row) => row.a && row.b ? row.a : row.b)',
    '  .filter((row) => row.a && row.b && row.c && row.d && row.e && row.f && row.g);'].join('\n');
  assert.deepStrictEqual(measureTs(src), []);
  assert.deepStrictEqual(findings('ts', src, 'a.ts'), []);
});

// --- a function named after a keyword is still a function --------------------
//
// The keyword sweep above rejects a statement-position head whose word is a
// control-flow keyword. Applied as one global list to every pack it went one
// step too far: a function *named* after one of those words — `match(pattern,
// input)` in a parser, `lock(resource)` in a scheduler, `using(handle)`,
// `when(condition)` — was not measured for length, nesting, parameters or
// complexity either. Silent coverage loss, in five of the six packs.
//
// A declaration is told from a statement by what is in front of the word.
// `function`, `func`, `fn`, `def`, an access modifier, a return type — anything
// ahead of the name in the head means a declaration, so only a head that is the
// keyword and nothing else can be a statement. That alone clears every pack
// whose head demands a declaration keyword (Go's `func`, Rust's `fn`) or two
// tokens (Java's and C#'s `<type> <name>`), and leaves the bare-name heads —
// TypeScript's — to the second half: the word must actually be control flow in
// *that* language. `match` is a statement in Rust and a name in JavaScript;
// `with` is a statement in Python and a name in C#; `lock` is C#'s alone.
const KEYWORD_NAMED = [
  ['ts', 'a.ts', ['function match(a, b, c, d, e, f) {', '  return a + b + c + d + e + f;', '}']],
  ['py', 'a.py', ['def match(a, b, c, d, e, f):', '    return a']],
  ['go', 'a.go', ['func lock(a, b, c, d, e, f int) int {', '\treturn a', '}']],
  ['rust', 'a.rs', ['fn lock(a: i32, b: i32, c: i32, d: i32, e: i32, f: i32) -> i32 {',
    '    a', '}']],
  ['jvm', 'X.java', ['class X {',
    '    public int lock(int a, int b, int c, int d, int e, int f) {', '        return a;',
    '    }', '}']],
  ['dotnet', 'X.cs', ['class X {',
    '    public int when(int a, int b, int c, int d, int e, int f) {', '        return a;',
    '    }', '}']],
];

test('a function named after a control-flow keyword is measured', () => {
  for (const [pack, relPath, lines] of KEYWORD_NAMED) {
    const found = paramFinding(pack, lines.join('\n'), relPath);
    assert.ok(found, `${pack}: a keyword-named function was not measured at all`);
    assert.strictEqual(found.message, '6 parameters (limit 4)', pack);
    assert.strictEqual(found.line, lines.findIndex((l) => l.includes('(')) + 1, pack);
  }
});

// The same word in the other role. Each body holds a control-flow statement
// long enough to trip obvious/function-too-long if it were mistaken for a
// function: exactly one finding must come back, at the enclosing declaration.
const STATEMENT_BODIES = [
  ['ts', 'a.ts', 1, (body) => ['function route(kind) {', '  switch (kind) {',
    ...body, '  }', '}']],
  ['py', 'a.py', 1, (body) => ['def route(path):', '    with open(path) as handle:',
    ...body.map((l) => '    ' + l), '    return 0']],
  ['go', 'a.go', 1, (body) => ['func route(kind int) int {', '\tfor i := 0; i < 3; i++ {',
    ...body, '\t}', '\treturn 0', '}']],
  ['rust', 'a.rs', 1, (body) => ['fn route(kind: i32) -> i32 {', '    match kind {',
    ...body, '        _ => 0,', '    }', '}']],
  ['jvm', 'X.java', 2, (body) => ['class X {', '    public int route(int k) {',
    '        synchronized (this) {', ...body, '        }', '        return 0;', '    }', '}']],
  ['dotnet', 'X.cs', 2, (body) => ['class X {', '    public int Route(int k) {',
    '        lock (gate) {', ...body, '        }', '        return 0;', '    }', '}']],
];

test('a control-flow statement is not measured as a function', () => {
  const body = Array.from({ length: 45 }, (unused, i) => `        step${i}();`);
  for (const [pack, relPath, line, shape] of STATEMENT_BODIES) {
    const found = findings(pack, shape(body).join('\n'), relPath)
      .filter((f) => f.id === 'obvious/function-too-long');
    assert.strictEqual(found.length, 1,
      `${pack}: reported at lines ${found.map((f) => f.line)}`);
    assert.strictEqual(found[0].line, line, pack);
  }
});

// The bare-name head is TypeScript's alone, and the only one where the word
// carries no declaration in front of it: a class method named `match` looks
// exactly like `switch (…)` to the pattern. What separates them is the
// language, not the shape — JavaScript has no `match` statement — so the sweep
// only ever rejects words that are reserved control flow in JS/TS.
test('a class method named after a non-keyword is measured', () => {
  for (const name of ['match', 'when', 'lock', 'using', 'unless', 'until', 'foreach']) {
    const src = ['class Parser {',
      `  ${name}(a, b, c, d, e, f) {`, '    return a;', '  }', '}'].join('\n');
    const found = paramFinding('ts', src, 'a.ts');
    assert.ok(found, `${name}: a method named after a non-keyword was not measured`);
    assert.strictEqual(found.line, 2, name);
  }
});

// Over the thresholds, at the right line, once — the whole point of measuring
// it at all.
test('an over-threshold function named after a keyword reports once', () => {
  const src = ['function match(kind, input) {',
    ...Array.from({ length: 12 }, (unused, i) => `  if (kind === ${i} && input) return ${i};`),
    '  return 0;', '}'].join('\n');
  const found = findings('ts', src, 'a.ts').filter((f) => f.id === 'obvious/complexity');
  assert.strictEqual(found.length, 1, `reported at lines ${found.map((f) => f.line)}`);
  assert.strictEqual(found[0].line, 1);
});

// --- depth is structural, not a column count -------------------------------
//
// Dividing a column by a single inferred unit answers "how many units wide is
// this indent", which is the nesting level only when every level in the file is
// exactly that unit wide. Two ways that goes wrong, both real:
//
//   too wide  — the unit was picked from a bounded candidate list (a step had
//               to be 8 columns or narrower), so a file indented 9, 10, 12 or
//               16 columns per level fell back to tabWidth and every real level
//               counted as two or four. Correct code, reported as over-nested.
//   two units — the unit is one number for the whole file, so a region indented
//               differently is measured against someone else's. Under-reported
//               depth is the worse half: a genuine violation, silently green.
//
// Counting enclosing indentation columns instead needs no unit at all: a line
// indented wider than the statement enclosing it is one level deeper, whatever
// the widths are.
const nested = (unit, levels) => [
  'def f():',
  ...Array.from({ length: levels }, (unused, i) => `${unit.repeat(i + 1)}if a${i}:`),
  `${unit.repeat(levels + 1)}go()`,
].join('\n');

test('an indent step wider than a tab stop measures its real depth', () => {
  for (const width of [9, 10, 12, 16]) {
    const src = nested(' '.repeat(width), 2);
    assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 3,
      `${width}-column steps`);
  }
});

test('a 10-column def with two levels of if raises no nesting finding', () => {
  const src = nested(' '.repeat(10), 2);
  const hit = findings('py', src, 'a.py').find((f) => f.id === 'obvious/nesting-depth');
  assert.strictEqual(hit, undefined, hit && hit.message);
});

// One file, two indent widths. The commonest step is 4 — twelve defs vote for
// it — so the two-space region was measured against it and lost half its depth.
const twoWidths = (levels) => [
  ...Array.from({ length: 12 }, (unused, i) => `def d${i}(x):\n    return ${i}`),
  nested('  ', levels),
].join('\n');

test('a region indented differently is measured against its own structure', () => {
  assert.strictEqual(analyzeIndent(twoWidths(5), { tabWidth: 4 }).maxDepth, 6);
  assert.strictEqual(analyzeIndent(twoWidths(6), { tabWidth: 4 }).maxDepth, 7);
});

test('the deep region of a mixed-width file is reported', () => {
  for (const [levels, depth] of [[5, 6], [6, 7]]) {
    const hit = findings('py', twoWidths(levels), 'a.py')
      .find((f) => f.id === 'obvious/nesting-depth');
    assert.ok(hit, `${levels} levels: no nesting finding at all`);
    assert.strictEqual(hit.message, `nesting depth ${depth} (limit 3)`);
  }
});

// The other direction of the same defect: a four-space region in a file whose
// commonest step is two used to be measured as twice as deep as it is.
test('the shallow region of a mixed-width file is not over-reported', () => {
  const src = [
    ...Array.from({ length: 12 }, (unused, i) => `def d${i}(x):\n  return ${i}`),
    nested('    ', 2),
  ].join('\n');
  assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 3);
});

// Alignment under an open bracket is not nesting, at any width — the wider the
// call, the further right the alignment lands.
test('continuation lines inside a call are not nesting', () => {
  const src = [
    'def f():',
    '    result = compute(alpha,',
    '                     beta,',
    '                     gamma)',
    '    return result',
  ].join('\n');
  assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 1);
});

test('tabs and mixed tabs and spaces still measure their real depth', () => {
  assert.strictEqual(
    analyzeIndent(nested('\t', 2), { tabWidth: 4 }).maxDepth, 3);
  const mixed = ['def f():', '  if a:', '\tif b:', '\t  if c:', '\t\tgo()'].join('\n');
  assert.strictEqual(analyzeIndent(mixed, { tabWidth: 4 }).maxDepth, 4);
});

// --- a member named after a statement is a declaration -----------------------
//
// The anchored refusal above reads a bare `<keyword> (…) {` as a statement,
// because a declaration always puts something in front of the name. A JS method
// shorthand puts nothing in front of it at all:
//
//   class Parser { with(a, b, c, d, e, f) { … } }
//   const p = { catch(a, b, c, d, e, f) { … } };
//
// so a method named after a JS statement was invisible to every shape rule —
// not length, not nesting, not parameters, not complexity. The line cannot
// decide it; only the enclosing brace can. A `{` that opens a class body or an
// object literal holds members, and a member is a declaration however it is
// named; a `{` that opens a block holds statements.
const STATEMENT_NAMES = ['with', 'catch', 'case', 'for', 'while', 'switch', 'if'];

const classMember = (name) => [
  'class Parser {',
  `  ${name}(a, b, c, d, e, f) {`,
  '    return a;',
  '  }',
  '}',
].join('\n');

const objectMember = (name) => [
  'const parser = {',
  `  ${name}(a, b, c, d, e, f) {`,
  '    return a;',
  '  },',
  '};',
].join('\n');

test('a class method named after a JS statement is measured, at its own line', () => {
  for (const name of STATEMENT_NAMES) {
    const found = paramFinding('ts', classMember(name), 'a.ts');
    assert.ok(found, `${name}: a six-parameter class method was not measured at all`);
    assert.strictEqual(found.line, 2, `${name}: reported at the wrong line`);
    assert.strictEqual(found.message, '6 parameters (limit 4)');
  }
});

test('an object-literal method named after a JS statement is measured', () => {
  for (const name of STATEMENT_NAMES) {
    const found = paramFinding('ts', objectMember(name), 'a.ts');
    assert.ok(found, `${name}: a six-parameter object method was not measured at all`);
    assert.strictEqual(found.line, 2, `${name}: reported at the wrong line`);
  }
});

// The other direction, which is the whole reason the refusal exists: the same
// words in statement position are still not signatures. A block is where
// statements live, and a `case` label opens a block whatever the colon before
// its brace suggests — switch-heavy code is where the 104 false positives came
// from.
test('the same words in statement position are still not measured', () => {
  for (const [keyword, body] of Object.entries(CONTROL_BODIES)) {
    const src = ['function route(kind) {', ...body, '  return 0;', '}'].join('\n');
    assert.deepStrictEqual(measureTs(src).map((b) => b.startLine), [1],
      `${keyword}: measured as a function of its own`);
  }
});

test('control flow inside a class method is still not measured', () => {
  const src = [
    'class Router {',
    '  route(kind) {',
    '    switch (kind) {',
    "      case 'a': {",
    '        if (kind) {',
    '          return 1;',
    '        }',
    '      }',
    '    }',
    '    return 0;',
    '  }',
    '}',
  ].join('\n');
  assert.deepStrictEqual(measureTs(src).map((b) => b.startLine), [2]);
});

// An over-threshold member of each kind reports, once, at its signature.
test('an over-threshold method named after a statement reports once', () => {
  const long = Array.from({ length: 45 }, (unused, i) => `    step${i}();`);
  const branches = Array.from({ length: 12 }, (unused, i) => `    if (a === ${i}) return ${i};`);
  const shapes = [
    ['obvious/function-too-long', ['class P {', '  with(a) {', ...long, '  }', '}']],
    ['obvious/too-many-params',
      ['class P {', '  with(a, b, c, d, e, f) {', '    return a;', '  }', '}']],
    ['obvious/complexity', ['class P {', '  with(a) {', ...branches, '    return 0;', '  }', '}']],
  ];
  for (const [id, lines] of shapes) {
    const found = findings('ts', lines.join('\n'), 'a.ts').filter((f) => f.id === id);
    assert.strictEqual(found.length, 1, `${id}: reported ${found.length} times`);
    assert.strictEqual(found[0].line, 2, `${id}: reported at line ${found[0].line}`);
  }
});

// --- raw identifiers ---------------------------------------------------------
//
// `r#match` is how Rust spells an identifier that collides with a keyword, and
// `@match` is C#'s. The `fn` and the `public int` in front of them make these
// declarations by construction — the packs' name patterns simply did not admit
// the escape character, so the declaration went unmatched and every shape rule
// skipped it.
const RAW_IDENTS = [
  ['rust', 'a.rs', (name) => [
    `fn ${name}(a: i32, b: i32, c: i32, d: i32, e: i32, f: i32) -> i32 {`,
    '    a',
    '}',
  ], 'r#match', 'matcher', 1],
  ['dotnet', 'X.cs', (name) => [
    'class X {',
    `    public int ${name}(int a, int b, int c, int d, int e, int f) {`,
    '        return a;',
    '    }',
    '}',
  ], '@match', 'Matcher', 2],
];

test('a raw identifier is measured like a plain one', () => {
  for (const [pack, path, src, raw, plain, line] of RAW_IDENTS) {
    const found = paramFinding(pack, src(raw).join('\n'), path);
    assert.ok(found, `${pack}: ${raw} was not measured at all`);
    assert.strictEqual(found.line, line, `${pack}: reported at the wrong line`);
    assert.strictEqual(found.message, paramFinding(pack, src(plain).join('\n'), path).message,
      `${pack}: ${raw} measured differently from ${plain}`);
  }
});

// Rust's raw strings start `r#"` too, and C#'s verbatim strings `@"`. Neither
// is an identifier, and neither may be read as one.
test('raw and verbatim string prefixes are not raw identifiers', () => {
  const rust = ['fn go() {', '    let p = r#"a{b"#;', '    p', '}'].join('\n');
  assert.deepStrictEqual(findings('rust', rust, 'a.rs').map((f) => f.id), []);
  const cs = ['class X {', '    public int Go() {', '        var p = @"c:\\a{b";',
    '        return 1;', '    }', '}'].join('\n');
  assert.deepStrictEqual(findings('dotnet', cs, 'X.cs').map((f) => f.id), []);
});
