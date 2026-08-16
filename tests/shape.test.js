// tests/shape.test.js
const test = require('node:test');
const assert = require('node:assert');
const {
  analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
  measureFunctions, signaturesFrom, stripNoise,
} = require('../hooks/checks/shape');
const { DEFAULTS } = require('../hooks/checks/config');

// Milliseconds for the fastest of three runs. The perf guards below all sit
// two orders of magnitude under their bound — the failure they exist to catch
// costs seconds — so the only realistic way for them to fail on a healthy tree
// is a single scheduler stall or a GC pause landing inside the measurement. A
// stall hits one run of three, not all three, so the best of three keeps the
// order-of-magnitude signal and drops the flake. It cannot help against
// sustained load that slows every run alike; nothing in-process can, short of
// asserting on a ratio between two input sizes, and these guards deliberately
// assert absolute cost against the hook's 2s whole-file budget.
function bestOf(runs, work) {
  let best = Infinity;
  for (let i = 0; i < runs; i += 1) {
    const started = Date.now();
    work();
    best = Math.min(best, Date.now() - started);
  }
  return best;
}

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

// A 2MB single-line minified file. Line numbering used to re-slice the whole
// source per match, so cost was quadratic: ~1.3s here, over the 2s whole-file
// hook budget on its own. Linear line indexing runs it in tens of ms; the
// bound is set an order of magnitude above that so a loaded CI machine does
// not flake, while a return to quadratic scaling still fails.
test('signaturesFrom stays linear on a huge single-line file', () => {
  const unit = 'function f(a,b){return a+b}';
  const src = unit.repeat(Math.ceil((2 * 1024 * 1024) / unit.length));
  const re = { head: /function\s+\w*\(/g, tail: /\s*\{/ };

  const ms = bestOf(3, () => signaturesFrom(stripNoise(src), re));
  assert.ok(ms < 500, `signaturesFrom scaled worse than linearly: ${ms}ms`);
});

// A 400KB minified line, the size at which the two used to cost 2.0s and 44.8s
// respectively — each on its own over the 2s whole-file hook budget. Both now
// run in single-digit milliseconds. The bound is 500ms, two orders of magnitude
// above that, so a loaded CI machine cannot flake it; a return to quadratic
// scaling costs tens of seconds and fails it by a wide margin.
test('analyzeBraces and measureFunctions stay linear in line length', () => {
  const unit = 'function f(a,b){if(a&&b){return a+b}else{return 0}}';
  const line = unit.repeat(Math.ceil((400 * 1024) / unit.length));
  const re = { head: /function\s+\w*\(/g, tail: /\s*\{/ };

  const ms = bestOf(3, () => {
    const { blocks } = analyzeBraces(line);
    measureFunctions([line], blocks, signaturesFrom(stripNoise(line), re));
  });
  assert.ok(ms < 500, `shape analysis scaled worse than linearly: ${ms}ms`);
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
  assert.deepStrictEqual(measured.map((b) => b.startLine).sort(), [1, 2]);
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

test('does not catastrophically backtrack on a long line', () => {
  const long = 'function a() { ' + 'x'.repeat(20000) + ' }';
  const ms = bestOf(3, () => {
    analyzeBraces(long);
    estimateComplexity(long);
  });
  assert.ok(ms < 500, `took too long on pathological input: ${ms}ms`);
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
