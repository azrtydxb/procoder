#!/usr/bin/env node
// procoder — orchestrates one file's checks.
//
// Order: exclusion → size guard → read → universal pack (always, on the raw
// source) → SAFE/TRUE rules → shape rules (unless the project's own linter
// covers them) → touched-range narrowing → baseline suppression → sort → cap.

const fs = require('fs');
const path = require('path');
const { excludeReason, isRuleExcluded, MAX_FILE_BYTES } = require('./config');
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
//
// Re-derived and kept: it is not a performance cap, it costs one Map over the
// findings, and it is the only thing standing between a generated one-liner and
// a report of 248 identical entries. A minified line still reaches that — with
// parameter counts now measured on long lines (see SHAPE_IDS), one generated
// line of 200 eight-parameter signatures produces 200 honest findings that are
// all the same sentence.
const MAX_FINDINGS_PER_LINE = 20;

// The hook runs on every write and the harness kills it at 2s, so the budget is
// real: past it the user gets a stall and no findings at all.
const BUDGET_MS = 2000;

// What a language pack costs per megabyte of source, measured on this engine
// across all six packs at 1MB: ts 187ms, java 188ms, cs 184ms, rs 169ms, go
// 165ms, py 134ms — linear in size, and the universal pack adds 12ms/MB on top.
// 200 is the worst of those rounded up, and it is used two ways: to reserve the
// pack's share of the budget from the linter below, and to derive MAX_FILE_BYTES.
const PACK_MS_PER_MB = 200;

// Total-size skip. Lives in config.js because `[limits] max_file_bytes` lets a
// project clamp it downward, and the clamp has to be applied — and warned about
// — once, where the config is read. The derivation stays here, with the numbers
// it was derived from.
//
// This came down from 4MB, which measurement did not support. The old comment
// claimed 4MB cost 122ms; measured end to end through the hook process it costs
// 896ms, and the number that matters is not the file alone but the worst
// realistic composition — the project's linter hangs and burns its slice, then
// the pack runs. That measured 1797ms of a 2000ms budget at 4MB, and the hook
// process as a whole took 2608ms, past the kill. Worse, 3MB and 4MB did not
// merely stall: one finding per line is ~157,000 findings, and the engine threw
// `Maximum call stack size exceeded` from inside the cap — which the hook's
// top-level catch turns into a silent clean result.
//
// 2MB is where the size-dependent work stays at a quarter of the budget:
// 2MB x (200 + 12)ms/MB = 424ms measured, against 2000ms. The worst realistic
// composition at 2MB measures ~1030ms — 52% of budget, so a machine would have
// to be 1.9x slower than this one to breach. At 4MB the same composition sat at
// 90% and any machine 1.15x slower breached it. 2MB is still larger than any
// file a human edits, and far above the 256KB this cap once used.
//
// The constant itself is `MAX_FILE_BYTES`, imported from config.js above.

// Line-length guard for the SPAN-derived shape metrics only — function length,
// nesting depth and complexity.
//
// Not a performance guard, and no longer a whole-shape guard. Measured with the
// guard lifted entirely, checkFile on a single line costs the same to within
// noise at every size: 4KB 0.9ms/0.9ms, 64KB 9.8/9.1, 256KB 38.4/37.7, 1MB
// 152.2/152.8, 4MB 624.8/640.0 (guarded/lifted). It buys no time at all — it
// even costs a little, because a file with a long line runs the pack twice.
//
// What it buys is silence where a measurement would be a lie. A metric derived
// from a function's line SPAN is meaningless once the function is one line:
// every function on a minified line is "1 line long", the nesting depth is the
// whole bundle's, and complexity is every branch in the file summed — measured
// output was "cyclomatic complexity ~497" repeated 248 times on an 8KB line.
//
// Parameter counts are not span-derived, so they are no longer guarded: a
// signature's parameters sit on the signature's own line whether or not the
// file was minified, and an 8-parameter function in a generated client is an
// 8-parameter function. See SHAPE_IDS.
//
// 4096 is therefore a plausibility threshold, not a cost one: past ~4KB on one
// line nobody wrote that line by hand, so its span means nothing.
//
// The universal pack has never been subject to this guard and still is not: it
// is the rung-1 path (secrets, PII in logs, rot) and measures flat — 12ms/MB,
// 9.7ms on a 4MB single line.
const MAX_LINE_BYTES = 4096;

