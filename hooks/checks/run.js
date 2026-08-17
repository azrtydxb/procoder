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

// The share of what is LEFT of the budget that the project's linter may take,
// computed at the moment the linter is invoked rather than split off in
// advance.
//
// This replaces `budgetMs/2 - 200ms/MB`, and the constant it replaces is the
// whole reason CI was red. 200ms/MB was this laptop's pack throughput; on a
// runner 2-3x slower the pack needed more than that reserve predicted, and the
// linter — whose timeout had already been fixed from the same constant — had no
// way to give any of it back. The composition then ran to 1794ms of a 2000ms
// budget where the same code measured 1332ms here. A constant derived by
// measurement only holds the budget on the machine that measured it.
//
// A share of the remainder holds on any machine, because it is arithmetic
// rather than a prediction: everything in-process has already been spent when
// this is computed, so total <= spent + SHARE * (budget - spent) < budget for
// any spent < budget, whatever the host's speed. A slow pack shrinks the
// linter's slice instead of overrunning after it.
//
// The share is below 1 because work remains after the linter returns — the
// marked-literal filter, the per-line cap, the sort, the baseline. That tail
// scales with the same host slowdown as everything else, which is exactly why
// its reserve is a fraction of what is left and not a millisecond constant.
// 0.4 of the remainder covers a tail measured at 30-90ms here on a file at the
// cap, with room for a host several times slower.
const LINTER_BUDGET_SHARE = 0.6;

// Below this there is no point spawning at all: no linter starts, reads a file
// and reports inside it, so the slice would buy nothing and the fork would cost
// real time. Not a throughput number — process startup, on any host, is tens of
// milliseconds before the tool's own runtime boots.
const LINTER_MIN_MS = 100;

