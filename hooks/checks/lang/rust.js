#!/usr/bin/env node
// procoder — Rust pack.

const { finding } = require('../finding');
const { stripComments } = require('./comments');
const {
  analyzeBraces, lineRuleFindings, measureFunctions, shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

const EXTENSIONS = ['.rs'];

const LINE_RULES = [
  {
    id: 'true/unwrap-in-library', rung: 'TRUE',
    re: /\.\s*(?:unwrap|expect)\s*\(/,
    message: 'unwrap/expect panics on the caller',
    fix: 'propagate with ? and a typed error',
  },
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\bquery\w*\s*\(\s*&?format!|\bexecute\s*\(\s*&?format!/,
    message: 'SQL built with format!',
    fix: 'use bind parameters',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /Command::new\s*\(\s*"(?:sh|bash|cmd|powershell)"\s*\)[^;]{0,500}\.arg\s*\(\s*"-c"/,
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with separate args',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /danger_accept_invalid_certs\s*\(\s*true\s*\)|danger_accept_invalid_hostnames\s*\(\s*true\s*\)/,
    message: 'TLS certificate verification disabled',
    fix: 'add the proper root certificate instead',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*(?::[^=]{1,500})?=\s*rand::(?:random|thread_rng)\b/i,
    message: 'general-purpose RNG used for a security value',
    fix: 'use a CSPRNG (rand::rngs::OsRng or the ring crate)',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bdbg!\s*\(|\bprintln!\s*\(|\beprintln!\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or use the tracing/log crate',
  },
];

const UNSAFE_BLOCK = /^\s*(?:.*\s)?unsafe\s*\{/;
const SAFETY_COMMENT = /\/\/\s*SAFETY:|\/\/!\s*SAFETY:/i;
// Head up to the parameter list's `(`, tail from where that list closes to the
// block-opening `{`; shape.js counts the parameters in between without
// capturing them, so the list has no length ceiling. See go.js.
const FN_SIGNATURE = {
  head: /fn\s+\w+\s*(?:<[^>]{0,500}>)?\s*\(/g,
  tail: /[^{\n]{0,500}\{/,
};
const TEST_ATTR = /#\[cfg\(test\)\]|#\[test\]|#\[tokio::test\]/;
// unwrap()/expect() directly on a Mutex/RwLock lock() is near-universal Rust
// practice: a poisoned lock means a prior panic already corrupted shared
// state, and there is no sensible recovery short of aborting — so this is
// deliberately not flagged, unlike unwrap on ordinary fallible calls.
const LOCK_UNWRAP = /\.\s*(?:lock|read|write)\s*\(\s*\)\s*\.\s*(?:unwrap|expect)\s*\(/;

// unwrap in a #[cfg(test)] module or a #[test] fn is normal, not a finding.
// Scoped by brace matching to just that block, not "everything after this
// attribute to end of file" — a #[cfg(test)] module in the middle of a file
// must not blind the checker to library code that follows it.
// The line that opens the attributed item's block: the attribute line itself
// for `#[test] fn t() {`, or a following one for stacked attributes and
// multi-line signatures.
function blockOpenIndex(lines, from) {
  for (let j = from; j < lines.length; j += 1) {
    if (lines[j].includes('{')) return j;
  }
  return -1;
}

// The line holding the `}` that balances the block opened on `openIndex`.
function blockCloseIndex(lines, openIndex) {
  let depth = 0;
  for (let j = openIndex; j < lines.length; j += 1) {
    for (const ch of lines[j]) {
      if (ch === '{') depth += 1;
      else if (ch === '}') depth -= 1;
    }
    if (depth === 0) return j;
  }
  return lines.length - 1;
}

// Braces are counted on noise-stripped lines, as shape.js does: a `{` inside a
// string or a comment is not structure, and counting one leaves the depth
// unbalanced and runs the region to end of file. The attributes are matched on
// comment-stripped lines — `#` is not a comment in Rust, so `#[cfg(test)]`
// survives — which keeps a commented-out `// #[test]` from opening a region
// and blinding the checker to the library code that follows.
function testRegions(attrLines, codeLines) {
  const regions = [];
  attrLines.forEach((line, index) => {
    if (!TEST_ATTR.test(line)) return;
    const openIndex = blockOpenIndex(codeLines, index);
    if (openIndex === -1) return;
    regions.push([index + 1, blockCloseIndex(codeLines, openIndex) + 1]);
  });
  return regions;
}

function inRegions(regions, lineNo) {
  return regions.some(([start, end]) => lineNo >= start && lineNo <= end);
}

const TEST_PATH = /(?:^|\/)tests\/|_test\.rs$/;
const SAFETY_LOOKBACK = 3;

// How many lines back a // SAFETY: comment may sit and still count as the
// justification for this unsafe block.
//
// The block itself is found on comment-stripped lines — `// unsafe { ... }` in
// prose is not an unsafe block — while the justification is looked for in the
// raw ones, since a comment is exactly what this rule asks for.
function unsafeFindings(codeLines, lines) {
  const findings = [];
  codeLines.forEach((line, index) => {
    if (!UNSAFE_BLOCK.test(line)) return;
    const preceding = lines.slice(Math.max(0, index - SAFETY_LOOKBACK), index).join('\n');
    if (SAFETY_COMMENT.test(preceding)) return;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/unsafe-block', line: index + 1,
      message: 'unsafe block with no SAFETY comment',
      fix: 'add // SAFETY: stating the invariants that make this sound',
    }));
  });
  return findings;
}

// Every rule here matches code, never prose: `// never parse(x).unwrap()` and
// a doc comment showing the SQL not to build are documentation, not defects.
// String literals stay — `format!("SELECT {}", id)` *is* the SQL. The one
// comment a rule genuinely asks about, `// SAFETY:`, still reads raw lines.
// See comments.js for the principle the six packs share.
function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const codeLines = stripComments(text, 'c').split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  const tests = testRegions(codeLines, stripped.split(/\r?\n/));
  const isTestFile = TEST_PATH.test(String(relPath || ''));
  const expectedPanic = (rule, line, lineNo) => rule.id === 'true/unwrap-in-library'
    && (isTestFile || inRegions(tests, lineNo) || LOCK_UNWRAP.test(line));

  return [
    ...lineRuleFindings(LINE_RULES, codeLines, { skip: expectedPanic }),
    ...unsafeFindings(codeLines, lines),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, FN_SIGNATURE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'fn',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
