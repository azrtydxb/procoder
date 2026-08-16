#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const { stripComments } = require('./comments');
const {
  analyzeBraces, emptyCatchFindings, lineRuleFindings, measureFunctions,
  shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

const EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs'];

// Every rule matches comment- and regex-stripped code, as in the other five
// packs; string literals stay, because build tooling and SSR assemble real
// sinks inside them. `onCode` additionally strips strings, and is set on the
// one rule whose pattern is an operator of the grammar rather than a sink: a
// ternary cannot occur inside a literal, so a string that merely reads
// `"a ? b ? 1 : 2 : 3"` is not one. See comments.js for the shared principle.
//
// A string literal followed by `+`, matching each quote style separately so a
// literal may contain the *other* quote — `"rm '"` and `'rm "'` are exactly how
// shell and SQL fragments get built, and a single `["'][^"']*["']` class misses
// both. Each branch is anchored on its own quote, so there is no backtracking.
const LITERAL_PLUS = String.raw`(?:"[^"]{0,500}"|'[^']{0,500}')\s*\+`;

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: new RegExp(String.raw`\b(?:query|execute|raw|exec)\s*\(\s*(?:\`[^\`]{0,500}\$\{|${LITERAL_PLUS})`, 'i'),
    message: 'SQL built by interpolation or concatenation',
    fix: 'use a parameterized query with bound values',
  },
  {
    id: 'safe/xss-sink', rung: 'SAFE',
    re: /\.innerHTML\s*=|\.outerHTML\s*=|dangerouslySetInnerHTML|document\.write\s*\(/,
    message: 'raw HTML sink',
    fix: 'use textContent, or sanitize before assigning',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE',
    re: /\beval\s*\(|new\s+Function\s*\(|setTimeout\s*\(\s*["'`]/,
    message: 'dynamic code evaluation',
    fix: 'replace with a lookup table or a direct call',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: new RegExp(String.raw`\b(?:child_process\.)?(?:exec|execSync)\s*\(\s*(?:\`[^\`]{0,500}\$\{|${LITERAL_PLUS})|\b(?:spawn|execFile)\s*\([^)]{0,500}\bshell\s*:\s*true`),
    message: 'shell invoked with an interpolated command',
    fix: 'use execFile/spawn with an argument array and shell:false',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /rejectUnauthorized\s*:\s*false|NODE_TLS_REJECT_UNAUTHORIZED\s*=\s*["']?0/,
    message: 'TLS certificate verification disabled',
    fix: 'trust the proper CA instead of disabling verification',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|otp|session[_-]?id|password)\b[^\n]{0,40}Math\.random\s*\(/i,
    message: 'Math.random() used for a security value',
    fix: 'use crypto.randomUUID() or crypto.randomBytes()',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bdebugger\s*;|\bconsole\.(?:log|debug|dir|trace)\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
  {
    id: 'obvious/nested-ternary', rung: 'OBVIOUS', onCode: true,
    re: /\?[^?:\n]{0,500}\?[^:\n]{0,500}:[^:\n]{0,500}:/,
    message: 'nested ternary',
    fix: 'rewrite as if/else or a lookup',
  },
];

// try { ... } catch (e) { }  — with only whitespace or a comment in the block.
const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;

// Method-shorthand and call-like branch — `foo(a) {`, `async foo(a) {` — is the
// only one not pinned to a keyword, so `\w+` may start anywhere. `(?<!\w)` pins
// it to the start of an identifier, which is what makes the scan linear: inside
// an unbroken word run every other offset now fails on the lookbehind instead of
// re-running the greedy `\w+` to the end of the run, which was quadratic (8KB
// 54ms, 50KB 2,230ms, 100KB 9,041ms). It excludes no signature that was matched
// before: starting mid-identifier could only ever reach the same `\w+` end, so
// the match at the identifier's start is the same match, on the same line, with
// the same parameter text. `\w` deliberately, not `[\w$]` — a `$` may precede an
// identifier (`$foo(a) {`), and a run mixing `$` in stays linear anyway because
// each word run still admits one starting offset.
//
// Spans stay bounded as in go.js and rust.js; jvm.js and dotnet.js anchor per
// line instead, which does not fit here — a TS signature may be nested in an
// expression or wrap its parameters across lines.
const FUNCTION_SIGNATURE =
  /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?(?<!\w)\w+\s*)\(([^)]{0,500})\)\s*(?::[^{=]{1,500})?(?:=>)?\s*\{/g;

function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const code = stripComments(text, 'js');
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  return [
    ...lineRuleFindings(LINE_RULES, code.split(/\r?\n/), { codeLines: stripped.split(/\r?\n/) }),
    ...emptyCatchFindings(code, SWALLOWED, 'error swallowed by an empty catch'),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, FUNCTION_SIGNATURE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'function',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
