// tests/shape.test.js
const test = require('node:test');
const assert = require('node:assert');
const {
  analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
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

test('does not catastrophically backtrack on a long line', () => {
  const long = 'function a() { ' + 'x'.repeat(20000) + ' }';
  const start = Date.now();
  analyzeBraces(long);
  estimateComplexity(long);
  assert.ok(Date.now() - start < 500, 'took too long on pathological input');
});
