#!/usr/bin/env node
// procoder — Java / Kotlin pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

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
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*=\s*[^;]*new\s+Random\s*\(|Math\.random\s*\(\s*\)[^;]*\b(?:token|key|secret)\b/i,
    message: 'java.util.Random used for a security value',
    fix: 'use SecureRandom',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /TrustAllCerts|checkServerTrusted\s*\([^)]*\)\s*\{\s*\}|ALLOW_ALL_HOSTNAME_VERIFIER/,
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
// Anchored to a single line, and with no `\s` inside the return-type
// character class: a signature must be TYPE, whitespace, NAME, params, then
// `{` all on one line, so a bare `if (...) {` or `catch (...) {` — which has
// only one identifier ahead of the parens — can never match. This also
// avoids the catastrophic backtracking a multi-line version of this pattern
// hits on ordinary class bodies: letting the return-type class span
// newlines (via \s) turns "no method starts here" into an exponential
// search over every blank-line/brace combination in the file.
const METHOD_SIGNATURE_LINE =
  /^\s*(?:(?:public|private|protected|internal|static|final|abstract|synchronized|native|strictfp|fun)\s+)*[\w<>[\],.]+\s+\w+\s*\(([^)]*)\)\s*(?:throws\s+[\w,.\s]+)?\{\s*$/;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);

  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      if (rule.re.test(line)) {
        findings.push(finding({
          rung: rule.rung, id: rule.id, line: index + 1,
          message: rule.message, fix: rule.fix,
        }));
      }
    }

    if (XXE_FACTORY.test(line)) {
      const nearby = lines.slice(index, index + 1 + XXE_LOOKAHEAD).join('\n');
      if (!XXE_HARDENED.test(nearby)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/xxe-risk', line: index + 1,
          message: 'XML parser created without external entities disabled',
          fix: 'setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)',
        }));
      }
    }
  });

  for (const match of text.matchAll(SWALLOWED)) {
    findings.push(finding({
      rung: 'TRUE', id: 'true/swallowed-error',
      line: text.slice(0, match.index).split('\n').length,
      message: 'exception swallowed by an empty catch',
      fix: 'log with context and rethrow, or handle it explicitly',
    }));
  }

  const { maxDepth, blocks } = analyzeBraces(text);
  const strippedLines = stripped.split(/\r?\n/);
  const signatures = new Map();
  strippedLines.forEach((line, index) => {
    const match = METHOD_SIGNATURE_LINE.exec(line);
    if (match) signatures.set(index + 1, match[1]);
  });

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'method',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
