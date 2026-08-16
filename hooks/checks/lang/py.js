#!/usr/bin/env node
// procoder — Python pack.

const { finding } = require('../finding');
const { stripComments } = require('./comments');
const {
  SIGNATURE_LOOKBACK,
  analyzeIndent, countParams, estimateComplexity, lineRuleFindings, shapeFindings,
} = require('../shape');

const EXTENSIONS = ['.py'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:execute|executemany|raw|text)\s*\(\s*(?:f["']|["'][^"']*["']\s*%|["'][^"']*["']\s*\+|["'][^"']*["']\s*\.format\s*\()/i,
    message: 'SQL built by f-string, % or concatenation',
    fix: 'pass parameters as the second argument instead',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /shell\s*=\s*True|\bos\.system\s*\(|\bos\.popen\s*\(/,
    message: 'shell execution with an interpolated command',
    fix: 'pass an argument list and leave shell=False',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE',
    re: /\beval\s*\(|\bexec\s*\(|\b__import__\s*\(/,
    message: 'dynamic code evaluation',
    fix: 'replace with a dict lookup or a direct call',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /\bpickle\.loads?\s*\(|\bmarshal\.loads?\s*\(|\byaml\.load\s*\((?![^)]*Safe)/,
    message: 'unsafe deserialization of untrusted bytes',
    fix: 'use json, or yaml.safe_load',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\bhashlib\.(?:md5|sha1)\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use hashlib.sha256, or argon2/bcrypt for passwords',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /verify\s*=\s*False|ssl\._create_unverified_context/,
    message: 'TLS certificate verification disabled',
    fix: 'point at the proper CA bundle instead',
  },
  {
    id: 'true/mutable-default', rung: 'TRUE',
    // The parameter span is bounded: `[^)]*` is retried from every `def` on a
    // line full of them, and with no `)` anywhere it runs to end of line each
    // time — quadratic (653ms on a 100KB line, 2739ms at 200KB, the whole cost
    // of this pack on that input). Bounding it makes it linear; the conjunction
    // is unchanged, and 500 characters is the same bound go.js, rust.js and
    // dotnet.js already put on their argument spans.
    re: /\bdef\s+\w+\s*\([^)]{0,500}=\s*(?:\[\s*\]|\{\s*\}|set\s*\(\s*\))/,
    message: 'mutable default argument — shared across calls',
    fix: 'default to None and build the value inside the function',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /^\s*print\s*\(|\bbreakpoint\s*\(\s*\)|\bpdb\.set_trace\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

const BARE_EXCEPT = /^\s*except\s*:\s*$/;
const BROAD_EXCEPT = /^\s*except\s+(?:Exception|BaseException)\b[^:]*:\s*$/;
const SILENT_BODY = /^\s*(?:pass|\.\.\.)\s*$/;
const DEF_HEAD = /^\s*(?:async\s+)?def\s+\w+\s*\(/;

// `self`/`cls` is the receiver, not an argument the caller supplies. Counting
// it would make the parameter budget one tighter for methods than for
// functions — and tighter than for the brace packs, where the receiver is an
// implicit `this` that was never counted.
const RECEIVER = /^\s*(?:self|cls)\s*(?:,|$)/;

// The parameter text of a `def`, read across the lines its list wraps over.
// black wraps one parameter per line past the line width, so in formatted
// Python a many-parameter signature is always wrapped — reading only the `def`
// line saw an empty list for exactly the functions the params check exists to
// catch. Same shape as shape.js's rescanner for the brace packs: take the
// signature's own line plus its continuations, bounded by the same lookback so
// an unclosed paren cannot walk the file.
//
// One left-to-right scan tracking bracket depth, rather than a regex: `)` also
// ends a default value or an annotation, and `[^)]*` stopped at the first of
// them.
const BRACKET = { '(': 1, '[': 1, '{': 1, ')': -1, ']': -1, '}': -1 };

// Index of the bracket that closes the signature within `text`, or -1. `state`
// carries the depth across the lines a wrapped list spans.
function closeIndex(text, state) {
  for (let i = 0; i < text.length; i += 1) {
    const step = BRACKET[text[i]] || 0;
    if (step < 0 && state.depth === 0) return i;
    state.depth += step;
  }
  return -1;
}

function defParams(lines, start) {
  const head = DEF_HEAD.exec(lines[start] || '');
  if (!head) return null;

  const state = { depth: 0 };
  let text = lines[start].slice(head[0].length);
  let params = '';
  for (let ln = start; ln - start < SIGNATURE_LOOKBACK; ln += 1) {
    const close = closeIndex(text, state);
    if (close !== -1) return params + text.slice(0, close);
    params += text;
    text = ' ' + (lines[ln + 1] === undefined ? '' : lines[ln + 1]);
  }
  return null;
}

function countDefParams(lines, start) {
  const params = defParams(lines, start);
  if (params === null) return 0;
  return countParams('(' + params + ')') - (RECEIVER.test(params) ? 1 : 0);
}

function exceptFindings(lines) {
  const findings = [];
  lines.forEach((line, index) => {
    if (BARE_EXCEPT.test(line)) {
      findings.push(finding({
        rung: 'TRUE', id: 'true/bare-except', line: index + 1,
        message: 'bare except catches SystemExit and KeyboardInterrupt too',
        fix: 'catch the specific exception you can actually handle',
      }));
      return;
    }
    if (BROAD_EXCEPT.test(line) && SILENT_BODY.test(lines[index + 1] || '')) {
      findings.push(finding({
        rung: 'TRUE', id: 'true/swallowed-error', line: index + 1,
        message: 'exception silently discarded',
        fix: 'log with context and re-raise, or handle it explicitly',
      }));
    }
  });
  return findings;
}

// Python has no signature-opening brace to key on, so every indent block is
// measured and the def line, when there is one, supplies the parameters.
function measureBlocks(lines, blocks) {
  return blocks.map((block) => ({
    ...block,
    params: countDefParams(lines, block.startLine - 1),
    complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
  }));
}

// Every rule here matches code, never prose: `# never use eval(user_input)`
// documents the hole, it does not open one. String literals stay — an f-string
// or a `%` template *is* the SQL, and blanking it would blind the rule that
// looks for it. See comments.js for the principle the six packs share.
function check(source, { relPath, config } = {}) {
  const lines = stripComments(source, 'py').split(/\r?\n/);
  const { maxDepth, blocks } = analyzeIndent(source, { tabWidth: 4 });

  return [
    ...lineRuleFindings(LINE_RULES, lines),
    ...exceptFindings(lines),
    ...shapeFindings({
      blocks: measureBlocks(lines, blocks),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'function',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