// Total-size skip. Lives in config.js because `[limits] max_file_bytes` lets a
// project clamp it downward, and the clamp has to be applied — and warned about
// — once, where the config is read. The derivation stays here, with the numbers
// it was derived from.
//
// It came down from 4MB, which measurement did not support: 3MB and 4MB did not
// merely stall, they threw `Maximum call stack size exceeded` from inside the
// cap — one finding per line is ~157,000 findings — which the hook's top-level
// catch turns into a silent clean result.
//
// It has now come down again, from 2MB to 1MB, and the reason is the point of
// this whole file: the language pack is the one stage of a check that cannot be
// abandoned part-way. Everything after it is deadline-driven and therefore
// host-independent (see LINTER_BUDGET_SHARE), but the pack is a single
// synchronous call, so the only lever on its cost is the size of what it is
// given. That makes this cap the one number that still has to be derived by
// measurement — and a number derived by measurement must state the slowdown it
// survives, or it is only true on the machine that measured it.
//
// Measured here at 1MB of ordinary source, worst pack of the six: ts 388ms,
// java 188ms, cs 188ms, go 202ms, rs 172ms, py 91ms, plus ~10ms for the
// universal pack. So 1MB costs ~400ms of a 2000ms budget on this machine, and
// the stated guarantee is that a host THREE TIMES SLOWER still finishes: 1.2s
// of pack, leaving ~800ms from which the linter takes its share and the tail
// runs. At the old 2MB the same file cost ~780ms here and 2.3s at 3x — over
// budget before the linter was even reached, which no scheduling scheme can
// repair.
//
// (388ms/MB is itself up from the 187ms the previous derivation used, because
// the taint work roughly doubled the ts pack's constant. That is the second
// reason to state a slowdown factor: the factor absorbs the engine getting
// slower as well as the host being slower, and a `[limits] max_file_bytes`
// clamp in .procoder.toml is the knob for a project that needs more headroom
// than 3x on hardware nobody here has measured.)
//
// Bytes are an imperfect proxy and knowingly so: most pack rules are per LINE,
// so cost tracks line count as much as size. 1MB of newlines costs the java
// pack 1265ms against 188ms for 1MB of code. No file a human edits looks like
// that, a generated one that does is what `[exclude] paths` and .procoderignore
// are for, and the deadline reporting below is what makes the overrun loud
// rather than silent if one ever arrives.
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
// The linter goes LAST and is paid out of what is actually left, because it is
// the one consumer of the budget that can hang and the one whose absence costs
// least: rung 1 has already run, the pack's SAFE rules have already run, and a
// timeout loses only the linter's own rung-2 findings. Anything it cannot be
// given is named in `unchecked` and reported — see checkFile.
function toolResults(relPath, { repoRoot, absPath, deadline, unchecked }) {
  const tool = resolveFor(relPath, { repoRoot });
  if (!tool) return { findings: [], answered: false };
  const timeoutMs = Math.floor((deadline - Date.now()) * LINTER_BUDGET_SHARE);
  if (timeoutMs < LINTER_MIN_MS) {
    unchecked.push(`${tool.name} (no budget left for it)`);
    return { findings: [], answered: false };
  }
  const result = runToolResult(tool, { repoRoot, absPath, timeoutMs });
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
//
// The pack runs BEFORE the linter, which is the other half of the fix. The pack
// is in-process, deterministic and bounded by MAX_FILE_BYTES; the linter is a
// subprocess that can hang for as long as it is allowed to. Spending the
// bounded work first means the linter's slice is computed from the time that is
// really left on this machine, instead of from a guess at what the pack was
// about to cost on some other one.
function narrowableFindings(relPath, source, shaped, opts) {
  const { repoRoot, absPath, config, deadline, unchecked } = opts;
  const local = [];

  const pack = packFor(relPath);
  let packFindings = null;
  if (pack) {
    if (Date.now() < deadline) {
      packFindings = packResults(pack, source, shaped, { relPath, config });
    } else {
      unchecked.push(`the ${path.extname(relPath).replace('.', '') || 'language'} rules`);
    }
  }

  const tool = toolResults(relPath, { repoRoot, absPath, deadline, unchecked });
  pushAll(local, tool.findings);
  if (packFindings) {
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

// What the budget cost us, said out loud. A stage the deadline cut is coverage
// the caller did not get, and coverage silently not taken is the failure this
// project has fixed nine times — so it arrives on both channels the caller has:
//
//   - as a finding, which is the only thing the hook renders into the model's
//     context, and which is appended AFTER the per-file cap so that five SAFE
//     findings can never push it out. A notice that survives only when there
//     was nothing else to report is not a notice.
//   - on stderr, which reaches the transcript whatever the caller does with the
//     finding list, and matches how a skipped file already announces itself.
//
// Never throws: `finding` only throws on an unknown rung, and this one is
// literal.
function reportUnchecked(report, unchecked, budgetMs) {
  if (unchecked.length === 0) return report;
  const what = unchecked.join(', ');
  process.stderr.write(
    `procoder: ${report.relPath}: ${budgetMs}ms budget exhausted — not checked: ${what}\n`);
  report.findings = report.findings.concat(finding({
    rung: 'TRUE',
    id: 'true/budget-exhausted',
    line: 1,
    message: `only partly checked: the ${budgetMs}ms budget ran out before ${what}`,
    fix: 'split the file, or exclude it if it is generated — its findings are unknown, not absent',
  }));
  return report;
}

function checkFile(absPath, {
  repoRoot, config, maxFindings = MAX_FINDINGS, applyBaseline = true,
  touched = null, budgetMs = BUDGET_MS,
} = {}) {
  const deadline = Date.now() + budgetMs;
  // Every stage the deadline cut, in the order it would have run. Threaded
  // rather than returned, because the stages that get cut are three levels down
  // and each one is the only place that knows what it was about to do.
  const unchecked = [];
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
    narrowableFindings(relPath, source, shaped, { repoRoot, absPath, config, deadline, unchecked }),
    source, touched,
  ));

  // Dependency manifests get one extra pass: a floating range or an absent
  // lockfile is a rung-1 finding no language pack looks for.
  if (MANIFEST_FILES.has(path.basename(relPath))) {
    if (Date.now() < deadline) pushAll(findings, checkManifest(absPath, source));
    else unchecked.push('the dependency manifest rules');
  }

  return reportUnchecked(
    reportOf(relPath, findings, source, { repoRoot, config, applyBaseline, maxFindings }),
    unchecked, budgetMs);
}

module.exports = {
  checkFile, MAX_FINDINGS, MAX_FINDINGS_PER_LINE, MAX_FILE_BYTES, MAX_LINE_BYTES, BUDGET_MS,
};
