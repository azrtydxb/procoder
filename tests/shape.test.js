// tests/shape.test.js
const test = require('node:test');
const assert = require('node:assert');
const {
  analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
  measureFunctions, signaturesFrom, stripNoise,
} = require('../hooks/checks/shape');
const { DEFAULTS } = require('../hooks/checks/config');

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
  const re = /function\s+\w*\(([^)]*)\)\s*\{/g;

  const start = Date.now();
  signaturesFrom(stripNoise(src), re);
  assert.ok(Date.now() - start < 500, 'signaturesFrom scaled worse than linearly');
});

// A 400KB minified line, the size at which the two used to cost 2.0s and 44.8s
// respectively — each on its own over the 2s whole-file hook budget. Both now
// run in single-digit milliseconds. The bound is 500ms, two orders of magnitude
// above that, so a loaded CI machine cannot flake it; a return to quadratic
// scaling costs tens of seconds and fails it by a wide margin.
test('analyzeBraces and measureFunctions stay linear in line length', () => {
  const unit = 'function f(a,b){if(a&&b){return a+b}else{return 0}}';
  const line = unit.repeat(Math.ceil((400 * 1024) / unit.length));
  const re = /function\s+\w*\(([^)]*)\)\s*\{/g;

  const start = Date.now();
  const { blocks } = analyzeBraces(line);
  measureFunctions([line], blocks, signaturesFrom(stripNoise(line), re));
  assert.ok(Date.now() - start < 500, 'shape analysis scaled worse than linearly');
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
  const re = /(?:function\s+\w*|(?<!\w)\w+\s*)\(([^)]{0,500})\)\s*\{/g;
  const { blocks } = analyzeBraces(src);
  const measured = measureFunctions(
    src.split('\n'), blocks, signaturesFrom(stripNoise(src), re));
  assert.deepStrictEqual(measured.map((b) => b.startLine).sort(), [1, 2]);
});

// The bound is what keeps the work per unmeasured block constant. A signature
// wrapped further than it stays unmeasured — the same false negative as before,
// now confined to a shape no formatter produces under a sane params limit.
test('the signature lookback is bounded', () => {
  const wrap = (count) => [
    'function wide(',
    ...Array.from({ length: count }, (unused, i) => `  p${i},`),
    ') {',
    '  return 1;',
    '}',
  ].join('\n');

  assert.ok(paramFinding('ts', wrap(8), 'a.ts'), 'an 8-line wrap should be measured');
  assert.strictEqual(paramFinding('ts', wrap(40), 'a.ts'), undefined);
});

// The lookback runs for every block that is not itself a signature — most
// blocks in a file — so it has to cost the same whatever the lines around it
// weigh. 100KB and 400KB of a single line, and a file of 20k unmeasurable
// blocks, all stay far inside the 2s whole-file hook budget.
test('the signature lookback stays linear in line length and block count', () => {
  const re = /function\s+\w*\(([^)]{0,500})\)\s*\{/g;
  for (const size of [100, 400]) {
    const unit = 'if(a&&b){return a+b}else{return 0}';
    const line = unit.repeat(Math.ceil((size * 1024) / unit.length));
    const src = 'const x = 1;\n' + line;
    const start = Date.now();
    const { blocks } = analyzeBraces(src);
    measureFunctions(src.split('\n'), blocks, signaturesFrom(stripNoise(src), re));
    assert.ok(Date.now() - start < 500, `${size}KB single line scaled worse than linearly`);
  }

  const many = Array.from({ length: 20000 }, () => 'while (a) {\n}').join('\n');
  const start = Date.now();
  const { blocks } = analyzeBraces(many);
  measureFunctions(many.split('\n'), blocks, signaturesFrom(stripNoise(many), re));
  assert.ok(Date.now() - start < 1000, 'the lookback scaled worse than linearly in blocks');
});

test('does not catastrophically backtrack on a long line', () => {
  const long = 'function a() { ' + 'x'.repeat(20000) + ' }';
  const start = Date.now();
  analyzeBraces(long);
  estimateComplexity(long);
  assert.ok(Date.now() - start < 500, 'took too long on pathological input');
});
