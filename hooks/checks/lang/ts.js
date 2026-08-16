#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const { stripComments } = require('./comments');
const { spanRuleFindings } = require('./spans');
const { CONCAT, skipConstant, taintFindings } = require('./taint');
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
//
// The literal is unbounded. It used to be `[^"]{0,500}`, which dropped the
// finding for any SQL or shell fragment longer than 500 characters — and a
// long literal is exactly where a stray `+ id` hides. Removing the bound does
// not reintroduce the quadratic shape the bound existed for, because the span
// here is *delimiter-terminated*: a scan can only start at a quote and ends at
// the next one, so the scans partition the line rather than each running to
// its end. The `[^)]`-style spans, which have no such terminator, moved to
// spans.js instead. Measured unchanged at 100KB and 400KB.
const LITERAL_PLUS = String.raw`(?:"[^"]*"|'[^']*')\s*\+`;

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE', dataSink: true,
    re: new RegExp(String.raw`\b(?:query|execute|raw|exec)\s*\(\s*(?:\`[^\`]*\$\{|${LITERAL_PLUS})`, 'i'),
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
    id: 'safe/dynamic-eval', rung: 'SAFE', dataSink: true,
    re: /\beval\s*\(|new\s+Function\s*\(|setTimeout\s*\(\s*["'`]/,
    message: 'dynamic code evaluation',
    fix: 'replace with a lookup table or a direct call',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE', dataSink: true,
    // The `shell: true` half moved to SPAN_RULES: its `[^)]{0,500}` had no
    // delimiter to terminate it, so it is the half that had to be bounded.
    re: new RegExp(String.raw`\b(?:child_process\.)?(?:exec|execSync)\s*\(\s*(?:\`[^\`]*\$\{|${LITERAL_PLUS})`),
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

// Rules whose needle may sit anywhere inside the call's own argument list —
// see spans.js. `spawn('sh', [cmd], { shell: true })` puts the option object
// after an array of any length, which is why this cannot be a line rule with a
// bounded span: the bound decided how long the command could be before the
// finding was silently dropped.
const SPAN_RULES = [
  {
    id: 'safe/shell-injection', rung: 'SAFE', within: 'call', dataSink: true,
    anchor: /\b(?:spawn|execFile)\s*\(/g,
    needles: [/\bshell\s*:\s*true/g],
    message: 'shell invoked with an interpolated command',
    fix: 'use execFile/spawn with an argument array and shell:false',
  },
];

// Local taint (taint.js): the assign-then-use form of the two rules above.
//
// The verb lists are deliberately not the line rules'. `exec` sits in the SQL
// rule's list there, which is why `exec("ls " + dir)` is reported as both
// safe/sql-injection and safe/shell-injection — one of the two is always
// wrong. A new mechanism does not have to inherit that, so `exec` belongs to
// the shell sink only, and the SQL sink keeps the verbs that are only ever
// SQL. `execSync` is spelled before `exec` so the alternation prefers it.
//
// safe/xss-sink and safe/dynamic-eval get no taint sink on purpose: both line
// rules already fire on the sink itself whatever the argument is
// (`.innerHTML =`, `eval(`), so a taint sink for them would report a second
// time for the same line and nothing new — the duplicate rung 4 forbids.
const JS_NAME = String.raw`([A-Za-z_$][\w$]*)`;

const TAINT = {
  assign: /^\s*(?:(?:const|let|var)\s+)?(?:this\.)?([A-Za-z_$][\w$]*)\s*\+?=(?![=>])/,
  sources: [/`[^`\n]*\$\{/, ...CONCAT],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: new RegExp(String.raw`\b(?:query|execute|raw)\s*\(\s*${JS_NAME}\s*[,)]`, 'i'),
      message: 'SQL built by interpolation or concatenation reaches a query',
      fix: 'use a parameterized query with bound values',
    },
    {
      id: 'safe/shell-injection',
      re: new RegExp(
        String.raw`(?:\bchild_process\.|(?<![.\w$]))(?:execSync|exec)\s*\(\s*${JS_NAME}\s*[,)]`),
      message: 'shell command built by interpolation or concatenation',
      fix: 'use execFile/spawn with an argument array and shell:false',
    },
  ],
};

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
// Head up to the parameter list's `(`, tail from where that list closes
// through an optional return-type annotation to the block-opening `{`. The
// parameters are counted by shape.js's paramSpans pass rather than captured,
// so a list of any length is measured — the bounded `([^)]{0,500})` this
// replaces had to stay bounded to keep the scan linear, and dropped the
// longest lists entirely. The remaining bounded spans are short by nature.
// jvm.js and dotnet.js anchor per line instead, which does not fit here — a TS
// signature may be nested in an expression or wrap its parameters across lines.
//
// `\w*`, not `\w+`, on that last branch: an arrow function passed as an
// argument — `rows.forEach((row, i) => {` — opens its parameter list with a
// bare `(` that no identifier precedes. The old pattern reached it only by
// accident, matching the *enclosing call's* paren and stopping its span at the
// first `)`, which happened to be the arrow's; splitting head from tail took
// that accident away, and with it the callback's length and complexity
// findings. Allowing the empty word matches the arrow's own list, and counts
// its parameters rather than the enclosing call's. The lookbehind still pins
// every non-empty match to an identifier's first character, so the scan stays
// linear: an empty `\w*` admits one attempt per offset, which is O(1) work
// each — the quadratic shape was re-running a greedy `\w+` to the end of a
// word run, and that is unchanged.
// `(?:\w+\s*)?`, not `\w*\s*`: a match may not *begin* on whitespace. With
// the old spelling an empty `\w*` let a match start at any offset of a
// whitespace run and scan the rest of it looking for a `(` — one scan per
// offset, which is quadratic, and comments.js blanks a comment to a run of
// spaces exactly that wide. A terminated 64KB license header on one line cost
// 1,611ms and 512KB cost 111,162ms, against 4ms and 28ms for the Python pack
// on the same input. The two branches here reach every `(` the old one did:
// with a word, the match starts at the word as before; without one, it starts
// at the `(` itself rather than somewhere in the whitespace ahead of it, and
// the parameter list it opens is the same list.
const FUNCTION_SIGNATURE = {
  head: /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?(?<!\w)(?:\w+\s*)?)\(/g,
  tail: /\s*(?::[^{=]{1,500})?(?:=>)?\s*\{/,
};

function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const code = stripComments(text, 'js');
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);
  const codeLines = code.split(/\r?\n/);
  const inline = lineRuleFindings(LINE_RULES, codeLines, {
    codeLines: stripped.split(/\r?\n/), skip: skipConstant,
  });

  return [
    ...inline,
    ...spanRuleFindings(SPAN_RULES, codeLines, { existing: inline }),
    ...taintFindings({
      lines: codeLines, stripped: stripped.split(/\r?\n/), spec: TAINT, existing: inline,
    }),
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
