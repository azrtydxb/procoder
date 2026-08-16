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

// Every `[^x]*` span below is capped at SPAN_MAX instead of running to the end
// of the line. Unbounded, each one is quadratic in line length: an unclosed `${`
// or `(` makes the scanner walk to end-of-line from every start position, so a
// 300KB minified line cost ~36s — sixteen times the whole hook budget, which
// means no findings at all. Capped, the work per start position is constant and
// the pass is linear. 200 characters is longer than any real interpolated
// expression or argument list; a span longer than that is minified noise, and
// the remaining arms of LOOKS_LIKE_CODE still classify it.
const SPAN_MAX = 200;

// Interpolation or concatenation of a variable into the logged string — a bare
// literal mentioning the word is fine ("password reset requested").
const INTERPOLATED = new RegExp(
  `\\$\\{[^}]{0,${SPAN_MAX}}\\}|%[sdv]|\\{\\}|\\{[a-z_][\\w.]*\\}|["'\`]\\s*[+,]\\s*\\w|\\bf["']`, 'i');

const COMMENT_LINE = /^\s*(?:\/\/|#|--|\*(?!\/))\s?(.*)$/;
// A commented line is CODE, not prose, when it ends in a code terminator or
// contains a statement keyword, an assignment or a call — prose sentences do
// not. Inline code spans are removed first: prose that quotes a fragment of
// code is still prose.
//
// The assignment arm is anchored and demands an identifier or member
// expression on the left, because the unanchored "= followed by non-=" it
// replaced read measured prose as code: "1MB = 34ms" matched, so a why-comment
// recording the numbers behind a threshold — exactly what rung 3 asks an
// author to write — was reported as commented-out code by rung 4. Identifiers
// cannot start with a digit, which is what separates a unit from a variable.
const INLINE_CODE_SPAN = /`[^`]*`/g;
const IDENT = '[A-Za-z_$][\\w$]*';
const ASSIGN_TARGET = `${IDENT}(?:\\s*\\.\\s*${IDENT}|\\s*\\[[^\\]]{0,40}\\])*`;
const LOOKS_LIKE_CODE = new RegExp(
  '[;{}]\\s*$' +
  '|^\\s*(?:if|for|while|return|const|let|var|def|func|fn|class|import|from|public|private)\\b' +
  `|^\\s*${ASSIGN_TARGET}\\s*(?:[-+*/%|&^]|\\?\\?|\\|\\||&&|<<|>>)?=[^=]` +
  // `\w` rather than `\w+`: for a boolean test the two are equivalent (any
  // match of `\w+\(` has one starting at its last word character), but the
  // greedy `\w+` re-walked the whole line from every position.
  `|\\w\\([^)]{0,${SPAN_MAX}}\\)\\s*[;{]?\\s*$`);

// A run this long, mostly code-shaped, is a commented-out block rather than a
// paragraph of explanation that happens to mention a symbol. "Mostly" is a
// majority of the run, not a fixed two lines: a long explanation with one
// code-shaped sentence in it is still an explanation.
const COMMENT_RUN_MIN = 3;
const CODE_COMMENTS_MIN = 2;

// Extracts the actual interpolated *expressions* — `${expr}`, an f-string/format
// `{expr}`, or the identifier chain on the other side of a `+`/`,` concatenation —
// so the secret/PII word check below tests what is being logged, not what the
// surrounding message happens to say about it.
function interpolatedExpressions(line) {
  const exprs = [];
  let m;
  const braceRe = new RegExp(`\\$\\{([^}]{0,${SPAN_MAX}})\\}|(?<!\\$)\\{([a-z_][\\w.]*)\\}`, 'gi');
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
    if (run >= COMMENT_RUN_MIN && codeComments >= CODE_COMMENTS_MIN && codeComments * 2 >= run) {
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

// The line-level literal marker — see markers.LITERAL_MARKER for the shape and
// the reasoning. Two scopes, and only two:
//
//   trailing   `assert(ids('AKIA…'))  // procoder: literal safe/hardcoded-secret test input`
//              applies to that line. The described pattern and the marker share
//              a line, which is the common case: an assertion, a table row, a
//              config value naming a rule id.
//   standalone `// procoder: literal safe/hardcoded-secret the key below is documentation`
//              applies to the next line as well. Needed because YAML
//              frontmatter, markdown tables and fenced examples cannot carry a
//              trailing comment without changing what they mean or render.
//
// Not a block form: a block is a region an author stops reading, and the whole
// failure being fixed here is text no one re-reads. Two lines is the widest
// scope that still forces a decision per occurrence.
//
// Which rules can it silence? All of them, safe/hardcoded-secret included. A
// marker that cannot cover a credential cannot cover this project's own test
// fixtures and doctrine pages, which is where the false positives are — it
// would be a mechanism that solves none of the cases it was built for. The
// safety is not in withholding rules from it; it is that the marker must name
// the rule, must state a reason, reaches one or two lines, and is opt-in text
// an author writes in the diff, right next to the value: a line reading
// `procoder: literal safe/hardcoded-secret it is only an example` beside a live
// key is louder in review than the key. A form that could not be abused could
// not be used either.
// An id the engine cannot produce silences nothing, which is exactly what a
// typo (`alone/orphan-todos`) or a renamed id looks like — and the author is
// left believing the line is marked. So the id is checked, and an unknown one
// is reported rather than swallowed.
//
// A configured linter's own rule ids are the tool's to define and cannot be
// enumerated, so `<rung>/<tool>:<rule>` is accepted on shape. Everything
// without a colon is a built-in id and must be one of the real ones.
const KNOWN_RULE_IDS = new Set(markers.BUILTIN_RULE_IDS);
const EXTERNAL_RULE_ID = /^(?:safe|true|obvious|alone)\/[\w@.-]+:\S+$/;

