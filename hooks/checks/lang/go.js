#!/usr/bin/env node
// procoder — Go pack.

const {
  analyzeBraces, lineRuleFindings, measureFunctions, shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

const EXTENSIONS = ['.go'];

// `, _ :=`/`, _ =` discarding an error. Deliberately requires the assigned
// expression to look like a call (`identifier(` or `pkg.Method(`, dots
// allowed) directly after the operator, with no arbitrary text in between.
// `for i, _ := range xs` / `for k, _ := range m` never satisfy this, because
// after `range` there is a space before the next identifier, never a `(` —
// so the loop form never matches, only genuine `x, _ := someCall(...)` does.
const IGNORED_ERROR = /,\s*_\s*:?=\s*[\w.]+\(|\b_\s*=\s*\w+\.(?:Close|Write|Exec)\s*\(/;

const LINE_RULES = [
  {
    id: 'true/ignored-error', rung: 'TRUE',
    re: IGNORED_ERROR,
    message: 'error discarded into _',
    fix: 'handle it, wrap it with context, or return it',
  },
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:Query|QueryRow|Exec)\w*\s*\(\s*(?:fmt\.Sprintf|["'`][^"'`]*["'`]\s*\+)/,
    message: 'SQL built by Sprintf or concatenation',
    fix: 'use placeholders ($1, ?) and pass the values as arguments',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /exec\.Command\s*\(\s*["'`](?:sh|bash|cmd|powershell)["'`]\s*,\s*["'`]-c/,
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with an argument slice',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /InsecureSkipVerify\s*:\s*true/,
    message: 'TLS certificate verification disabled',
    fix: 'configure RootCAs with the proper certificate',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\b(?:md5|sha1)\.(?:New|Sum)\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use sha256, or argon2/bcrypt for passwords',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*:?=\s*rand\.(?:Int|Intn|Int63|Float64)\b/i,
    message: 'math/rand used for a security value',
    fix: 'use crypto/rand',
  },
  {
    id: 'true/panic-in-library', rung: 'TRUE',
    re: /^\s*panic\s*\(/,
    message: 'panic in library code crashes the caller',
    fix: 'return an error and let the caller decide',
  },
  {
    id: 'true/unclosed-resource', rung: 'TRUE',
    re: /\b(?:resp|res|f|file|conn|rows)\s*,\s*(?:err|_)\s*:?=\s*(?:http\.(?:Get|Post|Do)|os\.(?:Open|Create)|net\.Dial|db\.Query)\b/,
    message: 'resource opened without a visible Close',
    fix: 'add defer <resource>.Close() nearby',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bfmt\.Print(?:ln|f)?\s*\(|\bspew\.Dump\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

// How many lines to look ahead for a `defer <resource>.Close()` that
// discharges the unclosed-resource finding — covers the common
// `if err != nil { return ... }` guard immediately before the defer.
const DEFER_LOOKAHEAD = 5;

const FUNC_SIGNATURE = /func\s+(?:\([^)]{0,500}\)\s*)?\w*\s*\(([^)]{0,500})\)[^{\n]{0,500}\{/g;

const DEFERRED_CLOSE = /defer\s+\w+(?:\.\w+)*\.Close\s*\(/;

// A nearby `defer x.Close()` discharges the unclosed-resource rule.
function closedNearby(rule, lines, lineNo) {
  if (rule.id !== 'true/unclosed-resource') return false;
  return DEFERRED_CLOSE.test(lines.slice(lineNo, lineNo + DEFER_LOOKAHEAD).join('\n'));
}

function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  return [
    ...lineRuleFindings(LINE_RULES, lines, {
      skip: (rule, line, lineNo) => closedNearby(rule, lines, lineNo),
    }),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, FUNC_SIGNATURE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'func',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
