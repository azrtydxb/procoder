#!/usr/bin/env node
// procoder — Java / Kotlin pack.

const { finding } = require('../finding');
const { stripComments } = require('./comments');
const { spanRuleFindings } = require('./spans');
const { CONCAT, taintFindings } = require('./taint');
const {
  analyzeBraces, emptyCatchFindings, lineRuleFindings, measureFunctions,
  shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

const EXTENSIONS = ['.java', '.kt', '.kts'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /(?:executeQuery|executeUpdate|createQuery|rawQuery|execute)\s*\(\s*(?:"[^"]*"\s*\+|String\.format|\w+\s*\+)|String\.format\s*\(\s*"\s*SELECT/i,
    message: 'SQL built by concatenation or format',
    fix: 'use PreparedStatement with bound parameters',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /new\s+ObjectInputStream\s*\(|XMLDecoder\s*\(|readObject\s*\(\s*\)/,
    message: 'Java native deserialization of untrusted bytes',
    fix: 'use a data format (JSON) with an explicit schema',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /MessageDigest\.getInstance\s*\(\s*"(?:MD5|SHA-?1)"/i,
    message: 'weak hash used where a secure one is expected',
    fix: 'use SHA-256, or BCrypt/Argon2 for passwords',
  },
  {
    // The two spanning halves — a security-named assignment reaching
    // `new Random(` in the same statement, and `Math.random()` reaching a
    // security-named word in it — are in SPAN_RULES. Their `[^;]{0,500}` had
    // no delimiter within reach to terminate it.
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /TrustAllCerts|ALLOW_ALL_HOSTNAME_VERIFIER/,
    message: 'TLS certificate or hostname verification disabled',
    fix: 'trust the proper CA instead of accepting all certificates',
  },
  {
    id: 'true/printstacktrace', rung: 'TRUE',
    re: /\.printStackTrace\s*\(\s*\)/,
    message: 'exception printed to stderr instead of handled',
    fix: 'log with context through the project logger, then rethrow or handle',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /System\.(?:out|err)\.print(?:ln|f)?\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

// Local taint (taint.js): the assign-then-use form of the two rules above.
// `String q = "SELECT ... id=" + id;` then `stmt.executeQuery(q);` is the
// shape a Java codebase actually has, and the line rule cannot see it.
//
// The assignment pattern carries an optional type, so both `String q = …` and
// a plain `q = …` reassignment are read; Kotlin's `val`/`var` sit in the same
// modifier list. The shell sink is pinned to `getRuntime().exec(` rather than
// a bare `exec(`, which is far too common a method name to key on.
const JVM_NAME = String.raw`([A-Za-z_$][\w$]*)`;

const TAINT = {
  assign: /^\s*(?:(?:final|val|var|public|private|protected|static)\s+)*(?:[\w$<>[\],.?]+\s+)?([A-Za-z_$][\w$]*)\s*=(?![=>])/,
  sources: [/\bString\.format\s*\(/, /"[^"\n]*\$[A-Za-z_{]/, ...CONCAT],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: new RegExp(String.raw`\b(?:executeQuery|executeUpdate|createQuery|rawQuery|execute)\s*\(\s*${JVM_NAME}\s*[,)]`),
      message: 'SQL built by concatenation or format reaches a statement',
      fix: 'use PreparedStatement with bound parameters',
    },
    {
      id: 'safe/shell-injection',
      re: new RegExp(String.raw`getRuntime\s*\(\s*\)\s*\.\s*exec\s*\(\s*${JVM_NAME}\s*[,)]`),
      message: 'shell command built by concatenation or format',
      fix: 'call the binary directly with a separate argument list',
    },
  ],
};

// Rules whose needle may sit anywhere inside the same call or the same
// statement — see spans.js. Every one of these carried a 500-character
// ceiling: an exec whose concatenation sat further than that from the call, a
// ProcessBuilder with a long argument between "sh" and "-c", a
// checkServerTrusted with a long parameter list, and a token assignment with a
// long expression before `new Random(` were all silently unreported.
const SPAN_RULES = [
  {
    id: 'safe/shell-injection', rung: 'SAFE', within: 'call',
    anchor: /Runtime\.getRuntime\(\)\.exec\s*\(/g,
    needles: [/\+/g],
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with a separate argument list',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE', within: 'call',
    anchor: /new\s+ProcessBuilder\s*\(/g,
    needles: [/"(?:sh|bash|cmd(?:\.exe)?|powershell)"/g, /"(?:-c|\/c)"/g],
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with a separate argument list',
  },
  {
    // The needle list is empty: what makes this a finding is not something
    // inside the parameter list but the empty block right after it, which
    // `after` tests where the list closes.
    id: 'safe/tls-disabled', rung: 'SAFE', within: 'call',
    anchor: /checkServerTrusted\s*\(/g,
    needles: [],
    after: /\s*\{\s*\}/y,
    message: 'TLS certificate or hostname verification disabled',
    fix: 'trust the proper CA instead of accepting all certificates',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE', within: 'statement',
    anchor: /\b(?:token|secret|key|nonce|salt|session)\w*\s*=/gi,
    needles: [/new\s+Random\s*\(/g],
    message: 'java.util.Random used for a security value',
    fix: 'use SecureRandom',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE', within: 'statement',
    anchor: /Math\.random\s*\(\s*\)/g,
    needles: [/\b(?:token|key|secret)\b/gi],
    message: 'java.util.Random used for a security value',
    fix: 'use SecureRandom',
  },
];

// XML factories created without hardening are the classic XXE hole, but the
// hardening call almost always lands on the next line or two, not the same
// one — a purely single-line rule would fire on every correctly-hardened
// factory and get the whole pack turned off. Bounded lookahead (a handful of
// lines) instead of "rest of file" keeps that cheap and keeps it honest:
// hardening far outside the construction site does not excuse a bare one.
const XXE_FACTORY = /(?:DocumentBuilderFactory|SAXParserFactory|XMLInputFactory)\.newInstance\s*\(/;
const XXE_HARDENED = /disallow-doctype-decl|setExpandEntityReferences\s*\(\s*false\s*\)|FEATURE_SECURE_PROCESSING|XMLConstants\.ACCESS_EXTERNAL/;
const XXE_LOOKAHEAD = 4;

const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;
// Anchored to a single line, and with `\s` allowed only inside a generic
// argument list — `Map<String, List<Integer>>` is idiomatic and must be
// measured, while a bare `\s` in the return-type class would let
// `else if (x) {` read as TYPE NAME(params). A signature must be TYPE,
// whitespace, NAME, params, then
// `{` all on one line, so a bare `if (...) {` or `catch (...) {` — which has
// only one identifier ahead of the parens — can never match. This also
// avoids the catastrophic backtracking a multi-line version of this pattern
// hits on ordinary class bodies: letting the return-type class span
// newlines (via \s) turns "no method starts here" into an exponential
// search over every blank-line/brace combination in the file.
// Split at the parameter list's `(`: shape.js counts the parameters between it
// and its matching `)` — the one that closes the list, not the first one, so a
// default or a nested call inside a parameter no longer ends it early — and the
// tail carries on through `throws` to the block-opening brace.
const METHOD_SIGNATURE_LINE = {
  head: /^\s*(?:(?:public|private|protected|internal|static|final|abstract|synchronized|native|strictfp|fun)\s+)*[\w<>[\],.]+(?:\s*<[\w\s<>[\],.]*>)?\s+\w+\s*\(/,
  tail: /\s*(?:throws\s+[\w,.\s]+)?\{\s*$/,
};

// An XML factory is only a finding when nothing nearby hardens it.
function xxeFindings(lines) {
  const findings = [];
  lines.forEach((line, index) => {
    if (!XXE_FACTORY.test(line)) return;
    const nearby = lines.slice(index, index + 1 + XXE_LOOKAHEAD).join('\n');
    if (XXE_HARDENED.test(nearby)) return;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/xxe-risk', line: index + 1,
      message: 'XML parser created without external entities disabled',
      fix: 'setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)',
    }));
  });
  return findings;
}

// Every rule here matches code, never prose: a comment naming
// `new ObjectInputStream(...)` documents the hole, it does not open one. The
// XXE hardening lookahead reads code too, so a commented-out setFeature call
// cannot discharge a bare factory. String literals stay — `"SELECT " + id`
// *is* the SQL. See comments.js for the principle the six packs share.
function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const code = stripComments(text, 'c');
  const lines = code.split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  const inline = lineRuleFindings(LINE_RULES, lines);

  return [
    ...inline,
    ...spanRuleFindings(SPAN_RULES, lines, { existing: inline }),
    ...taintFindings({
      lines, stripped: stripped.split(/\r?\n/), spec: TAINT, existing: inline,
    }),
    ...xxeFindings(lines),
    ...emptyCatchFindings(code, SWALLOWED, 'exception swallowed by an empty catch'),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, METHOD_SIGNATURE_LINE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'method',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
