#!/usr/bin/env node
// procoder — CLI for pre-commit hooks and CI. Same engine as the PostToolUse
// hook, so what fails locally fails in CI for the same reason.
//
// Usage:
//   procoder check <paths...>     exit 1 if any non-baselined blocking finding exists
//   procoder baseline <paths...>  record current findings as accepted
//   procoder verify <paths...>    exit 1 if any finding is not in the baseline

const fs = require('fs');
const path = require('path');
const { loadConfig, findRepoRoot } = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { formatFindings } = require('../hooks/checks/finding');
const { readLevel } = require('../hooks/procoder-runtime');
const {
  fingerprintsFor, writeBaseline, loadBaseline, growthCheck, baselinePath, BASELINE_VERSION,
} = require('../hooks/checks/baseline');

const USAGE = `usage: procoder <check|baseline|verify> [--unused-exclusions] <paths...>

  check     report findings not present in the baseline; exit 1 if any of them
            blocks at the active level (at pragmatic, OBVIOUS and ALONE are
            reported but do not block; every other level gates all four rungs)
  baseline  record every current finding as accepted, so only new code is gated
  verify    exit 1 if any finding present today is not in the baseline — the CI ratchet

  --unused-exclusions  (verify only) also fail if a [exclude] rules entry
                        suppressed nothing in this run — a stale suppression
                        left behind after the finding it silenced was fixed
`;

const SKIP_DIRS = new Set(['.git', 'node_modules']);

function expandDirectory(abs) {
  return fs.readdirSync(abs)
    .filter((entry) => !SKIP_DIRS.has(entry))
    .flatMap((entry) => expand([path.join(abs, entry)]));
}

function expand(targets) {
  const files = [];
  for (const target of targets) {
    const abs = path.resolve(target);
    let stat = null;
    try { stat = fs.statSync(abs); } catch (e) { stat = null; }
    if (stat === null) continue;
    files.push(...(stat.isDirectory() ? expandDirectory(abs) : [abs]));
  }
  return files;
}

// maxFindings Infinity throughout: the CLI reports and records everything,
// unlike the hook, which shows a top-5 sample inside its time budget.
function findingsFor(absPath, repoRoot, config, applyBaseline = true) {
  return checkFile(absPath, { repoRoot, config, maxFindings: Infinity, applyBaseline });
}

function runBaseline(files, repoRoot, config) {
  const entries = Array.from(loadBaseline(repoRoot, config));
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, config);
    if (skipped) continue;
    const lines = fs.readFileSync(absPath, 'utf8').split(/\r?\n/);
    entries.push(...fingerprintsFor(findings, relPath, lines));
  }
  writeBaseline(repoRoot, config, entries);
  process.stdout.write(`procoder: baseline recorded (${entries.length} accepted findings)\n`);
  return 0;
}

// Fingerprint → a human-readable location, for every finding present today.
// Findings are collected BEFORE baseline suppression, so the ratchet compares
// the full picture against what was accepted. Every occurrence gets its own
// entry — the ordinal in the fingerprint keeps identical lines apart, so
// cloning a baselined violation cannot collapse into the accepted one.
function presentFindings(files, repoRoot, config) {
  const present = new Map();
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, config, false);
    if (skipped) continue;
    const lines = fs.readFileSync(absPath, 'utf8').split(/\r?\n/);
    const fps = fingerprintsFor(findings, relPath, lines);
    findings.forEach((f, i) => {
      present.set(fps[i], `${relPath}:${f.line}  ${f.id} — ${f.message}`);
    });
  }
  return present;
}

