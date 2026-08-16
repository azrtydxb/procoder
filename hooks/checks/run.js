#!/usr/bin/env node
// procoder — orchestrates one file's checks.
//
// Order: exclusion → size guard → read → universal pack (always, on the raw
// source) → SAFE/TRUE rules → shape rules (unless the project's own linter
// covers them) → touched-range narrowing → baseline suppression → sort → cap.

const fs = require('fs');
const path = require('path');
const { isExcluded, isRuleExcluded } = require('./config');
const { packFor } = require('./registry');
const { resolveFor, runToolResult } = require('./resolve');
const { checkUniversal } = require('./universal');
const { loadBaseline, suppress } = require('./baseline');
const { finding, sortFindings, capFindings } = require('./finding');
const { MANIFEST_FILES, checkManifest } = require('./deps');

const MAX_FINDINGS = 5;

// Findings a single line may contribute, applied before the per-file cap above.
// One minified line is thousands of statements sharing one line number: the ts
// pack alone reports 2,702 findings on a 100KB line, 13,204 on 500KB and 26,931
// on 1MB. Dumped into the hook's context or into `procoder check` output that is
// not a report, it is a denial of service against the real finding.
//
// 20 is chosen to sit just above what an ordinary line can produce: a line rule
// fires at most once per line, the widest language pack has 10 of them and the
// universal pack 5, plus the handful of shape findings that can share a start
// line — under 20 in total, so no honest line is ever capped. Anything past it
// is the same rule matching a minified line over and over.
const MAX_FINDINGS_PER_LINE = 20;

// The hook runs on every write and the harness kills it at 2s, so the budget is
// real: past it the user gets a stall and no findings at all.
const BUDGET_MS = 2000;

// Total-size skip. Cost of everything that survives the line guard below is
// linear in file size: measured end to end on many-short-line files, 1MB costs
// 34ms, 4MB costs 122ms, 8MB costs 250ms, 16MB costs 507ms. 4MB is about 6% of
// the budget and larger than any file a human edits, so that is where the skip
// sits — not 256KB, which was inherited from when a long line could blow the
// budget, and which threw away every finding on an ordinary large source.
const MAX_FILE_BYTES = 4 * 1024 * 1024;

// Line-length guard for the SHAPE path only.
//
// Cost: the language packs do not see the content of a line this long, so their
// shape and SAFE rules miss anything on it. Justification: analyzeBraces and
// measureFunctions are still quadratic in line length — measured on one minified
// line, 50KB costs 299ms, 100KB costs 1177ms, 200KB costs 4592ms and 400KB costs
// 18337ms, four times the cost per doubling. 200KB alone is more than twice the
// whole budget.
//
// The universal pack is deliberately NOT subject to this guard: it is the rung-1
// path (secrets, PII in logs, rot), its rules are line-oriented regexes with no
// length sensitivity, and it measures flat — 12ms on a 5MB single line. Blanking
// long lines for it meant a hardcoded credential on a minified or generated line
// was never reported at all, which is exactly where a leaked key hides.
//
// procoder: guard, not a fix. Drop it when analyzeBraces/measureFunctions are
// linear in line length — they slice the source per brace and per block.
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

// Keeps at most MAX_FINDINGS_PER_LINE findings per line, most severe first, and
// says so where it cuts. Silent truncation is the failure mode this project has
// repeatedly punished, so the overflow is reported as a finding of its own —
// that is the only channel the hook renders.
//
// It composes with the per-file cap by running first and per line: a pathological
// line contributes at most 21 findings to the pool the file cap then sorts, so
// findings from every other line are still reachable.
function capFindingsPerLine(findings) {
  const byLine = new Map();
  for (const f of findings) {
    if (!byLine.has(f.line)) byLine.set(f.line, []);
    byLine.get(f.line).push(f);
  }

  const kept = [];
  for (const [line, group] of byLine) {
    if (group.length <= MAX_FINDINGS_PER_LINE) {
      kept.push(...group);
      continue;
    }
    // Ranked before slicing: what survives a cap must be the rung-1 findings,
    // never whichever pack happened to run first.
    kept.push(...sortFindings(group).slice(0, MAX_FINDINGS_PER_LINE));
    const suppressed = group.length - MAX_FINDINGS_PER_LINE;
    kept.push(finding({
      rung: 'TRUE', id: 'true/findings-suppressed', line,
      message: `line ${line}: ${suppressed} further findings suppressed (cap ${MAX_FINDINGS_PER_LINE} per line)`,
      fix: 'split the line, or exclude the file if it is generated',
    }));
  }
  return kept;
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

  // Only the shape path gets the blanked copy. See MAX_LINE_BYTES above.
  const shaped = blankLongLines(source);
  const findings = [];

  // The universal pack runs first and on the raw source: no linter checks for
  // credentials in source, PII in logs, or a deprecation with no removal
  // trigger, and rung 1 must not be the stage that loses its budget to a slow
  // linter subprocess further down.
  findings.push(...checkUniversal(source, { relPath, config }));

  // Narrowed to the touched region when the caller could identify one. The
  // universal pack above is deliberately excluded: a credential committed
  // anywhere in the file is a leak regardless of which line was edited.
  const local = [];

  // The project's own linter defines this project's shape thresholds, so its
  // findings replace the pack's obvious/* rules. They never replace the pack's
  // SAFE rules: rung 1 is non-negotiable, and eslint/ruff do not check for SQL
  // injection, shell injection or disabled TLS verification by default.
  const tool = resolveFor(relPath, { repoRoot });
  let toolAnswered = false;
  if (tool && Date.now() < deadline) {
    const result = runToolResult(tool, {
      repoRoot, absPath, timeoutMs: Math.min(1500, Math.max(250, Math.floor(budgetMs / 2))),
    });
    local.push(...result.findings);
    toolAnswered = result.ok;
  }

  const pack = packFor(relPath);
  if (pack && Date.now() < deadline) {
    const packFindings = pack.check(shaped, { relPath, config });
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

  // Dependency manifests get one extra pass: a floating range or an absent
  // lockfile is a rung-1 finding no language pack looks for.
  if (MANIFEST_FILES.has(path.basename(relPath)) && Date.now() < deadline) {
    findings.push(...checkManifest(absPath, source));
  }

  const scoped = capFindingsPerLine(findings)
    .filter((f) => !isRuleExcluded(config, relPath, f.id));

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

module.exports = {
  checkFile, MAX_FINDINGS, MAX_FINDINGS_PER_LINE, MAX_FILE_BYTES, MAX_LINE_BYTES,
};
