#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const { blankStringInteriors, stripComments } = require('./comments');
const { spanRuleFindings } = require('./spans');
const {
  CONCAT, packContext, skipConstant, taintFindings, valuePattern,
} = require('./taint');
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
    // `dataSink`: the finding is "untrusted markup reaches this sink", not
    // "this API is the defect" — `el.innerHTML = ''` is how a list is cleared,
    // and constant markup is not XSS. constantLine reads the right of a
    // top-level `=` as data, which is what keeps `el.innerHTML = userInput`
    // reported; without that the discharge would silence the rule outright.
    id: 'safe/xss-sink', rung: 'SAFE', dataSink: true,
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
    // `fetch` has no timeout anywhere it runs — not in a browser, not in Node —
    // and an axios call without `timeout` inherits none either. Both park the
    // caller until the far end answers.
    //
    // Only the SINGLE-argument form, and that bound is the whole rule. A second
    // argument may carry the `signal` (or axios's `timeout`) and may be a name
    // this rule cannot see inside — `fetch(url, opts)` — so reading it as
    // missing a timeout would be a blocking rung-2 finding on correct code.
    // Measured on a 6,000-file corpus, and three shapes had to be excluded for
    // the same reason, each of them a real package's correct code:
    //
    //   `fetch(...args)`   a passthrough wrapper (@reduxjs/toolkit). The
    //                      caller's own options, signal included, are in the
    //                      spread, and this rule cannot see into it.
    //   `axios(config)`    the bare call takes ONE config object, which is
    //                      where axios's own `timeout` lives — so the bare
    //                      form is not matched at all, only `axios.get(url)`
    //                      and its siblings, whose first argument is the URL.
    //   a local `fetch`    a module that declares its own (esbuild's installer
    //                      wraps `https.get`; @protobufjs/fetch requires one)
    //                      is not calling the global — see FETCH_SHADOW.
    //
    // The lookbehind keeps the method spelling out for the same reason:
    // `cache.fetch(key)` and `this.fetch(url)` are somebody else's fetch.
    id: 'true/missing-timeout', rung: 'TRUE',
    re: /(?<![.\w$])(?:fetch|axios\.(?:get|post|put|patch|delete|head|request))\s*\(\s*(?!\.\.\.)[^(),]{0,200}\)/,
    message: 'outbound call with no timeout — fetch has none, and neither has an axios call without one',
    fix: 'pass { signal: AbortSignal.timeout(ms) }, or axios { timeout: ms }',
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
const JS_WORD = String.raw`[A-Za-z_$][\w$]*`;
// A dotted path, so `o.q` and `this.query` are bindings of their own, plus the
// call and method-chain suffixes a value picks up on its way into a sink — see
// valuePattern in taint.js.
const JS_VALUE = valuePattern(JS_WORD);
const JS_PATH = String.raw`(${JS_WORD}(?:\.${JS_WORD})*)`;

// A template literal with a hole in it — unless the tag is `sql`.
//
// A tagged template is a call over the template's *raw parts*, and the `sql`
// tag — drizzle, postgres.js, slonik — turns every `${…}` into a bind
// parameter rather than into text: interpolating into it is the parameterized
// form, not string building, so it is not a source. Reporting it was a rung-1
// finding on the exact API those libraries tell you to use.
//
// `(?<![\w$])` pins the match to the start of a word run — one starting offset
// per run rather than one per character, the same pin and the same reason as
// CONCAT in taint.js — and the lookahead rejects only the exact `sql` tag:
// `raw`, `mysql` and an untagged `` `…${x}` `` are all still sources.
const JS_TEMPLATE = /(?<![\w$])(?!sql`)[\w$]*`[^`\n]*\$\{/i;

const TAINT = {
  // The optional `(?::[^=\n]*)?` is a type annotation: `const q: string = …` is
  // how a typed codebase writes the binding, and without it the recogniser
  // stopped at the `:` and established no taint at all.
  assign: new RegExp(String.raw`^\s*(?:(?:const|let|var)\s+)?${JS_PATH}\s*(?::[^=\n]*)?\+?=(?![=>])`),
  // The same recogniser with the declarator required. A declaration binds
  // afresh at this block's level; a bare assignment writes whatever binding is
  // already live, which is what carries a branch's or a loop body's work out
  // of the block — see bind() in taint.js.
  declare: /^\s*(?:const|let|var)\s/,
  // The enclosing function's name, for return propagation. `function build(` and
  // `const build = (` are the two spellings worth following; a method shorthand
  // is deliberately not, since its name is ambiguous with any call.
  func: new RegExp(String.raw`^\s*(?:export\s+)?(?:async\s+)?(?:function\s*\*?\s*|(?:const|let|var)\s+)(${JS_WORD})`),
  sources: [JS_TEMPLATE, ...CONCAT],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: new RegExp(String.raw`\b(?:query|execute|raw)\s*\(\s*${JS_VALUE}\s*[,)]`, 'i'),
      message: 'SQL built by interpolation or concatenation reaches a query',
      fix: 'use a parameterized query with bound values',
    },
    {
      id: 'safe/shell-injection',
      re: new RegExp(
        String.raw`(?:\bchild_process\.|(?<![.\w$]))(?:execSync|exec)\s*\(\s*${JS_VALUE}\s*[,)]`),
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

// A module that declares its own `fetch` is not calling the platform's, and
// what that local one does about timeouts is its own business — esbuild's
// installer wraps `https.get` in one, @protobufjs/fetch requires one from its
// own package. File-scoped, like the Python pack's Session bindings: the name
// means whatever the file bound it to.
const FETCH_SHADOW = /(?:function\s+fetch\s*\(|(?:const|let|var)\s+fetch\s*=)/;
const FETCH_CALL = /(?<![.\w$])fetch\s*\(/;

function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const code = stripComments(text, 'js');
  // The structure input, and only it: a multi-line string's interior is data,
  // and the taint scan reads statements off this text. The rule input above
  // keeps every literal whole — see comments.js.
  //
  // stripNoise runs first, never after. Blanking takes the closing delimiter
  // with it, and stripNoise reading the blanked text would then find an opener
  // with no closer and eat forward to the next quote in the file.
  const stripped = blankStringInteriors(stripNoise(text), 'js');
  const { maxDepth, blocks } = analyzeBraces(text);
  const codeLines = code.split(/\r?\n/);
  const strippedLines = stripped.split(/\r?\n/);
  const ctx = packContext({ lines: codeLines, stripped: strippedLines, spec: TAINT });
  const shadowed = FETCH_SHADOW.test(code);
  const inline = lineRuleFindings(LINE_RULES, codeLines, {
    codeLines: strippedLines,
    skip: (rule, line) => (rule.id === 'true/missing-timeout' && shadowed && FETCH_CALL.test(line))
      || skipConstant(rule, line, ctx),
  });

  return [
    ...inline,
    ...spanRuleFindings(SPAN_RULES, codeLines, { existing: inline, ctx }),
    ...taintFindings({ spec: TAINT, ctx, existing: inline }),
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
