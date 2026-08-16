#!/usr/bin/env node
// procoder — the language-independent pack.
//
// These are the checks linters do not run: credentials in source, secrets and
// PII reaching logs, code left commented out, and deprecations with no removal
// trigger. They apply to every file type, including config and docs.
//
// The marker patterns live in ./patterns/markers.js — see the note there for
// why those literals are kept apart from the logic that uses them.

const { finding } = require('./finding');
const markers = require('./patterns/markers');

const SECRET_PATTERNS = [
  { re: /\bAKIA[0-9A-Z]{16}\b/, what: 'AWS access key id' },
  { re: /\bgh[pousr]_[A-Za-z0-9]{30,}\b/, what: 'GitHub token' },
  { re: /\bxox[baprs]-[A-Za-z0-9-]{10,}\b/, what: 'Slack token' },
  { re: /\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{10,}\b/, what: 'Stripe key' },
  { re: /-----BEGIN (?:RSA |EC |OPENSSH |PGP )?PRIVATE KEY-----/, what: 'private key' },
  { re: /\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\./, what: 'JWT' },
];

// A literal assigned to a credential-shaped name. Values that are empty, obvious
// placeholders, or reads from env/secret managers are not credentials.
const CREDENTIAL_ASSIGN =
  /\b(?:password|passwd|secret|api[_-]?key|apikey|access[_-]?token|auth[_-]?token|client[_-]?secret|private[_-]?key)\b\s*[:=]\s*["'`]([^"'`]{8,})["'`]/i;

const PLACEHOLDER = /^(?:x{3,}|\.{3,}|<[^>]+>|\{\{.*\}\}|\$\{.*\}|changeme|placeholder|example|test|dummy|redacted|your[_-]?\w+)$/i;

const FROM_SECRET_STORE = /process\.env|os\.environ|getenv|secrets?\./i;

const LOG_CALL = /\b(?:console\.(?:log|info|warn|error|debug)|logger?\.(?:log|info|warn|error|debug|trace)|log\.(?:info|warn|error|debug|trace)|print|println|printf|fmt\.Print\w*|System\.out\.print\w*)\s*\(/i;

const SECRET_WORD = /\b(?:token|password|passwd|secret|api[_-]?key|authorization|auth[_-]?header|cookie|session[_-]?id|credential|private[_-]?key)\b/i;
const PII_WORD = /\b(?:email|e-mail|ssn|social[_-]?security|phone[_-]?number|date[_-]?of[_-]?birth|dob|home[_-]?address|street[_-]?address|passport|credit[_-]?card|card[_-]?number|iban)\b/i;

// Interpolation or concatenation of a variable into the logged string — a bare
// literal mentioning the word is fine ("password reset requested").
const INTERPOLATED = /\$\{[^}]*\}|%[sdv]|\{\}|\{[a-z_][\w.]*\}|["'`]\s*[+,]\s*\w|\bf["']/i;

const COMMENT_LINE = /^\s*(?:\/\/|#|--|\*(?!\/))\s?(.*)$/;
// A commented line is CODE, not prose, when it ends in a code terminator or
// contains an assignment/call/brace — prose sentences do not. Inline code spans
// are removed first: prose that quotes a fragment of code is still prose.
const INLINE_CODE_SPAN = /`[^`]*`/g;
const LOOKS_LIKE_CODE =
  /[;{}]\s*$|^\s*(?:if|for|while|return|const|let|var|def|func|fn|class|import|from|public|private)\b|=\s*[^=]|\w+\([^)]*\)\s*[;{]?\s*$/;

// A run this long, mostly code-shaped, is a commented-out block rather than a
// paragraph of explanation that happens to mention a symbol.
const COMMENT_RUN_MIN = 3;
const CODE_COMMENTS_MIN = 2;

// Extracts the actual interpolated *expressions* — `${expr}`, an f-string/format
// `{expr}`, or the identifier chain on the other side of a `+`/`,` concatenation —
// so the secret/PII word check below tests what is being logged, not what the
// surrounding message happens to say about it.
function interpolatedExpressions(line) {
  const exprs = [];
  let m;
  const braceRe = /\$\{([^}]*)\}|(?<!\$)\{([a-z_][\w.]*)\}/gi;
  while ((m = braceRe.exec(line))) exprs.push(m[1] || m[2]);
  const concatRe = /["'`]\s*[+,]\s*([A-Za-z_][\w.]*)|([A-Za-z_][\w.]*)\s*\+\s*["'`]/g;
  while ((m = concatRe.exec(line))) exprs.push(m[1] || m[2]);
  return exprs.join(' ');
}