// What the shape guard covers: the metrics a long line corrupts, which is every
// metric derived from a function's line span. obvious/too-many-params is
// deliberately absent — it counts within one signature and stays exact on a
// minified line, so it is taken from the raw source like the line rules.
const SHAPE_IDS = new Set([
  'obvious/function-too-long',
  'obvious/complexity',
  'obvious/nesting-depth',
]);

// `target.push(...items)` puts every item on the call stack. A 3MB file with
// one finding per line is ~157,000 items and V8 throws `Maximum call stack size
// exceeded` somewhere past 125,000 — which the hook's top-level catch renders
// as a clean file. Findings are unbounded until the caps below run, so every
// bulk append goes through here.
function pushAll(target, items) {
  for (const item of items) target.push(item);
  return target;
}

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
      pushAll(kept, group);
      continue;
    }
    // Ranked before slicing: what survives a cap must be the rung-1 findings,
    // never whichever pack happened to run first.
    pushAll(kept, sortFindings(group).slice(0, MAX_FINDINGS_PER_LINE));
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

// The source, or the reason there is none. Each `skipped` value is reported
// verbatim by the caller — `excluded` for .procoder.toml, `ignored:<file>` for
// the .procoderignore that did it, `too-large` and `unreadable` for the rest.
function readSource(absPath, relPath, config) {
  const excluded = excludeReason(config, relPath);
  if (excluded) return { skipped: excluded };
  // `[limits] max_file_bytes`, already clamped downward by config.js. The
  // fallback covers a hand-built config object — every caller in this repo goes
  // through loadConfig, but a missing section must never lift the cap.
  const cap = (config.limits && config.limits.max_file_bytes) || MAX_FILE_BYTES;
  try {
    if (fs.statSync(absPath).size > cap) return { skipped: 'too-large' };
    return { source: fs.readFileSync(absPath, 'utf8') };
  } catch (e) {
    return { skipped: 'unreadable' };
  }
}

// The project's own linter defines this project's shape thresholds, so its
// findings replace the pack's obvious/* rules. They never replace the pack's
// SAFE rules: rung 1 is non-negotiable, and eslint/ruff do not check for SQL
// injection, shell injection or disabled TLS verification by default.
// `answered` is false for a linter that timed out or crashed, and for no
// linter at all — both leave the pack covering the whole file.
//
// The linter gets half the budget minus the pack's share of it, because the
// linter is the one consumer of the budget that can hang and the one whose
// absence costs least: rung 1 has already run, the pack's SAFE rules run after
// this regardless, and all a timeout loses is the linter's own rung-2 findings.
// Reserving first is what keeps the total flat as the file grows — measured
// against a hung linter, a one-line file cost 1005ms and a file at the cap
// 1788ms before the reserve, and ~1030ms for both after it.
function toolResults(relPath, { repoRoot, absPath, deadline, budgetMs, bytes }) {
  const tool = resolveFor(relPath, { repoRoot });
  if (!tool || Date.now() >= deadline) return { findings: [], answered: false };
  const packReserveMs = Math.ceil((bytes / (1024 * 1024)) * PACK_MS_PER_MB);
  const result = runToolResult(tool, {
    repoRoot,
    absPath,
    timeoutMs: Math.min(1500, Math.max(250, Math.floor(budgetMs / 2) - packReserveMs)),
  });
  return { findings: result.findings, answered: result.ok };
}

