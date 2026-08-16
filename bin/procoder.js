#!/usr/bin/env node
// procoder — CLI for pre-commit hooks and CI. Same engine as the PostToolUse
// hook, so what fails locally fails in CI for the same reason.
//
// Usage:
//   procoder check <paths...>     exit 1 if any non-baselined finding exists
//   procoder baseline <paths...>  record current findings as accepted
//   procoder verify <paths...>    exit 1 if any finding is not in the baseline

const fs = require('fs');
const path = require('path');
const { loadConfig, findRepoRoot } = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { formatFindings } = require('../hooks/checks/finding');
const { fingerprintsFor, writeBaseline, loadBaseline, growthCheck } = require('../hooks/checks/baseline');

const USAGE = `usage: procoder <check|baseline|verify> <paths...>

  check     report findings not present in the baseline; exit 1 if any
  baseline  record every current finding as accepted, so only new code is gated
  verify    exit 1 if any finding present today is not in the baseline — the CI ratchet
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

const SAMPLE_SIZE = 3;

// Naming a few of the new findings makes a CI failure actionable; the full
// list is what `procoder check` is for.
function sample(added, present) {
  const shown = added.slice(0, SAMPLE_SIZE).map((fp) => `  ${present.get(fp)}\n`).join('');
  const rest = added.length - SAMPLE_SIZE;
  return shown + (rest > 0 ? `  ...and ${rest} more\n` : '');
}

// The ratchet: accepted debt may shrink, never grow. Compares fingerprints,
// not counts, so fixing an old finding buys no room for a new one.
function runVerify(files, repoRoot, config) {
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
  return 0;
}

function runCheck(files, repoRoot, config) {
  let total = 0;
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, config);
    if (skipped || findings.length === 0) continue;
    total += findings.length;
    process.stdout.write(formatFindings(findings, relPath) + '\n');
  }

  if (total === 0) return 0;
  process.stdout.write(`\nprocoder: ${total} finding${total === 1 ? '' : 's'}. ` +
    'Fix them, or run `procoder baseline <paths>` to accept pre-existing ones.\n');
  return 1;
}

// A Map, not an object literal: argv is user input, and `procoder constructor`
// must not find a method on Object.prototype and try to run it.
const COMMANDS = new Map([['check', runCheck], ['baseline', runBaseline], ['verify', runVerify]]);

function main(argv) {
  const [command, ...targets] = argv;
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
  return run(files, repoRoot, loadConfig(repoRoot));
}

process.exit(main(process.argv.slice(2)));
