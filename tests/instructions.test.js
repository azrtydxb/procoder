// tests/instructions.test.js
const test = require('node:test');
const assert = require('node:assert');
const { getProcoderInstructions, RANK } = require('../hooks/procoder-instructions');

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
