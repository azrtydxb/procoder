#!/usr/bin/env node
// procoder — orchestrates one file's checks.
//
// Order: exclusion → size guard → read → SAFE/TRUE rules (always) →
// shape rules (unless the project's own linter covers them) → universal pack
// (always) → touched-range narrowing → baseline suppression → sort → cap.

const fs = require('fs');
const path = require('path');
const { isExcluded, isRuleExcluded } = require('./config');
const { packFor } = require('./registry');
const { resolveFor, runToolResult } = require('./resolve');
const { checkUniversal } = require('./universal');
const { loadBaseline, suppress } = require('./baseline');
const { sortFindings, capFindings } = require('./finding');
const { MANIFEST_FILES, checkManifest } = require('./deps');

const MAX_FINDINGS = 5;

// The hook runs on every write and the harness kills it at 2s, so the budget is
// real: past it the user gets a stall and no findings at all. A file this large
// is a bundle or a fixture, not something a human is reading; a line this long
// is minified. Both are skipped so the scanners never meet the input they are
// quadratic on.
const BUDGET_MS = 2000;
const MAX_FILE_BYTES = 256 * 1024;
const MAX_LINE_BYTES = 4096;

// Findings this many lines either side of the touched region still belong to
// the edit — a guard clause removed just above it, a brace it unbalanced.
const CONTEXT_MARGIN = 3;

// Minified content defeats the shape scanners' line-oriented assumptions and
// costs more than the whole budget. Blanked rather than dropped: line numbers
// on everything else must survive.
function blankLongLines(source) {
  if (source.length <= MAX_LINE_BYTES) return source;
  return source.split('\n').map((l) => (l.length > MAX_LINE_BYTES ? '' : l)).join('\n');
}

// The line span of each touched text, located in the file as written. A text
// that cannot be found — a formatter rewrote it, or the payload shape has no
// such field — contributes no range, and no ranges at all means whole file.
function touchedRanges(source, texts) {
  const ranges = [];
  for (const text of texts || []) {
    if (!text) continue;
    const at = source.indexOf(text);
    if (at < 0) continue;
    const start = source.slice(0, at).split('\n').length;
    ranges.push([start - CONTEXT_MARGIN, start + text.split('\n').length - 1 + CONTEXT_MARGIN]);
  }
  return ranges.length ? ranges : null;
}

function checkFile(absPath, {
  repoRoot, config, maxFindings = MAX_FINDINGS, applyBaseline = true,
  touched = null, budgetMs = BUDGET_MS,
} = {}) {
  const deadline = Date.now() + budgetMs;
  const relPath = path.relative(repoRoot, absPath).replace(/\\/g, '/');

  if (isExcluded(config, relPath)) {
    return { relPath, findings: [], skipped: 'excluded' };
  }

  let source;
  try {
    if (fs.statSync(absPath).size > MAX_FILE_BYTES) {
      return { relPath, findings: [], skipped: 'too-large' };
    }
    source = fs.readFileSync(absPath, 'utf8');
  } catch (e) {
    return { relPath, findings: [], skipped: 'unreadable' };
  }

  const scanned = blankLongLines(source);
  const findings = [];
  // Narrowed to the touched region when the caller could identify one. The
  // universal pack below is deliberately excluded: a credential committed
  // anywhere in the file is a leak regardless of which line was edited.
  const local = [];

  // The project's own linter defines this project's shape thresholds, so its
  // findings replace the pack's obvious/* rules. They never replace the pack's
  // SAFE rules: rung 1 is non-negotiable, and eslint/ruff do not check for SQL
  // injection, shell injection or disabled TLS verification by default.
  const tool = resolveFor(relPath, { repoRoot });
  let toolAnswered = false;
  if (tool) {
    const result = runToolResult(tool, {
      repoRoot, absPath, timeoutMs: Math.min(1500, Math.max(250, Math.floor(budgetMs / 2))),
    });
    local.push(...result.findings);
    toolAnswered = result.ok;
  }

  const pack = packFor(relPath);
  if (pack && Date.now() < deadline) {
    const packFindings = pack.check(scanned, { relPath, config });
    // A linter that timed out or crashed answered nothing, so the pack covers
    // the whole file rather than leaving a hole where the shape rules were.
    local.push(...(toolAnswered
      ? packFindings.filter((f) => !String(f.id).startsWith('obvious/'))
      : packFindings));
  }

  const ranges = touched && touchedRanges(source, touched);
  findings.push(...(ranges
    ? local.filter((f) => ranges.some(([lo, hi]) => f.line >= lo && f.line <= hi))
    : local));

  // The universal pack runs regardless: no linter checks for credentials in
  // source, PII in logs, or a deprecation with no removal trigger.
  findings.push(...checkUniversal(scanned, { relPath, config }));

  // Dependency manifests get one extra pass: a floating range or an absent
  // lockfile is a rung-1 finding no language pack looks for.
  if (MANIFEST_FILES.has(path.basename(relPath)) && Date.now() < deadline) {
    findings.push(...checkManifest(absPath, source));
  }

  const scoped = findings.filter((f) => !isRuleExcluded(config, relPath, f.id));

  const lines = source.split(/\r?\n/);
  const baseline = applyBaseline ? loadBaseline(repoRoot, config) : null;
  const kept = baseline ? suppress(scoped, { baseline, relPath, lines }) : scoped;

  return {
    relPath,
    findings: capFindings(sortFindings(kept), maxFindings),
    skipped: null,
    // A baseline too old to match suppresses nothing, so every caller reporting
    // these findings needs to say why the backlog appeared. The CLI has its own
    // notice; the hook uses this one.
    staleBaseline: (baseline && baseline.staleVersion) || null,
  };
}

module.exports = { checkFile, MAX_FINDINGS, MAX_FILE_BYTES, MAX_LINE_BYTES };
