// tests/instructions.test.js
const test = require('node:test');
const assert = require('node:assert');
const { getProcoderInstructions, RANK, collapseBlankRuns } = require('../hooks/procoder-instructions');

test('off yields no instructions', () => {
  assert.strictEqual(getProcoderInstructions('off'), '');
});

test('frontmatter is stripped from every level', () => {
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    const out = getProcoderInstructions(level);
    assert.ok(!out.startsWith('---'), `${level} leaked frontmatter`);
    assert.ok(out.includes('SAFE'), `${level} lost the ladder`);
  }
});

test('level markers never appear in output', () => {
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    assert.ok(!/<!-- \/?level/.test(getProcoderInstructions(level)),
      `${level} leaked a level marker`);
  }
});

function nonBlankLines(text) {
  return text.split('\n').filter((line) => line.trim() !== '');
}

function assertLineSubset(lowerLevel, higherLevel) {
  const lowerLines = nonBlankLines(getProcoderInstructions(lowerLevel));
  const higherLines = nonBlankLines(getProcoderInstructions(higherLevel));
  const higherSet = new Set(higherLines);
  for (const line of lowerLines) {
    assert.ok(higherSet.has(line),
      `${higherLevel} is missing a line present in ${lowerLevel}: ${JSON.stringify(line)}`);
  }
  assert.ok(higherLines.length > lowerLines.length,
    `${higherLevel} must add content over ${lowerLevel}`);
}

test('higher levels are supersets of lower ones', () => {
  // Real containment, not just length: every non-blank line of the lower
  // level must appear somewhere in the higher level's output.
  assertLineSubset('pragmatic', 'strict');
  assertLineSubset('strict', 'paranoid');
});

test('an unknown level is treated as the default', () => {
  assert.strictEqual(
    getProcoderInstructions('bogus'),
    getProcoderInstructions('strict'));
});

test('RANK orders the levels', () => {
  assert.ok(RANK.pragmatic < RANK.strict && RANK.strict < RANK.paranoid);
});

test('blank-line collapse skips fenced code blocks', () => {
  const fixture = [
    'before',
    '',
    '',
    '',
    '```js',
    'const a = 1;',
    '',
    '',
    'const b = 2;',
    '```',
    '',
    '',
    '',
    'after',
  ].join('\n');

  const out = collapseBlankRuns(fixture);

  // Outside the fence, 3+ blank lines still collapse to one.
  assert.ok(out.includes('before\n\n```'), 'blank-run collapse outside fences regressed');
  assert.ok(out.includes('```\n\nafter'), 'blank-run collapse outside fences regressed');

  // Inside the fence, the blank line between the two statements must survive.
  assert.ok(out.includes('const a = 1;\n\n\nconst b = 2;'),
    'blank line inside a fenced code block was collapsed');
});
