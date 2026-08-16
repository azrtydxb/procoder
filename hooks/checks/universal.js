#!/usr/bin/env node
// procoder — the language-independent pack.
//
// These are the checks linters do not run: credentials in source, secrets and
// PII reaching logs, code left commented out, and deprecations with no removal
// trigger. They apply to every file type, including config and docs.

const { finding } = require('./finding');

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

const LOG_CALL = /\b(?:console\.(?:log|info|warn|error|debug)|logger?\.(?:log|info|warn|error|debug|trace)|log\.(?:info|warn|error|debug|trace)|print|println|printf|fmt\.Print\w*|System\.out\.print\w*)\s*\(/i;

const SECRET_WORD = /\b(?:token|password|passwd|secret|api[_-]?key|authorization|auth[_-]?header|cookie|session[_-]?id|credential|private[_-]?key)\b/i;
const PII_WORD = /\b(?:email|e-mail|ssn|social[_-]?security|phone[_-]?number|date[_-]?of[_-]?birth|dob|home[_-]?address|street[_-]?address|passport|credit[_-]?card|card[_-]?number|iban)\b/i;

// Interpolation or concatenation of a variable into the logged string — a bare
// literal mentioning the word is fine ("password reset requested").
const INTERPOLATED = /\$\{[^}]*\}|%[sdv]|\{\}|\{[a-z_][\w.]*\}|["'`]\s*[+,]\s*\w|\bf["']/i;

const COMMENT_LINE = /^\s*(?:\/\/|#|--|\*(?!\/))\s?(.*)$/;
// A commented line is CODE, not prose, when it ends in a code terminator or
// contains an assignment/call/brace — prose sentences do not.
const LOOKS_LIKE_CODE =
  /[;{}]\s*$|^\s*(?:if|for|while|return|const|let|var|def|func|fn|class|import|from|public|private)\b|=\s*[^=]|\w+\([^)]*\)\s*[;{]?\s*$/;

const TODO = /\b(TODO|FIXME|HACK|XXX)\b(?!\s*[(:]?\s*(?:[A-Z]{2,}-\d+|\([^)]+\)))/;
const TODO_OWNED = /\b(?:TODO|FIXME|HACK|XXX)\b\s*(?:\([^)]+\)|[:\s]*[A-Z]{2,}-\d+)/;

// Suppressions. Silencing a tool is a claim the tool is wrong; an unnamed one also
// swallows every future finding at that location, which is how a codebase ends up
// looking clean while rotting.
const SUPPRESSION =
  /\beslint-disable(?:-next-line|-line)?\b|#\s*noqa\b|#\s*type:\s*ignore\b|\/\/\s*nolint\b|@SuppressWarnings\s*\(|#pragma\s+warning\s+disable\b|\/\/\s*@ts-(?:ignore|expect-error)\b|#\s*pylint:\s*disable\b|\/\/\s*deepcode\s+ignore\b/i;

// The rule identifier that scopes the suppression, per ecosystem.
const SUPPRESSION_NAMED =
  /eslint-disable(?:-next-line|-line)?\s+[\w@/-]+|#\s*noqa:\s*\w+|#\s*type:\s*ignore\[[^\]]+\]|\/\/\s*nolint:\s*[\w,-]+|@SuppressWarnings\s*\(\s*"(?!all")[^"]+"|#pragma\s+warning\s+disable\s+\w+|#\s*pylint:\s*disable=\s*[\w,-]+/i;

// A whole-file disable, or an explicit "everything" target.
const SUPPRESSION_BLANKET =
  /\/\*\s*eslint-disable\s*\*\/|@SuppressWarnings\s*\(\s*"all"\s*\)|\/\/\s*nolint\s*$|#\s*pylint:\s*skip-file\b/i;

// The stated reason: substantive human text after the rule identifier. Ecosystems
// spell the separator differently (`--`, `//`, `-`, `:`), or skip a separator and
// just continue with prose. What matters is that *something* beyond the rule name
// follows — so this only requires a run of word characters, anywhere in the rest
// of the line, once the rule-naming portion itself has been stripped out below.
const SUPPRESSION_REASON = /\S+\s+\S+/;

const DEPRECATED = /@?\bdeprecated\b|\bDeprecated\s*\(|#\[deprecated/i;
const REMOVAL_TRIGGER =
  /\b(?:remove|delete|drop|sunset)\b[^.\n]{0,40}\b(?:after|by|in|once|when)\b|\bv?\d+\.\d+\b|\b20\d\d-\d\d(?:-\d\d)?\b/i;

function checkUniversal(source, { relPath, config } = {}) {
  const findings = [];
  const lines = String(source || '').split(/\r?\n/);

  let commentRun = 0;
  let commentRunStart = 0;
  let codeCommentsInRun = 0;

  lines.forEach((line, index) => {
    const lineNo = index + 1;

    for (const { re, what } of SECRET_PATTERNS) {
      if (re.test(line)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/hardcoded-secret', line: lineNo,
          message: `${what} literal in source`,
          fix: 'read from env or a secret manager, and rotate this value — it is in git history',
        }));
        break;
      }
    }

    const credential = CREDENTIAL_ASSIGN.exec(line);
    if (credential && !PLACEHOLDER.test(credential[1]) && !/process\.env|os\.environ|getenv|secrets?\./i.test(line)) {
      findings.push(finding({
        rung: 'SAFE', id: 'safe/hardcoded-secret', line: lineNo,
        message: 'credential assigned a literal value',
        fix: 'read from env or a secret manager; fail loudly at startup if absent',
      }));
    }

    if (LOG_CALL.test(line) && INTERPOLATED.test(line)) {
      if (SECRET_WORD.test(line)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/secret-in-log', line: lineNo,
          message: 'credential interpolated into a log call',
          fix: 'log a correlation id instead; never the credential',
        }));
      } else if (PII_WORD.test(line)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/pii-in-log', line: lineNo,
          message: 'PII interpolated into a log call',
          fix: 'redact or hash the field, or log the record id only',
        }));
      }
    }

    const comment = COMMENT_LINE.exec(line);
    if (comment) {
      if (commentRun === 0) commentRunStart = lineNo;
      commentRun += 1;
      if (LOOKS_LIKE_CODE.test(comment[1])) codeCommentsInRun += 1;
    } else {
      if (commentRun >= 3 && codeCommentsInRun >= 2) {
        findings.push(finding({
          rung: 'ALONE', id: 'alone/commented-code', line: commentRunStart,
          message: `${commentRun} lines of commented-out code`,
          fix: 'delete it — version control remembers',
        }));
      }
      commentRun = 0;
      codeCommentsInRun = 0;
    }

    if (TODO.test(line) && !TODO_OWNED.test(line)) {
      findings.push(finding({
        rung: 'ALONE', id: 'alone/orphan-todo', line: lineNo,
        message: 'TODO with no owner or ticket',
        fix: 'add TODO(owner) or a ticket id, or do it now',
      }));
    }

    if (SUPPRESSION.test(line)) {
      if (SUPPRESSION_BLANKET.test(line) || !SUPPRESSION_NAMED.test(line)) {
        findings.push(finding({
          rung: 'ALONE', id: 'alone/blanket-suppression', line: lineNo,
          message: 'suppression names no specific rule, or disables a whole file',
          fix: 'fix the code instead; if it is genuinely a false positive, name the rule and scope it to this line',
        }));
      } else {
        // Strip the rule-naming portion, then anything substantive left over —
        // in any separator the ecosystem happens to use — counts as a reason.
        const named = SUPPRESSION_NAMED.exec(line)[0];
        const rest = line.slice(line.indexOf(named) + named.length);
        if (!SUPPRESSION_REASON.test(rest)) {
          findings.push(finding({
            rung: 'ALONE', id: 'alone/unexplained-suppression', line: lineNo,
            message: 'suppression states no reason',
            fix: 'say what makes this a false positive, on the same line',
          }));
        }
      }
    }

    if (DEPRECATED.test(line) && !REMOVAL_TRIGGER.test(line)) {
      findings.push(finding({
        rung: 'ALONE', id: 'alone/deprecated-no-trigger', line: lineNo,
        message: 'deprecation with no removal trigger',
        fix: 'add "remove after <version|date|condition>", or delete the old path now',
      }));
    }
  });

  // A comment block running to end-of-file still counts.
  if (commentRun >= 3 && codeCommentsInRun >= 2) {
    findings.push(finding({
      rung: 'ALONE', id: 'alone/commented-code', line: commentRunStart,
      message: `${commentRun} lines of commented-out code`,
      fix: 'delete it — version control remembers',
    }));
  }

  return findings;
}

module.exports = { checkUniversal };
