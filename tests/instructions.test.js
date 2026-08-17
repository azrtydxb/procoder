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

// --- the subagent digest ---------------------------------------------------
//
// The session pays for the doctrine once; SubagentStart pays per subagent. The
// digest is what makes that multiplier affordable, and these tests are what
// stop it from becoming a second, weaker doctrine.

test('digest markers never appear in either output', () => {
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    for (const digest of [false, true]) {
      assert.ok(!/<!-- \/?digest/.test(getProcoderInstructions(level, { digest })),
        `${level} digest=${digest} leaked a digest marker`);
    }
  }
});

test('the digest is materially smaller than the full text', () => {
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    const full = getProcoderInstructions(level).length;
    const digest = getProcoderInstructions(level, { digest: true }).length;
    assert.ok(digest < full * 0.9,
      `${level}: digest ${digest} is not meaningfully smaller than ${full}`);
  }
});

// The point of the digest is that a subagent still writes gated code. Every
// rung, and every non-negotiable rule inside rungs 1 and 2, has to survive it.
test('the digest keeps all four rungs and every rule the engine cannot compute', () => {
  const digest = getProcoderInstructions('strict', { digest: true }).toLowerCase().replace(/\s+/g, ' ');
  for (const phrase of [
    'safe', 'true', 'obvious', 'alone',
    'validate at the boundary, not downstream',
    'parameterized queries only',
    'enforced server-side, per-request',
    'the agent is a boundary too',
    'money is never a float',
    'cost is behavior',
    'typecheck → lint → tests → build',
    'removal trigger',
    'names say **what**, never **how**',
  ]) {
    assert.ok(digest.includes(phrase), `the digest dropped a rule: ${phrase}`);
  }
});

// Polarity. An unmarked rule is IN the digest, so the failure direction is a
// digest that is slightly too long rather than one missing a rung. If this
// inverts, every rule added from then on is silently absent from every subagent.
test('text nobody marked survives into the digest', () => {
  const full = getProcoderInstructions('strict');
  const digest = getProcoderInstructions('strict', { digest: true });
  const unmarked = 'A change isn\'t done until the thing it replaced is gone.';
  assert.ok(full.includes(unmarked), 'fixture line is no longer in the doctrine');
  assert.ok(digest.includes(unmarked), 'the digest dropped unmarked text');
});