// The pack's line rules see long lines. Only when the file actually has one is
// the pack run a second time over the shape copy, and the shape findings taken
// out of that second pass — which is cheap precisely because the long line is
// no longer in it.
function packResults(pack, source, shaped, options) {
  const findings = pack.check(source, options);
  if (shaped === source) return findings;
  return findings.filter((f) => !SHAPE_IDS.has(f.id))
    .concat(pack.check(shaped, options).filter((f) => SHAPE_IDS.has(f.id)));
}

// The project's linter plus the language pack: everything that answers for this
// one file's own code, and so everything the touched-range narrowing may reduce.
// The universal pack is deliberately not here — it is never narrowed.
function narrowableFindings(relPath, source, shaped, opts) {
  const { repoRoot, absPath, config, deadline, budgetMs } = opts;
  const tool = toolResults(relPath, {
    repoRoot, absPath, deadline, budgetMs, bytes: source.length,
  });
  const local = [...tool.findings];

  const pack = packFor(relPath);
  if (pack && Date.now() < deadline) {
    const packFindings = packResults(pack, source, shaped, { relPath, config });
    pushAll(local, tool.answered
      ? packFindings.filter((f) => !String(f.id).startsWith('obvious/'))
      : packFindings);
  }
  return local;
}

// Narrowed to the touched region when the caller could identify one.
function withinTouched(findings, source, touched) {
  const ranges = touched && touchedRanges(source, touched);
  if (!ranges) return findings;
  return findings.filter((f) => ranges.some(([lo, hi]) => f.line >= lo && f.line <= hi));
}

// Rule exclusions, the marked-literal filter, the per-line cap and the
// baseline, applied in that order.
function reportOf(relPath, findings, source, { repoRoot, config, applyBaseline, maxFindings }) {
  // Lines the author marked as describing a pattern rather than instancing one
  // drop out here, across every pack: a doctrine page quoting an injection sink
  // and a test asserting on one are the same problem as a test asserting on a
  // credential, and one mechanism has to cover all three. The marker names its
  // rules and reaches at most two lines — see universal.js.
  // relPath is passed so an unknown-id warning names the file. Without it this
  // pass — the one that actually runs, since the pack's own returns early when
  // the pack found nothing — printed "at line N" and no path.
  const scoped = capFindingsPerLine(filterMarkedLiterals(source, findings, relPath))
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

function checkFile(absPath, {
  repoRoot, config, maxFindings = MAX_FINDINGS, applyBaseline = true,
  touched = null, budgetMs = BUDGET_MS,
} = {}) {
  const deadline = Date.now() + budgetMs;
  const relPath = path.relative(repoRoot, absPath).replace(/\\/g, '/');
  const { source, skipped } = readSource(absPath, relPath, config);
  if (skipped) return { relPath, findings: [], skipped };

  // Two views for the pack: `shaped` drops every long line for the shape rules
  // (MAX_LINE_BYTES); the line rules and the universal pack read the source as
  // written, unguarded, because every path over a long line is linear.
  const shaped = blankLongLines(source);

  // The universal pack runs first and on the raw source: no linter checks for
  // credentials in source or PII in logs, and rung 1 must not lose its budget
  // to a slow linter subprocess further down. It is also the one pack exempt
  // from the narrowing below — a credential is a leak wherever it sits.
  const findings = checkUniversal(source, { relPath, config });

  pushAll(findings, withinTouched(
    narrowableFindings(relPath, source, shaped, { repoRoot, absPath, config, deadline, budgetMs }),
    source, touched,
  ));

  // Dependency manifests get one extra pass: a floating range or an absent
  // lockfile is a rung-1 finding no language pack looks for.
  if (MANIFEST_FILES.has(path.basename(relPath)) && Date.now() < deadline) {
    pushAll(findings, checkManifest(absPath, source));
  }

  return reportOf(relPath, findings, source, { repoRoot, config, applyBaseline, maxFindings });
}

module.exports = {
  checkFile, MAX_FINDINGS, MAX_FINDINGS_PER_LINE, MAX_FILE_BYTES, MAX_LINE_BYTES, BUDGET_MS,
};
