// tests/shape.test.js
const test = require('node:test');
const assert = require('node:assert');
const {
  analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
  signaturesFrom, stripNoise,
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

test('does not catastrophically backtrack on a long line', () => {
  const long = 'function a() { ' + 'x'.repeat(20000) + ' }';
  const start = Date.now();
  analyzeBraces(long);
  estimateComplexity(long);
  assert.ok(Date.now() - start < 500, 'took too long on pathological input');
});
