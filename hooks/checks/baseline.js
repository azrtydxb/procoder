#!/usr/bin/env node
// procoder — the ratchet.
//
// Without this, opening procoder on a five-year-old repo produces four thousand
// findings and gets switched off within the hour. Existing violations are
// recorded once and suppressed everywhere; only new and changed code is gated,
// and the recorded set may shrink but never gain a member.
//
// Fingerprints deliberately exclude line numbers, and identify a finding by the
// construct it points at rather than by one line's text: neither reindenting
// nor a re-wrapping formatter may resurrect a suppressed finding.

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

// What identifies a finding is the code construct it points at, not the
// characters of one line. A statement prettier splits over four lines is the
// same statement, so the hashed text is the *token sequence* of the whole
// logical statement, gathered forward from the finding's line.
//
// Tokens rather than whitespace-stripped text: stripping every space would make
// `const x` and `constx` the same construct, which is a merge, and merges are
// how a baseline silently starts suppressing code nobody accepted.
const TOKEN_RE = /[\w$]+|[^\s\w$]/g;
const SEP = '\u001f';
const CLOSERS = new Set([')', ']', '}']);

// Depth is tracked for `(` and `[` only. A line ending in `{` opens a block far
// more often than it wraps an expression, and following it would tie a finding
// on a function's signature to every line of its body.
const DEPTH = { '(': 1, '[': 1, ')': -1, ']': -1 };

// Operators a wrapping formatter leaves at the end of a line when it breaks a
// long assignment or concatenation, plus the C/shell line-continuation. `?`,
// `:`, `.` and `,` are deliberately absent: prettier moves those to the START
// of the continuation line, and a trailing `:` is a Python block header whose
// body must not be swallowed.
const CONTINUES = new Set(['+', '-', '*', '/', '%', '=', '&', '|', '^', '\\']);

// A statement longer than this stops being one construct, and a stray unclosed
// bracket must not make one finding's identity depend on half the file.
const WINDOW_MAX_LINES = 10;

function tokensOf(text) {
  return String(text === undefined || text === null ? '' : text).match(TOKEN_RE) || [];
}

// A formatter that moves a closer onto its own line also adds a comma before
// it (prettier's and black's defaults). That comma is punctuation the author
// never wrote, so it cannot be part of identity.
function normalizeLine(text) {
  const toks = tokensOf(text);
  // Joined on a unit separator, which no token can contain: concatenating them
  // outright would make `const x` and `constx` the same string.
  return toks.filter((t, i) => !(t === ',' && CLOSERS.has(toks[i + 1]))).join(SEP);
}

// The source slice a finding points at: its own line, extended forward while
// the construct is still open — an unbalanced `(`/`[`, or a trailing operator —
// and never past WINDOW_MAX_LINES. A statement already on one line is balanced
// and yields exactly that line, so the flat and the wrapped form agree.
function statementAt(lines, index) {
  const all = Array.isArray(lines) ? lines : [];
  let toks = tokensOf(all[index]);
  let depth = toks.reduce((d, t) => d + (DEPTH[t] || 0), 0);
  let last = index;
  while (last + 1 < all.length && last - index + 1 < WINDOW_MAX_LINES) {
    if (depth <= 0 && !CONTINUES.has(toks[toks.length - 1])) break;
    last += 1;
    const next = tokensOf(all[last]);
    depth += next.reduce((d, t) => d + (DEPTH[t] || 0), 0);
    toks = toks.concat(next);
  }
  return all.slice(index, last + 1).join('\n');
}

function fingerprint(finding, relPath, sourceLine, ordinal = 0) {
  const normalizedPath = String(relPath).replace(/\\/g, '/');
  return crypto.createHash('sha1')
    .update(`${finding.id}\0${normalizedPath}\0${normalizeLine(sourceLine)}\0${ordinal}`)
    .digest('hex');
}

// One fingerprint per finding, in order. Identical statements share id, path
// and normalized content, so each gets the ordinal of its occurrence: without
// it one accepted violation would accept fifty copies of itself, and
// copy-paste is exactly how legacy code grows. The ordinal survives
// reformatting too — wrapping and reindenting do not reorder findings.
//
// The path stays in the fingerprint. A violation pasted into a different file
// is a different violation: dropping the path would let one baselined line
// license unlimited copies across the tree, which is the same hole the ordinal
// closed, and the ordinal itself is only meaningful within one file.
function fingerprintsFor(findings, relPath, lines) {
  const seen = new Map();
  return findings.map((f) => {
    const statement = statementAt(lines, f.line - 1);
    const key = `${f.id}\0${normalizeLine(statement)}`;
    const ordinal = seen.get(key) || 0;
    seen.set(key, ordinal + 1);
    return fingerprint(f, relPath, statement, ordinal);
  });
}

function baselinePath(repoRoot, config) {
  return path.join(repoRoot, (config.baseline && config.baseline.file) || '.procoder-baseline.json');
}

// v2 added the occurrence ordinal to every fingerprint; v3 moved identity from
// one line's text to the token sequence of the whole statement, so a formatter
// can re-wrap it. Neither change is migratable — the old file records hashes,
// not the source they were taken from — so an old file is dropped rather than
// half-honoured. Loading it as-is would suppress nothing and report a legacy
// repo's whole backlog as new, which is how the tool gets uninstalled.
const BASELINE_VERSION = 3;

// Returns a Set, always: the hook path (checks/run.js) takes it straight to
// suppress(). A stale file yields an empty Set carrying `staleVersion`, which
// the CLI reports on stderr — the library never prints, because the hook's
// stdout is a JSON protocol and its stderr is shown to the user verbatim.
function loadBaseline(repoRoot, config) {
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(baselinePath(repoRoot, config), 'utf8'));
  } catch (e) {
    return new Set();
  }
  if (parsed.version !== BASELINE_VERSION) {
    const stale = new Set();
    stale.staleVersion = parsed.version === undefined ? 'unknown' : parsed.version;
    return stale;
  }
  return new Set(Array.isArray(parsed.fingerprints) ? parsed.fingerprints : []);
}

function writeBaseline(repoRoot, config, entries) {
  const payload = {
    version: BASELINE_VERSION,
    note: 'procoder ratchet. Generated by `procoder baseline`. Shrinking is good; growth fails CI.',
    fingerprints: Array.from(new Set(entries)).sort(),
  };
  fs.writeFileSync(baselinePath(repoRoot, config), JSON.stringify(payload, null, 2) + '\n');
}

function suppress(findings, { baseline, relPath, lines }) {
  if (!baseline || baseline.size === 0) return findings;
  const fps = fingerprintsFor(findings, relPath, lines);
  return findings.filter((f, i) => !baseline.has(fps[i]));
}

// The ratchet: a finding present today that the baseline never accepted is
// growth, no matter how many old findings were fixed in the same run.
// Counting totals instead would let a new violation ride in behind an
// unrelated fix.
function growthCheck(baseline, currentFingerprints) {
  const added = [...currentFingerprints].filter((fp) => !baseline.has(fp));
  return { ok: added.length === 0, added, delta: added.length };
}

module.exports = {
  fingerprint, fingerprintsFor, loadBaseline, writeBaseline, suppress, growthCheck, baselinePath,
  BASELINE_VERSION,
};
