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

test('higher levels are supersets of lower ones', () => {
  const pragmatic = getProcoderInstructions('pragmatic').length;
  const strict = getProcoderInstructions('strict').length;
  const paranoid = getProcoderInstructions('paranoid').length;
  assert.ok(pragmatic < strict, 'strict must add content over pragmatic');
  assert.ok(strict < paranoid, 'paranoid must add content over strict');
});

test('an unknown level is treated as the default', () => {
  assert.strictEqual(
    getProcoderInstructions('bogus'),
    getProcoderInstructions('strict'));
});

test('RANK orders the levels', () => {
  assert.ok(RANK.pragmatic < RANK.strict && RANK.strict < RANK.paranoid);
});