function secretFindings(line, lineNo) {
  const hit = SECRET_PATTERNS.find(({ re }) => re.test(line));
  if (hit) {
    return [finding({
      rung: 'SAFE', id: 'safe/hardcoded-secret', line: lineNo,
      message: `${hit.what} literal in source`,
      fix: 'read from env or a secret manager, and rotate this value — it is in git history',
    })];
  }

  const credential = CREDENTIAL_ASSIGN.exec(line);
  if (!credential || PLACEHOLDER.test(credential[1]) || FROM_SECRET_STORE.test(line)) return [];

  return [finding({
    rung: 'SAFE', id: 'safe/hardcoded-secret', line: lineNo,
    message: 'credential assigned a literal value',
    fix: 'read from env or a secret manager; fail loudly at startup if absent',
  })];
}

function logLeakFindings(line, lineNo) {
  if (!LOG_CALL.test(line) || !INTERPOLATED.test(line)) return [];

  const interpolated = interpolatedExpressions(line);
  if (SECRET_WORD.test(interpolated)) {
    return [finding({
      rung: 'SAFE', id: 'safe/secret-in-log', line: lineNo,
      message: 'credential interpolated into a log call',
      fix: 'log a correlation id instead; never the credential',
    })];
  }
  if (PII_WORD.test(interpolated)) {
    return [finding({
      rung: 'SAFE', id: 'safe/pii-in-log', line: lineNo,
      message: 'PII interpolated into a log call',
      fix: 'redact or hash the field, or log the record id only',
    })];
  }
  return [];
}

function suppressionFindings(line, lineNo) {
  if (!markers.SUPPRESSION.test(line)) return [];

  if (markers.SUPPRESSION_BLANKET.test(line) || !markers.SUPPRESSION_NAMED.test(line)) {
    return [finding({ ...markers.BLANKET_SUPPRESSION_FINDING, line: lineNo })];
  }

  // Strip the rule-naming portion, then anything substantive left over — in any
  // separator the ecosystem happens to use — counts as a reason.
  const named = markers.SUPPRESSION_NAMED.exec(line)[0];
  const rest = line.slice(line.indexOf(named) + named.length);
  if (markers.SUPPRESSION_REASON.test(rest)) return [];

  return [finding({ ...markers.UNEXPLAINED_SUPPRESSION_FINDING, line: lineNo })];
}

function markerFindings(line, lineNo) {
  const findings = [];

  if (markers.ORPHAN_MARKER.test(line) && !markers.OWNED_MARKER.test(line)) {
    findings.push(finding({ ...markers.ORPHAN_MARKER_FINDING, line: lineNo }));
  }

  findings.push(...suppressionFindings(line, lineNo));

  if (markers.DEPRECATION_MARK.test(line) && !markers.REMOVAL_TRIGGER.test(line)) {
    findings.push(finding({ ...markers.STALE_DEPRECATION_FINDING, line: lineNo }));
  }

  return findings;
}

// Walks the file's comment runs, reporting each one that is mostly code. A run
// ends at the first non-comment line, or at end of file.
function commentedCodeFindings(lines) {
  const findings = [];
  let run = 0;
  let runStart = 0;
  let codeComments = 0;

  const close = () => {
    if (run >= COMMENT_RUN_MIN && codeComments >= CODE_COMMENTS_MIN) {
      findings.push(finding({
        rung: 'ALONE', id: 'alone/commented-code', line: runStart,
        message: `${run} lines of commented-out code`,
        fix: 'delete it — version control remembers',
      }));
    }
    run = 0;
    codeComments = 0;
  };

  lines.forEach((line, index) => {
    const comment = COMMENT_LINE.exec(line);
    if (!comment) {
      close();
      return;
    }
    if (run === 0) runStart = index + 1;
    run += 1;
    if (LOOKS_LIKE_CODE.test(comment[1].replace(INLINE_CODE_SPAN, ''))) codeComments += 1;
  });

  close();
  return findings;
}

function checkUniversal(source, { relPath, config } = {}) {
  const lines = String(source || '').split(/\r?\n/);
  const findings = [];

  lines.forEach((line, index) => {
    const lineNo = index + 1;
    findings.push(
      ...secretFindings(line, lineNo),
      ...logLeakFindings(line, lineNo),
      ...markerFindings(line, lineNo));
  });

  findings.push(...commentedCodeFindings(lines));
  return findings;
}

module.exports = { checkUniversal };
