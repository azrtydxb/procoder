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
//
// There is no word boundary in FRONT of the credential word, and that asymmetry
// is the whole rule. With one there, only the bare spelling matched —
// `password = "..."` — and every ordinary identifier a codebase actually uses
// put a word character in front of it: `dbPassword`, `userPassword`,
// `apiSecret`, `api_secret`, `myApiKey`. The bare word is the rare form; the
// compound is the common one, so the boundary was silencing the majority case.
//
// The boundary AFTER the word stays, and it is what keeps this from becoming a
// false-positive generator on rung 1 — which is the real risk of loosening a
// credential pattern. It is what makes `passwordPolicy`, `tokenizer`,
// `keyboard`, `keys(...)` and `secretsManager.get(...)` not match: in every one
// of them the credential word is a PREFIX of a longer identifier that means
// something else. A credential-shaped name ends in the credential word — the
// noun is the head of the phrase — and reads as anything else the moment the
// name continues past it. `monkeypatch` matches nothing either way: `key` alone
// was never a credential word here, only `api_key`/`apikey`/`private_key`.
const CREDENTIAL_ASSIGN =
  /(?:password|passwd|secret|api[_-]?key|apikey|access[_-]?token|auth[_-]?token|client[_-]?secret|private[_-]?key)\b\s*[:=]\s*["'`]([^"'`]{8,})["'`]/i;

// A redaction marker written INTO a file. A security layer between the model
// and the file system replaces a credential it recognises with one of these
// before the text is handed over, so the marker means "the real file still
// holds a secret here, and you were not shown it". Writing that text back is
// the one edit that must never happen: it destroys the credential the file was
// carrying and leaves a placeholder that reads, to every later reader, like a
// value somebody chose.
const REDACTION_MARKER = /\[REDACTED[:\]]/;  // procoder: literal safe/redaction-marker the pattern the rule matches, not an instance of it

const PLACEHOLDER = /^(?:x{3,}|\.{3,}|<[^>]+>|\{\{.*\}\}|\$\{.*\}|changeme|placeholder|example|test|dummy|redacted|your[_-]?\w+)$/i;

const FROM_SECRET_STORE = /process\.env|os\.environ|getenv|secrets?\./i;

const LOG_CALL = /\b(?:console\.(?:log|info|warn|error|debug)|logger?\.(?:log|info|warn|error|debug|trace)|log\.(?:info|warn|error|debug|trace)|print|println|printf|fmt\.Print\w*|System\.out\.print\w*)\s*\(/i;

const SECRET_WORD = /\b(?:token|password|passwd|secret|api[_-]?key|authorization|auth[_-]?header|cookie|session[_-]?id|credential|private[_-]?key)\b/i;
const PII_WORD = /\b(?:email|e-mail|ssn|social[_-]?security|phone[_-]?number|date[_-]?of[_-]?birth|dob|home[_-]?address|street[_-]?address|passport|credit[_-]?card|card[_-]?number|iban)\b/i;

// Interpolation or concatenation of a variable into the logged string — a bare
// literal mentioning the word is fine ("password reset requested").
//
// The `${` arm does not require the closing brace. A regex that does — an
// unbounded `[^}]*\}` — walks to end-of-line from every `${` on the line, which
// is quadratic: a 300KB minified line cost ~36s, sixteen times the whole hook
// budget. What the brace actually delimits is read by interpolatedExpressions
// below, in one forward pass; this is only the cheap gate in front of it, and a
// `${` with no close yields no expression there, so opening the gate on the
// two characters alone adds no finding.
const INTERPOLATED =
  /\$\{|%[sdv]|\{\}|\{[a-z_][\w.]*\}|["'`]\s*[+,]\s*\w|\bf["']/i;

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
// The subscript span stays bounded at 40: an index expression genuinely is
// short, and unlike the spans below this one is inside a `*` loop, where a
// second unbounded span would multiply rather than add.
const ASSIGN_TARGET = `${IDENT}(?:\\s*\\.\\s*${IDENT}|\\s*\\[[^\\]]{0,40}\\])*`;
const LOOKS_LIKE_CODE = new RegExp(
  '[;{}]\\s*$' +
  '|^\\s*(?:if|for|while|return|const|let|var|def|func|fn|class|import|from|public|private)\\b' +
  `|^\\s*${ASSIGN_TARGET}\\s*(?:[-+*/%|&^]|\\?\\?|\\|\\||&&|<<|>>)?=[^=]`);

// The fourth arm of the above, which no regex can express without an unbounded
// `[^)]*` between the parens: the line ends in a call. That span is what the
// 200-character ceiling was capping, and capping it meant a call with a longer
// argument list simply stopped counting as code.
//
// Walked by index instead: find the closing paren the line ends on, then the
// last `)` before it, then any `(` after that with a word character in front.
// indexOf only moves forward, so the line is read a constant number of times
// however many parens it holds.
//
// Deliberately NOT bracket-matched the way shape.js's paramSpans matches a
// signature. The span must contain no `)` at all, so `f(g(x))` does not count
// — which reads like an accident of the old regex but is what keeps prose out:
// a doctrine line reading `never db.Query(fmt.Sprintf("…", id))` is a sentence
// about a call, not a call. Matching brackets properly here turns two clean
// fixtures into commented-out-code findings. Same reason the marker's reason
// runs to end of line: the cheap rule is the one that stays right.
const CALL_TAIL_END = /\)\s*[;{]?\s*$/;
function endsInCall(text) {
  const tail = CALL_TAIL_END.exec(text);
  if (!tail) return false;
  const head = text.slice(0, tail.index);
  for (let at = head.indexOf('(', head.lastIndexOf(')') + 1); at >= 0; at = head.indexOf('(', at + 1)) {
    if (at > 0 && /\w/.test(head[at - 1])) return true;
  }
  return false;
}

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
  // `${…}` by index rather than by regex, and by the same rule the regex used:
  // the span ends at the first `}`. indexOf only ever moves forward, so the
  // whole line is read once however many opens have no close — where the
  // regex, having no closing brace to stop at, re-read the tail from each one.
  for (let at = line.indexOf('${'); at >= 0; at = line.indexOf('${', at)) {
    const end = line.indexOf('}', at + 2);
    if (end < 0) break;
    exprs.push(line.slice(at + 2, end));
    at = end + 1;
  }
  const braceRe = /(?<!\$)\{([a-z_][\w.]*)\}/gi;
  while ((m = braceRe.exec(line))) exprs.push(m[1]);
  // `\b` before the identifier arm, and it is load-bearing, not decoration:
  // without it the scan restarts inside every run of word characters, and each
  // restart walks the run to its end looking for the `+` — 43s on a 200KB run
  // of them. A match cannot start mid-identifier anyway; an identifier does not
  // begin after a digit or a letter.
  const concatRe = /["'`]\s*[+,]\s*([A-Za-z_][\w.]*)|\b([A-Za-z_][\w.]*)\s*\+\s*["'`]/g;
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

function redactionFindings(line, lineNo) {
  if (!REDACTION_MARKER.test(line)) return [];
  return [finding({
    rung: 'SAFE', id: 'safe/redaction-marker', line: lineNo,
    message: 'a redaction marker written into the file',
    fix: 'restore the real value from the source of truth — the marker overwrote a credential you were never shown',
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
    const text = comment[1].replace(INLINE_CODE_SPAN, '');
    if (LOOKS_LIKE_CODE.test(text) || endsInCall(text)) codeComments += 1;
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
// Not a block form, and this was reconsidered rather than inherited. A bounded
// block — explicit start, explicit end, naming its rules and its reason —
// would let a file that is entirely violating input (a fixture) be marked
// instead of path-excluded in .procoder.toml. It is still refused. A block is
// a region an author stops reading: everything added between the two markers
// afterwards is covered by a decision nobody made, which is the blanket
// disable this project's own rung 4 forbids, only with a reason attached to
// make it feel examined. The path exclusion is the honest form of the same
// thing — it is in one file, it names the directory, it is reviewable in one
// place, and it does not travel with the code. Two lines is the widest scope
// that still forces a decision per occurrence.
//
// No wildcard, for the same reason and deliberately: the bare form routes to
// alone/blanket-suppression and is reported.
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
// A configured linter's own *rule* ids are the tool's to define and cannot be
// enumerated. Two halves of the id are checked anyway, each as far as it
// honestly can be. The tool half is a closed set: registry.js invokes four, and
// only those four ever appear on the left of the colon, always on rung TRUE, so
// `true/eslnit:no-eval` is reported like any other typo. The rule half is
// checked against that tool's own id GRAMMAR — see EXTERNAL_RULE_SHAPES — which
// catches an id pasted under the wrong tool (`true/ruff:no-eval`) without
// needing a registry of anyone's rules.
//
// What remains unvalidated, stated plainly because a user who thinks otherwise
// is back in the position this whole mechanism exists to prevent: a
// well-shaped id for a rule that does not exist. `true/eslint:no-such-rule`,
// `true/ruff:ZZ999`. See EXTERNAL_RULE_SHAPES for why guessing there would
// trade a silent no-op for a warning on correct markers.
const KNOWN_RULE_IDS = new Set(markers.BUILTIN_RULE_IDS);
const EXTERNAL_RULE_ID = /^true\/([\w-]+):(.+)$/;

function isExternalRuleId(id) {
  const m = EXTERNAL_RULE_ID.exec(id);
  const shape = m && markers.EXTERNAL_RULE_SHAPES.get(m[1]);
  return Boolean(shape && shape.test(m[2]));
}

// Which unknown ids have already been complained about, keyed by file as well
// as id and line: the same typo on the same line of two files is two mistakes
// in two places, and warning once named neither of them.
//
// Both engine passes over a file supply the path — this pack's own, and
// run.js's over every pack's findings — so they share a key and warn once
// between them. That was not always true. run.js used to call in without a
// path, and because THIS pass returns early when the universal pack found
// nothing, the path-less pass was usually the only one to run: through the CLI
// the warning therefore never named a file, and a typo on the same line of two
// files warned once for both. A diagnostic that cannot say which file to open
// is close to useless on a repository-wide run.
//
// The bare `id@line` key is still claimed alongside, for a caller that supplies
// no path at all (a direct API call). It dedups on that key as before, and a
// file-aware warning satisfies it — one warning per occurrence, never one per
// pass.
const reported = new Set();

// stderr, never stdout: the PostToolUse hook's stdout carries a JSON protocol,
// and stderr is where config.js already reports an exclusion it is dropping.
function unknownRuleId(id, lineNo, relPath) {
  if (KNOWN_RULE_IDS.has(id) || isExternalRuleId(id)) return false;
  const bare = `${id}@${lineNo}`;
  const seen = relPath ? `${relPath}|${bare}` : bare;
  if (!reported.has(seen)) {
    reported.add(seen);
    reported.add(bare);
    process.stderr.write(
      `procoder: unknown rule id "${id}" in the literal marker at ${relPath ? `${relPath}:${lineNo}` : `line ${lineNo}`} — it suppresses nothing\n`);
  }
  return true;
}

function markedLines(lines, relPath) {
  const marked = new Map();
  lines.forEach((line, index) => {
    const m = markers.LITERAL_MARKER.exec(line);
    if (!m) return;
    const ids = m[1].split(',')
      .map((id) => id.trim())
      .filter((id) => !unknownRuleId(id, index + 1, relPath));
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
// `relPath` is optional and only names the file in an unknown-id warning. Every
// caller inside the engine supplies it — run.js included, over findings from
// every pack at once — because a warning that cannot name the file cannot be
// acted on. It stays optional for a direct API call that has no path to give.
function filterMarkedLiterals(source, findings, relPath) {
  if (!findings.length || String(source).indexOf('procoder:') < 0) return findings;
  const marked = markedLines(String(source).split(/\r?\n/), relPath);
  if (!marked.size) return findings;
  return findings.filter((f) => !markedIds(marked, f).has(f.id));
}

const EMPTY_SET = new Set();

// A cross-line finding — the taint scan's — is reported at the SINK, and its
// message names the line the value was built on. Both lines are the finding:
// the sink is where the hole opens, the build line is where the string was
// assembled, and which of the two an author reads as the subject depends on
// the code. Marking the build line used to suppress nothing and say nothing,
// so an author who read the message, went to the line it named and marked it
// there got no effect and no explanation — the failure mode this whole
// mechanism exists to prevent.
//
// So the marker reaches either line. It still names its rule, still states a
// reason, and still reaches no line the finding does not already name: the
// scope is the finding's own two lines, not a range between them.
//
// The build line arrives as `sourceLine` on the finding — set by taint.js
// wherever a finding's sink and its source are different lines. It used to be
// read back out of the message text with a `built at line (\d+)` regex, which
// worked and was tested, and which would have broken silently the day anyone
// reworded the message. A field cannot be reworded.
function markedIds(marked, f) {
  const at = marked.get(f.line) || EMPTY_SET;
  const also = f.sourceLine ? marked.get(f.sourceLine) : undefined;
  return also ? new Set([...at, ...also]) : at;
}

function checkUniversal(source, { relPath, config } = {}) {
  const lines = String(source || '').split(/\r?\n/);
  const findings = [];

  lines.forEach((line, index) => {
    const lineNo = index + 1;
    findings.push(
      ...secretFindings(line, lineNo),
      ...redactionFindings(line, lineNo),
      ...logLeakFindings(line, lineNo),
      ...markerFindings(line, lineNo));
  });

  findings.push(...commentedCodeFindings(lines));
  return filterMarkedLiterals(source, findings, relPath);
}

module.exports = { checkUniversal, filterMarkedLiterals };
