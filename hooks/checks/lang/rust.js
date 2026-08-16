#!/usr/bin/env node
// procoder — Rust pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

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
    re: /Command::new\s*\(\s*"(?:sh|bash|cmd|powershell)"\s*\)[^;]*\.arg\s*\(\s*"-c"/,
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
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*(?::[^=]+)?=\s*rand::(?:random|thread_rng)\b/i,
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
const FN_SIGNATURE = /fn\s+\w+\s*(?:<[^>]*>)?\s*\(([^)]*)\)[^{\n]*\{/g;
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
function testRegions(lines) {
  const regions = [];
  lines.forEach((line, index) => {
    if (!TEST_ATTR.test(line)) return;

    // Find the line that opens the attributed item's block (may be the
    // attribute line itself for `#[test] fn t() {`, or a following line for
    // stacked attributes / multi-line signatures).
    let openIndex = -1;
    for (let j = index; j < lines.length; j += 1) {
      if (lines[j].includes('{')) { openIndex = j; break; }
    }
    if (openIndex === -1) return;

    let depth = 0;
    let closeIndex = lines.length - 1;
    for (let j = openIndex; j < lines.length; j += 1) {
      for (const ch of lines[j]) {
        if (ch === '{') depth += 1;
        else if (ch === '}') {
          depth -= 1;
          if (depth === 0) { closeIndex = j; break; }
        }
      }
      if (depth === 0 && j >= openIndex) { closeIndex = j; break; }
    }

    regions.push([index + 1, closeIndex + 1]);
  });
  return regions;
}

function inRegions(regions, lineNo) {
  return regions.some(([start, end]) => lineNo >= start && lineNo <= end);
}

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);
  const tests = testRegions(lines);
  const isTestFile = /(?:^|\/)tests\//.test(String(relPath || '')) || /_test\.rs$/.test(String(relPath || ''));

  lines.forEach((line, index) => {
    const lineNo = index + 1;

    for (const rule of LINE_RULES) {
      if (!rule.re.test(line)) continue;
      if (rule.id === 'true/unwrap-in-library'
          && (isTestFile || inRegions(tests, lineNo) || LOCK_UNWRAP.test(line))) continue;
      findings.push(finding({
        rung: rule.rung, id: rule.id, line: lineNo,
        message: rule.message, fix: rule.fix,
      }));
    }

    if (UNSAFE_BLOCK.test(line)) {
      const preceding = lines.slice(Math.max(0, index - 3), index).join('\n');
      if (!SAFETY_COMMENT.test(preceding)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/unsafe-block', line: lineNo,
          message: 'unsafe block with no SAFETY comment',
          fix: 'add // SAFETY: stating the invariants that make this sound',
        }));
      }
    }
  });

  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(FN_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'fn',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
