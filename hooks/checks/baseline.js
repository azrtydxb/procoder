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
// not the source they were taken from — so a v1 or v2 file is dropped rather
// than half-honoured. Loading it as-is would suppress nothing and report a
// legacy repo's whole backlog as new, which is how the tool gets uninstalled.
//
// v4 changes the FILE, not the fingerprint. Every entry now carries the rule it
// accepted, the path it sits in, and the date it was accepted — because a
// baseline of bare hashes is a blanket, unexplained, undated suppression, which
// is the one thing rung 4 forbids everywhere else in this codebase. Nobody
// could read their own accepted debt: `/procoder:debt` could count it and not
// name it, and no entry had an age, so nothing could ever be said to have sat
// there too long.
//
// A v3 file migrates cleanly and silently, because its fingerprints are
// computed exactly as v4 computes them: the entries load with an unknown date,
// and the first `procoder baseline` after that records real ones.
const BASELINE_VERSION = 4;

// What an entry whose date predates v4 carries. Not the epoch and not today:
// either would be a claim about when the debt was accepted, and the file does
// not say.
const UNKNOWN_DATE = 'unknown';

// Returns a Set, always: the hook path (checks/run.js) takes it straight to
// suppress(). A stale file yields an empty Set carrying `staleVersion`, which
// the CLI reports on stderr — the library never prints, because the hook's
// stdout is a JSON protocol and its stderr is shown to the user verbatim.
// A v3 file's `fingerprints` array, as v4 entries with no date. Separate from
// the loader so the migration is one named thing rather than a branch inside a
// function that is mostly about I/O.
function migrateV3(parsed) {
  return (Array.isArray(parsed.fingerprints) ? parsed.fingerprints : [])
    .map((fp) => ({ fp, id: UNKNOWN_DATE, path: UNKNOWN_DATE, added: UNKNOWN_DATE }));
}

function entriesOf(parsed) {
  if (parsed.version === 3) return migrateV3(parsed);
  return (Array.isArray(parsed.entries) ? parsed.entries : [])
    .filter((e) => e && typeof e.fp === 'string');
}

// Returns a Set of fingerprints, always: the hook path (checks/run.js) takes it
// straight to suppress(), and every caller that only asks "is this suppressed"
// keeps working unchanged. The full entries ride along on `.entries` for the
// two callers that need to NAME the debt — the aging report and the ledger.
//
// A stale file yields an empty Set carrying `staleVersion`, which the CLI
// reports on stderr; the library never prints, because the hook's stdout is a
// JSON protocol and its stderr is shown to the user verbatim.
function loadBaseline(repoRoot, config) {
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(baselinePath(repoRoot, config), 'utf8'));
  } catch (e) {
    return new Set();
  }
  if (parsed.version !== BASELINE_VERSION && parsed.version !== 3) {
    const stale = new Set();
    stale.staleVersion = parsed.version === undefined ? 'unknown' : parsed.version;
    return stale;
  }
  const entries = entriesOf(parsed);
  const set = new Set(entries.map((e) => e.fp));
  // `accepted`, not `entries`: Set.prototype.entries is a built-in method, so a
  // Set with no baseline behind it would answer that name with a function and
  // every `|| []` guard downstream would sail past it.
  set.accepted = entries;
  return set;
}

// Today, as a plain date. Not a timestamp: what the aging report asks is how
// many days a suppression has sat there, and an hour is not a unit anybody
// reasons about for technical debt.
const today = () => new Date().toISOString().slice(0, 10);

// Accepting debt again must not reset its clock. `procoder baseline` is run
// whenever something new is accepted, and if every run re-stamped every entry
// then nothing would ever age and the report below would always be empty —
// which is exactly how a "we will fix it later" list becomes permanent.
function writeBaseline(repoRoot, config, entries) {
  const existing = new Map(
    (loadBaseline(repoRoot, config).accepted || []).map((e) => [e.fp, e.added]));
  const byFingerprint = new Map();
  for (const entry of entries) {
    if (byFingerprint.has(entry.fp)) continue;
    byFingerprint.set(entry.fp, {
      fp: entry.fp,
      id: entry.id,
      path: entry.path,
      added: existing.get(entry.fp) || today(),
    });
  }
  const payload = {
    version: BASELINE_VERSION,
    note: 'procoder ratchet. Generated by `procoder baseline`. Shrinking is good; growth fails CI.',
    entries: Array.from(byFingerprint.values()).sort((a, b) => a.fp.localeCompare(b.fp)),
  };
  fs.writeFileSync(baselinePath(repoRoot, config), JSON.stringify(payload, null, 2) + '\n');
}

// Accepted findings older than `days`, oldest first. An entry with no date —
// migrated from v3 — is reported too, and says so: "we do not know when this
// was accepted" is an answer about debt, not a reason to leave it out.
function agingEntries(baseline, days, now = Date.now()) {
  const cutoff = now - days * 24 * 60 * 60 * 1000;
  return (baseline.accepted || [])
    .filter((e) => e.added === UNKNOWN_DATE || Date.parse(e.added) < cutoff)
    .sort((a, b) => String(a.added).localeCompare(String(b.added)));
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
  agingEntries, BASELINE_VERSION, UNKNOWN_DATE,
};