// A rule exclusion ("path:rule-id") is real rot the moment it stops silencing
// anything — the finding it named got fixed, or the file moved on — and it
// then sits in .procoder.toml suppressing nothing, forever, unnoticed.
// Detected by re-running the same files through a config with `rules` cleared
// (paths exclusions untouched) and checking, per exclusion, whether a finding
// with its exact (path, id) still turns up. Only `rules` are checked: `paths`
// exclusions are commonly aspirational (a vendor/ that does not exist yet),
// so flagging them would just be noise. Only files the run actually covered
// can answer for an exclusion naming them — a single-file `verify` cannot
// know whether some other file's exclusion is still earning its keep, so an
// exclusion naming a path outside this run is skipped rather than guessed at.
function unusedRuleExclusions(files, repoRoot, config) {
  const rules = config.exclude.rules;
  if (rules.length === 0) return [];

  const withoutRules = { ...config, exclude: { ...config.exclude, rules: [] } };
  const coveredPaths = new Set();
  const stillFires = new Set();
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, withoutRules, false);
    if (skipped) continue;
    coveredPaths.add(relPath);
    for (const f of findings) stillFires.add(`${relPath}\0${f.id}`);
  }

  return rules.filter((r) => coveredPaths.has(r.path) && !stillFires.has(`${r.path}\0${r.id}`));
}

const SAMPLE_SIZE = 3;

// Naming a few of the new findings makes a CI failure actionable; the full
// list is what `procoder check` is for.
function sample(added, present) {
  const shown = added.slice(0, SAMPLE_SIZE).map((fp) => `  ${present.get(fp)}\n`).join('');
  const rest = added.length - SAMPLE_SIZE;
  return shown + (rest > 0 ? `  ...and ${rest} more\n` : '');
}

// Reported under plain `verify` too — a stale exclusion is rot worth seeing —
// but it only fails the build under the dedicated flag: the honest case (the
// underlying finding got fixed) must not turn into a CI failure by default.
function reportUnusedExclusions(files, repoRoot, config, unusedExclusions) {
  const stale = unusedRuleExclusions(files, repoRoot, config);
  if (stale.length === 0) return 0;
  process.stdout.write(
    `procoder: ${stale.length} exclusion rule${stale.length === 1 ? '' : 's'} suppressed nothing ` +
    'in this run:\n' + stale.map((r) => `  ${r.path}:${r.id}\n`).join('') +
    'Remove them from [exclude] rules in .procoder.toml, or note why they still apply.\n');
  return unusedExclusions ? 1 : 0;
}

// The ratchet: accepted debt may shrink, never grow. Compares fingerprints,
// not counts, so fixing an old finding buys no room for a new one.
function runVerify(files, repoRoot, config, { unusedExclusions = false } = {}) {
  const baseline = loadBaseline(repoRoot, config);
  const present = presentFindings(files, repoRoot, config);

  const { ok, added, delta } = growthCheck(baseline, present.keys());
  if (!ok) {
    process.stdout.write(
      `procoder: ${delta} finding${delta === 1 ? '' : 's'} not in the baseline ` +
      `(${baseline.size} accepted, ${present.size} present).\n` + sample(added, present) +
      'Fix them, or run `procoder baseline <paths>` only if they are genuinely pre-existing.\n');
    return 1;
  }
  process.stdout.write(
    `procoder: ${present.size} findings against a baseline of ${baseline.size} — ratchet holds.\n`);
  return reportUnusedExclusions(files, repoRoot, config, unusedExclusions);
}

// Same rule the PostToolUse hook applies: at `pragmatic` the judgment rungs
// (OBVIOUS, ALONE — the ones the severity map marks `warn`) are flagged but do
// not gate. Every other level enforces all four, and so does a missing level
// file, which readLevel resolves to the default `strict` — CI, which has no
// user-level config at all, therefore stays strict.
function isBlocking(finding, level, config) {
  if (level !== 'pragmatic') return true;
  return config.rungs[finding.rung.toLowerCase()] !== 'warn';
}

function summarize(blocking, advisory, level) {
  const total = blocking + advisory;
  const counts = `${total} finding${total === 1 ? '' : 's'}` +
    (advisory > 0 ? ` — ${blocking} blocking, ${advisory} advisory at [${level}]` : '');
  return blocking > 0
    ? `\nprocoder: ${counts}. Fix them, or run \`procoder baseline <paths>\` ` +
      'to accept pre-existing ones.\n'
    : `\nprocoder: ${counts} — advisory only at this level, not failing the run.\n`;
}

