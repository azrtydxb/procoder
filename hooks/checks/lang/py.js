#!/usr/bin/env node
// procoder — Python pack.

const { finding } = require('../finding');
const { stripComments } = require('./comments');
const { CONCAT, skipConstant, taintFindings } = require('./taint');
const {
  SIGNATURE_LOOKBACK,
  analyzeIndent, countParams, estimateComplexity, lineRuleFindings, shapeFindings, stripNoise,
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
    id: 'safe/sql-injection', rung: 'SAFE', dataSink: true,
    re: /\b(?:execute|executemany|raw|text)\s*\(\s*(?:f["']|["'][^"']*["']\s*%|["'][^"']*["']\s*\+|["'][^"']*["']\s*\.format\s*\()/i,
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
  assign: /^\s*([A-Za-z_][\w]*)\s*\+?=(?!=)/,
  // A `def`'s parameters, which shadow any enclosing binding of the same name.
  // Python's statements have no block-opening brace to cut them at, so the
  // generic "the list this statement ends with" would read every call's
  // arguments as a binding; `def` is the only thing here that binds.
  params: /^\s*(?:async\s+)?def\s+\w+\s*\(([^()]*)\)/,
  sources: [
    /\bf["'][^"'\n]*\{/,
    /["'][^"'\n]*["']\s*%\s*[A-Za-z_({[]/,
    /["'][^"'\n]*["']\s*\.\s*format\s*\(\s*[^)\s]/,
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

// `x=[]`, `x={}`, `x=set()` — a default built once at definition time and
// then shared by every call that does not override it.
const MUTABLE_DEFAULT = /=\s*(?:\[\s*\]|\{\s*\}|set\s*\(\s*\))/;

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
// defParams already reads the whole list, across its continuations, by
// tracking bracket depth rather than by matching a span — so the rule costs
// one forward scan per `def` line with no ceiling on either. DEF_HEAD anchors
// to the start of the line, which is where Python's grammar puts `def`
// regardless.
function mutableDefaultFindings(lines) {
  const findings = [];
  lines.forEach((line, index) => {
    if (!DEF_HEAD.test(line)) return;
    const params = defParams(lines, index);
    if (params === null || !MUTABLE_DEFAULT.test(params)) return;
    findings.push(finding({
      rung: 'TRUE', id: 'true/mutable-default', line: index + 1,
      message: 'mutable default argument — shared across calls',
      fix: 'default to None and build the value inside the function',
    }));
  });
  return findings;
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
  const inline = lineRuleFindings(LINE_RULES, lines, { skip: skipConstant });

  return [
    ...inline,
    ...taintFindings({
      lines, stripped: stripNoise(String(source || ''), 'py').split(/\r?\n/),
      spec: TAINT, existing: inline,
    }),
    ...mutableDefaultFindings(lines),
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
