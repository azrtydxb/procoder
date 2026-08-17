#!/usr/bin/env node
// procoder — Python pack.

const { finding } = require('../finding');
const { stripComments } = require('./comments');
const { CONCAT, packContext, skipConstant, taintFindings } = require('./taint');
const {
  analyzeIndent, countParams, estimateComplexity, lineRuleFindings, paramSpans, shapeFindings,
  stripNoise,
} = require('../shape');

const EXTENSIONS = ['.py'];

// `dataSink` marks the rules whose finding is "untrusted data reaches this
// call", as against "this API is the defect": a shell command, a string
// evaluated as code, bytes deserialized. Those are discharged when everything
// the call carries is a constant — see taint.js. safe/weak-hash and
// safe/tls-disabled carry no such mark, because `hashlib.md5(b"x")` is a
// finding about the algorithm and a literal argument says nothing about it.
const LINE_RULES = [
  {
    // Each quote style is matched on its own branch. A single `["'][^"']*["']`
    // class excludes *both* quotes from the literal's body, so it never saw
    // `"… name = '%s'" % name` — a literal that contains the other quote is
    // exactly how SQL gets written, and that is the most common Python
    // injection spelling there is. Each branch is anchored on its own quote and
    // terminated by it, so the scans partition the line and there is no
    // backtracking; see LITERAL_PLUS in ts.js for the same shape.
    id: 'safe/sql-injection', rung: 'SAFE', dataSink: true,
    re: /\b(?:execute|executemany|raw|text)\s*\(\s*(?:f["']|"[^"]*"\s*(?:[%+]|\.format\s*\()|'[^']*'\s*(?:[%+]|\.format\s*\())/i,
    message: 'SQL built by f-string, % or concatenation',
    fix: 'pass parameters as the second argument instead',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE', dataSink: true,
    re: /shell\s*=\s*True|\bos\.system\s*\(|\bos\.popen\s*\(/,
    message: 'shell execution with an interpolated command',
    fix: 'pass an argument list and leave shell=False',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE', dataSink: true,
    re: /\beval\s*\(|\bexec\s*\(|\b__import__\s*\(/,
    message: 'dynamic code evaluation',
    fix: 'replace with a dict lookup or a direct call',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE', dataSink: true,
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
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /^\s*print\s*\(|\bbreakpoint\s*\(\s*\)|\bpdb\.set_trace\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

// Local taint (taint.js): the assign-then-use form of the SQL rule above.
// Scope is the indentation column the name was bound at, so a name bound in
// one `def` is gone by the next — Python has no block scope, so a value bound
// inside an `if` and used after it is a deliberate miss, erring toward silence.
//
// `text` is in the line rule's verb list and not here: as a bare
// `text(value)` it is far too common an identifier to key a finding on. Shell
// and eval get no taint sink — `os.system(`, `shell=True`, `eval(` and
// `exec(` are already reported on the sink itself whatever the argument is.
const TAINT = {
  indent: true,
  // The optional `(?:\s*:[^=\n]*)?` is an annotation: `q: str = "…" + user_id`
  // established no taint at all, because the recogniser stopped at the `:`.
  assign: /^\s*([A-Za-z_][\w]*)\s*(?::[^=\n]*)?\+?=(?!=)/,
  // A `def`'s parameters, which shadow any enclosing binding of the same name.
  // Python's statements have no block-opening brace to cut them at, so the
  // generic "the list this statement ends with" would read every call's
  // arguments as a binding; `def` is the only thing here that binds.
  params: /^\s*(?:async\s+)?def\s+\w+\s*\(([^()]*)\)/,
  // Each quote style on its own branch, for the reason spelled out on the line
  // rule above: `["'][^"'\n]*` excludes both quotes from the literal's body, so
  // `f"… name = '{name}'"` — an interpolation inside single quotes inside a
  // double-quoted f-string, and the most common Python injection spelling —
  // matched nothing and established no taint.
  sources: [
    /\bf"[^"\n]*\{/,
    /\bf'[^'\n]*\{/,
    /"[^"\n]*"\s*%\s*[A-Za-z_({[]/,
    /'[^'\n]*'\s*%\s*[A-Za-z_({[]/,
    /"[^"\n]*"\s*\.\s*format\s*\(\s*[^)\s]/,
    /'[^'\n]*'\s*\.\s*format\s*\(\s*[^)\s]/,
    ...CONCAT,
  ],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: /\b(?:executemany|execute|raw)\s*\(\s*([A-Za-z_][\w]*)\s*[,)]/i,
      message: 'SQL built by f-string, % or concatenation reaches a cursor',
      fix: 'pass parameters as the second argument instead',
    },
  ],
};

const BARE_EXCEPT = /^\s*except\s*:\s*$/;
const BROAD_EXCEPT = /^\s*except\s+(?:Exception|BaseException)\b[^:]*:\s*$/;
const SILENT_BODY = /^\s*(?:pass|\.\.\.)\s*$/;
const DEF_HEAD = /^\s*(?:async\s+)?def\s+\w+\s*\(/;

// `self`/`cls` is the receiver, not an argument the caller supplies. Counting
// it would make the parameter budget one tighter for methods than for
// functions — and tighter than for the brace packs, where the receiver is an
// implicit `this` that was never counted.
const RECEIVER = /^\s*(?:self|cls)\s*(?:,|$)/;

// The parameter text of every `def` in the file, keyed by the line its `def`
// starts on, however many lines its list wraps over.
//
// black wraps one parameter per line past the line width, so in formatted
// Python a many-parameter signature is always wrapped — reading only the `def`
// line saw an empty list for exactly the functions the params check exists to
// catch. Following the continuations fixed that, but bounded: the walk stopped
// after ten lines, so one parameter per line made an eight-parameter `def` the
// widest this pack could see. Past that the list never closed, `defParams`
// returned null, and the whole function dropped out of measurement — length,
// complexity and mutable defaults with it. The five brace packs have no such
// bound, so Python's most-wrapped — most-parametered — functions were the ones
// nobody measured.
//
// The bound cannot simply be raised: a scan that walks from every `def` until
// its list closes is quadratic on a file of unclosed parens, and two separate
// quadratics have already come out of exactly that shape here. shape.js's
// paramSpans answers where *every* `(` in the file closes and how many
// top-level parameters it holds, in one left-to-right pass over the source —
// so matching each `def` to its own `)` costs one map lookup and no search,
// whatever the wrap. It is the same pass, by the same rule, the brace packs
// already match their signatures with.
//
// Read off the noise-stripped source rather than the comment-stripped lines,
// so a bracket or a comma inside a string literal cannot move the count.
function defParamText(stripped) {
  const spans = paramSpans(stripped);
  const texts = new Map();
  let at = 0;
  stripped.split('\n').forEach((line, index) => {
    const head = DEF_HEAD.exec(line);
    const span = head && spans.get(at + head[0].length - 1);
    if (span) texts.set(index, stripped.slice(at + head[0].length, span.end));
    at += line.length + 1;
  });
  return texts;
}

// `x=[]`, `x={}`, `x=set()` — a default built once at definition time and
// then shared by every call that does not override it.
const MUTABLE_DEFAULT = /=\s*(?:\[\s*\]|\{\s*\}|set\s*\(\s*\))/;

// `hashlib.md5(data, usedforsecurity=False)` — the standard, explicit way to
// say this hash is not a security primitive: a cache key, a checksum, a legacy
// file id, a FIPS-mode escape hatch. safe/weak-hash is a finding about the
// algorithm, so a literal argument says nothing about it and the rule carries
// no `dataSink` mark; this flag is the one argument that does answer it, and
// reporting the developer who wrote it down is the finding that teaches people
// to ignore rung 1.
const NOT_FOR_SECURITY = /usedforsecurity\s*=\s*False/;

const notForSecurity = (rule, line) => rule.id === 'safe/weak-hash'
  && NOT_FOR_SECURITY.test(line);

// This used to be a line rule, `\bdef\s+\w+\s*\([^)]{0,500}=\s*…`, whose
// parameter span was bounded to 500 characters because an unbounded `[^)]*` is
// retried from every `def` on the line and, with no `)` anywhere, runs to end
// of line each time — quadratic, 653ms on a 100KB line and 2739ms at 200KB.
// The bound bought that back by giving up the finding on any `def` with more
// than 500 characters of parameters ahead of the mutable one, and a signature
// that long is exactly where a stray `[]` hides. It also never saw a wrapped
// `def` at all, since a line rule tests one line and black puts each parameter
// on its own.
//
// defParamText already holds the whole list, across every continuation, from
// the one paramSpans pass — so the rule costs one map walk with no ceiling on
// the list and no scan per `def`. DEF_HEAD anchors to the start of the line,
// which is where Python's grammar puts `def` regardless.
function mutableDefaultFindings(texts) {
  const findings = [];
  for (const [index, params] of texts) {
    if (!MUTABLE_DEFAULT.test(params)) continue;
    findings.push(finding({
      rung: 'TRUE', id: 'true/mutable-default', line: index + 1,
      message: 'mutable default argument — shared across calls',
      fix: 'default to None and build the value inside the function',
    }));
  }
  return findings;
}

function countDefParams(texts, start) {
  const params = texts.get(start);
  if (params === undefined) return 0;
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
function measureBlocks(lines, blocks, texts) {
  return blocks.map((block) => ({
    ...block,
    params: countDefParams(texts, block.startLine - 1),
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
  const stripped = stripNoise(String(source || ''), 'py');
  const texts = defParamText(stripped);
  const ctx = packContext({ lines, stripped: stripped.split(/\r?\n/), spec: TAINT });
  const inline = lineRuleFindings(LINE_RULES, lines, {
    skip: (rule, line) => notForSecurity(rule, line) || skipConstant(rule, line, ctx),
  });

  return [
    ...inline,
    ...taintFindings({ spec: TAINT, ctx, existing: inline }),
    ...mutableDefaultFindings(texts),
    ...exceptFindings(lines),
    ...shapeFindings({
      blocks: measureBlocks(lines, blocks, texts),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'function',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
