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
const { checkUniversal, filterMarkedLiterals } = require('./universal');
const { loadBaseline, suppress } = require('./baseline');
const { finding, sortFindings, capFindings } = require('./finding');
const { MANIFEST_FILES, checkManifest } = require('./deps');

const MAX_FINDINGS = 5;

// Findings a single line may contribute, applied before the per-file cap above.
// One minified line is thousands of statements sharing one line number: the ts
// pack reports 26,931 findings on a 1MB minified line, and 3,000 swallowed
// errors on one line of 3,000 minified try/catch blocks. Dumped into the hook's
// context, or into `procoder check` output, that is not a report — it is a
// denial of service against the real finding.
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

// Line-length guard for the SHAPE path only — function length, nesting depth
// and complexity.
//
// Cost: nothing real. Every function on a minified line starts and ends on that
// one line, so "function is 1 line" and the brace depth of a whole bundle are
// noise, not measurements of code a human wrote.
//
// It is no longer a performance guard: the paths that were quadratic in line
// length are linear now, so the packs are cheap on a minified line — checkFile
// end to end costs 7ms at 100KB, 23ms at 500KB and 42ms at 1MB against a 2s
// budget, against 1ms/1ms/2ms when the guard blanked the line and found nothing.
// That is why the language packs' line rules now read it: safe/sql-injection,
// safe/xss-sink, safe/dynamic-eval,
// safe/shell-injection and safe/tls-disabled were invisible on any line over
// 4KB, and a minified bundle, a generated client or a vendored file is exactly
// where an injection sink hides.
//
// The universal pack has never been subject to this guard and still is not: it
// is the rung-1 path (secrets, PII in logs, rot) and measures flat — 12ms on a
// 5MB single line.
const MAX_LINE_BYTES = 4096;

// What the shape guard covers: everything shapeFindings emits. The rest of a
// pack's output is line-oriented and reads the raw source.
const SHAPE_IDS = new Set([
  'obvious/function-too-long',
  'obvious/too-many-params',
  'obvious/complexity',
  'obvious/nesting-depth',
]);

// Findings this many lines either side of the touched region still belong to
// the edit — a guard clause removed just above it, a brace it unbalanced.
const CONTEXT_MARGIN = 3;

// Minified content defeats the shape scanners' line-oriented assumptions.
// Blanked rather than dropped: line numbers on everything else must survive.
function blankLongLines(source) {
  if (source.length <= MAX_LINE_BYTES) return source;
  return source.split('\n').map((l) => (l.length > MAX_LINE_BYTES ? '' : l)).join('\n');
}

// Keeps at most MAX_FINDINGS_PER_LINE findings per line, most severe first, and
// says so where it cuts. Silent truncation is the failure mode this project has
// repeatedly punished, so the overflow is reported as a finding of its own —
// that is the only channel the hook renders.
//
// It composes with the per-file cap by running first and per line: a
// pathological line contributes at most 21 findings instead of thousands, so
// what the file cap then sorts is a bounded pool where every other line is
// represented. Which of that pool survives 5 slots is still the file cap's
// rung ordering — SAFE first — not the order the packs happened to run in.
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

  // Two views of the source for the pack: `shaped` drops every long line for the
  // shape rules (MAX_LINE_BYTES); the line rules and the universal pack read the
  // source as written, unguarded, because every path over a long line is linear.
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
    // The pack's line rules see long lines. Only when the file actually has one
    // is the pack run a second time over the shape copy, to take the shape
    // findings from there — that pass is cheap precisely because the long line
    // is gone from it.
    let packFindings = pack.check(source, { relPath, config });
    if (shaped !== source) {
      packFindings = packFindings.filter((f) => !SHAPE_IDS.has(f.id))
        .concat(pack.check(shaped, { relPath, config }).filter((f) => SHAPE_IDS.has(f.id)));
    }
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

  // Lines the author marked as describing a pattern rather than instancing one
  // drop out here, across every pack: a doctrine page quoting an injection sink
  // and a test asserting on one are the same problem as a test asserting on a
  // credential, and one mechanism has to cover all three. The marker names its
  // rules and reaches at most two lines — see universal.js.
  const scoped = capFindingsPerLine(filterMarkedLiterals(source, findings))
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