// An `excluded` skip is the config doing its job — vendor/ is excluded on
// purpose, and saying so once per file would bury the findings. `too-large` and
// `unreadable` are different: a file that should have been gated was not, and
// silence makes that indistinguishable from a clean pass. Said on stderr so it
// survives a piped stdout, and it does not fail the run — an unchecked file is
// news, not a violation.
//
// A `.procoderignore` skip is also deliberate config, but unlike .procoder.toml
// it can sit anywhere in the tree, so "which file did this and how much did it
// cover" is not something the user can see at a glance. It is counted here and
// reported once per ignore file rather than once per skipped file: the case the
// feature exists for is a large generated subtree, and a line per file would
// bury the findings it was supposed to make room for.
function reportSkip(relPath, skipped, ignored) {
  if (skipped === 'excluded') return;
  if (skipped.startsWith('ignored:')) {
    const file = skipped.slice('ignored:'.length);
    ignored.set(file, (ignored.get(file) || 0) + 1);
    return;
  }
  process.stderr.write(`procoder: skipped ${relPath} (${skipped}) — not checked.\n`);
}

function reportIgnored(ignored) {
  for (const [file, count] of ignored) {
    process.stderr.write(
      `procoder: ${count} file${count === 1 ? '' : 's'} skipped by ${file} — not checked.\n`);
  }
}

function runCheck(files, repoRoot, config) {
  const level = readLevel();
  const ignored = new Map();
  let blocking = 0;
  let advisory = 0;
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, config);
    if (skipped) { reportSkip(relPath, skipped, ignored); continue; }
    if (findings.length === 0) continue;
    const gating = findings.filter((f) => isBlocking(f, level, config)).length;
    blocking += gating;
    advisory += findings.length - gating;
    process.stdout.write(formatFindings(findings, relPath) + '\n');
  }

  reportIgnored(ignored);
  if (blocking + advisory === 0) return 0;
  process.stdout.write(summarize(blocking, advisory, level));
  return blocking > 0 ? 1 : 0;
}

// A Map, not an object literal: argv is user input, and `procoder constructor`
// must not find a method on Object.prototype and try to run it.
const COMMANDS = new Map([['check', runCheck], ['baseline', runBaseline], ['verify', runVerify]]);

// Isolated so main() can pull the flag out of argv without pushing the
// function that dispatches commands over the line-count threshold.
function parseFlags(argv) {
  const unusedExclusions = argv.includes('--unused-exclusions');
  const rest = unusedExclusions ? argv.filter((a) => a !== '--unused-exclusions') : argv;
  return { unusedExclusions, rest };
}

function main(argv) {
  const { unusedExclusions, rest } = parseFlags(argv);
  const [command, ...targets] = rest;
  const run = COMMANDS.get(command);
  if (!run || targets.length === 0) {
    process.stderr.write(USAGE);
    return 2;
  }

  // A mistyped or renamed path used to exit 0, which in CI is indistinguishable
  // from "all clean" and turns the gate off permanently. An existing path that
  // yields no checkable files (empty, or entirely excluded) is still a clean run
  // — only a path that does not exist at all is an error.
  const missing = targets.filter((t) => !fs.existsSync(path.resolve(t)));
  if (missing.length > 0) {
    process.stderr.write(`procoder: no such path: ${missing.join(', ')}\n`);
    return 2;
  }

  const files = expand(targets);
  if (files.length === 0) return 0;

  const repoRoot = findRepoRoot(path.dirname(files[0]));
  const config = loadConfig(repoRoot);

  // A baseline from an older procoder suppresses nothing, so a legacy repo would
  // light up red with its whole backlog and no explanation. `verify` stops at 2
  // — "cannot verify, re-baseline required" — rather than exiting 1 with a
  // findings count, which would blame the user for debt they did not add.
  const stale = loadBaseline(repoRoot, config).staleVersion;
  if (stale !== undefined && command !== 'baseline') {
    process.stderr.write(
      `procoder: ${baselinePath(repoRoot, config)} is format v${stale}, this procoder writes ` +
      `v${BASELINE_VERSION}. The fingerprint format changed and old entries cannot be migrated; ` +
      'nothing is suppressed until you re-run `procoder baseline <paths>`.\n');
    if (command === 'verify') return 2;
  }

  return run(files, repoRoot, config, { unusedExclusions });
}

process.exit(main(process.argv.slice(2)));