// Which unknown ids have already been complained about. filterMarkedLiterals
// runs twice over a file — once for this pack's own findings, once in run.js
// over every pack's — and the same typo must not be reported twice.
const reported = new Set();

// stderr, never stdout: the PostToolUse hook's stdout carries a JSON protocol,
// and stderr is where config.js already reports an exclusion it is dropping.
function unknownRuleId(id, lineNo) {
  if (KNOWN_RULE_IDS.has(id) || EXTERNAL_RULE_ID.test(id)) return false;
  const seen = `${id}@${lineNo}`;
  if (!reported.has(seen)) {
    reported.add(seen);
    process.stderr.write(
      `procoder: unknown rule id "${id}" in the literal marker on line ${lineNo} — it suppresses nothing\n`);
  }
  return true;
}

function markedLines(lines) {
  const marked = new Map();
  lines.forEach((line, index) => {
    const m = markers.LITERAL_MARKER.exec(line);
    if (!m) return;
    const ids = m[1].split(',')
      .map((id) => id.trim())
      .filter((id) => !unknownRuleId(id, index + 1));
    const last = markers.LITERAL_MARKER_ALONE.test(line.slice(0, m.index)) ? index + 2 : index + 1;
    for (let lineNo = index + 1; lineNo <= last; lineNo += 1) {
      if (!marked.has(lineNo)) marked.set(lineNo, new Set());
      ids.forEach((id) => marked.get(lineNo).add(id));
    }
  });
  return marked;
}

// Drops the findings an author marked as descriptions. Exported because the
// marker has to reach every pack's findings, not just this one's: most of what
// a test file or a doctrine page quotes is an injection sink or a debug
// statement. checkFile applies it once over the whole set.
//
// The `indexOf` guard is what keeps this free on the files that have no marker
// at all, which is nearly all of them: no split, no regex, no allocation.
function filterMarkedLiterals(source, findings) {
  if (!findings.length || String(source).indexOf('procoder:') < 0) return findings;
  const marked = markedLines(String(source).split(/\r?\n/));
  if (!marked.size) return findings;
  return findings.filter((f) => !(marked.get(f.line) || EMPTY_SET).has(f.id));
}

const EMPTY_SET = new Set();

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
  return filterMarkedLiterals(source, findings);
}

module.exports = { checkUniversal, filterMarkedLiterals };
