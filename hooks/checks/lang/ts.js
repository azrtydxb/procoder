#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const {
  analyzeBraces, emptyCatchFindings, lineRuleFindings, measureFunctions,
  shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

const EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs'];

// `onCode` runs the rule against the noise-stripped line instead of the raw one.
// Set it where the pattern describes code shape rather than string content, so a
// regex literal or a string that merely quotes the pattern is not a hit. Rules
// that must see literal text (SQL built inside a template string) leave it off.
const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:query|execute|raw|exec)\s*\(\s*(?:`[^`]*\$\{|["'][^"']*["']\s*\+)/i,
    message: 'SQL built by interpolation or concatenation',
    fix: 'use a parameterized query with bound values',
  },
  {
    id: 'safe/xss-sink', rung: 'SAFE', onCode: true,
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
    re: /\b(?:child_process\.)?(?:exec|execSync)\s*\(\s*(?:`[^`]*\$\{|["'][^"']*["']\s*\+)|\b(?:spawn|execFile)\s*\([^)]*\bshell\s*:\s*true/,
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
    re: /\?[^?:\n]*\?[^:\n]*:[^:\n]*:/,
    message: 'nested ternary',
    fix: 'rewrite as if/else or a lookup',
  },
];

// try { ... } catch (e) { }  — with only whitespace or a comment in the block.
const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;

const FUNCTION_SIGNATURE =
  /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?\w+\s*)\(([^)]*)\)\s*(?::[^{=]+)?(?:=>)?\s*\{/g;

function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  return [
    ...lineRuleFindings(LINE_RULES, lines, { codeLines: stripped.split(/\r?\n/) }),
    ...emptyCatchFindings(stripped, SWALLOWED, 'error swallowed by an empty catch'),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, FUNCTION_SIGNATURE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'function',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
