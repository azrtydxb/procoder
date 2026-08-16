#!/usr/bin/env node
// procoder — Go pack.

const { stripComments } = require('./comments');
const { CONCAT, skipConstant, taintFindings } = require('./taint');
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
    id: 'safe/sql-injection', rung: 'SAFE', dataSink: true,
    re: /\b(?:Query|QueryRow|Exec)\w*\s*\(\s*(?:fmt\.Sprintf|["'`][^"'`]*["'`]\s*\+)/,
    message: 'SQL built by Sprintf or concatenation',
    fix: 'use placeholders ($1, ?) and pass the values as arguments',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE', dataSink: true,
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

// Local taint (taint.js): the assign-then-use form of the SQL rule above.
// `q := "SELECT ... id=" + id` then `db.Query(q)` is the shape gofmt'd code
// takes, and the line rule cannot see it.
//
// The sink allows the query to be either the first or the second argument,
// because the Context variants take `ctx` first; both captures are tested, so
// `db.QueryContext(ctx, q)` and `db.Query(q)` are read the same way. Shell
// gets no taint sink: `exec.Command("sh", "-c", …)` is already reported on
// the shell invocation itself, whatever the argument is.
const GO_NAME = String.raw`([A-Za-z_]\w*)`;

const TAINT = {
  // The optional type in `var q string = …` is written `(?:\s+[\w*.[\]]+)?`,
  // with the whitespace *inside* the group. Written the other way round the
  // group could start inside the name's own `\w*` run, and two nested
  // quantifiers over the same characters is the quadratic shape: an unbroken
  // 100KB word run cost 4,887ms, against 13ms for every other pack. Requiring
  // the space first means the group cannot overlap the name at all, so each
  // backtrack step of the name fails in constant time.
  assign: /^\s*(?:var\s+)?([A-Za-z_]\w*)(?:\s+[\w*.[\]]+)?\s*[:+]?=(?!=)/,
  sources: [/\bfmt\.Sprintf\s*\(/, /`[^`\n]*`\s*\+\s*(?=[A-Za-z_(])/, ...CONCAT],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: new RegExp(String.raw`\b(?:Query|QueryRow|QueryContext|QueryRowContext|ExecContext|Exec)\s*\(\s*(?:${GO_NAME}\s*,\s*)?${GO_NAME}\s*[,)]`),
      message: 'SQL built by Sprintf or concatenation reaches a query',
      fix: 'use placeholders ($1, ?) and pass the values as arguments',
    },
  ],
};

// How many lines to look ahead for a `defer <resource>.Close()` that
// discharges the unclosed-resource finding — covers the common
// `if err != nil { return ... }` guard immediately before the defer.
const DEFER_LOOKAHEAD = 5;

// Head up to the parameter list's `(`, tail from where that list closes to the
// block-opening `{`. The parameters themselves are counted by shape.js's single
// paramSpans pass rather than captured, so the list may be any length — a
// captured `([^)]{0,500})` had to be bounded to stay linear and lost every
// signature past the bound. The receiver group stays bounded: a receiver is one
// parameter by construction.
const FUNC_SIGNATURE = {
  head: /func\s+(?:\([^)]{0,500}\)\s*)?\w*\s*\(/g,
  tail: /[^{\n]{0,500}\{/,
};

const DEFERRED_CLOSE = /defer\s+\w+(?:\.\w+)*\.Close\s*\(/;

// A nearby `defer x.Close()` discharges the unclosed-resource rule.
function closedNearby(rule, lines, lineNo) {
  if (rule.id !== 'true/unclosed-resource') return false;
  return DEFERRED_CLOSE.test(lines.slice(lineNo, lineNo + DEFER_LOOKAHEAD).join('\n'));
}

// Every rule here matches code, never prose — `// don't build SQL with
// fmt.Sprintf` documents the hole, it does not open one — and the discharge
// looks at code too, so a commented-out `defer resp.Close()` cannot silence
// the resource rule. String literals stay: `"SELECT " + id` *is* the SQL.
// See comments.js for the principle the six packs share.
function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = stripComments(text, 'c').split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  const inline = lineRuleFindings(LINE_RULES, lines, {
    skip: (rule, line, lineNo) => closedNearby(rule, lines, lineNo) || skipConstant(rule, line),
  });

  return [
    ...inline,
    ...taintFindings({
      lines, stripped: stripped.split(/\r?\n/), spec: TAINT, existing: inline,
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
