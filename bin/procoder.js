#!/usr/bin/env node
// procoder — CLI for pre-commit hooks and CI. Same engine as the PostToolUse
// hook, so what fails locally fails in CI for the same reason.
//
// Usage:
//   procoder check <paths...>     exit 1 if any non-baselined finding exists
//   procoder baseline <paths...>  record current findings as accepted
//   procoder verify <paths...>    exit 1 only if findings exceed the baseline

const fs = require('fs');
const path = require('path');
const { loadConfig, findRepoRoot } = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { formatFindings } = require('../hooks/checks/finding');
const { fingerprint, writeBaseline, loadBaseline, growthCheck } = require('../hooks/checks/baseline');

const USAGE = `usage: procoder <check|baseline|verify> <paths...>

  check     report findings not present in the baseline; exit 1 if any
  baseline  record every current finding as accepted, so only new code is gated
  verify    exit 1 only if total findings exceed the baseline — the CI ratchet
`;

function expand(targets) {
  const files = [];
  for (const target of targets) {
    const abs = path.resolve(target);
    let stat;
    try { stat = fs.statSync(abs); } catch (e) { continue; }
    if (stat.isDirectory()) {
      for (const entry of fs.readdirSync(abs)) {
        if (entry === '.git' || entry === 'node_modules') continue;
        files.push(...expand([path.join(abs, entry)]));
      }
    } else {
      files.push(abs);
    }
  }
  return files;
}

function main(argv) {
  const [command, ...targets] = argv;
  if (!['check', 'baseline', 'verify'].includes(command) || targets.length === 0) {
    process.stderr.write(USAGE);
    return 2;
  }

  const files = expand(targets);
  if (files.length === 0) return 0;

  const repoRoot = findRepoRoot(path.dirname(files[0]));
  const config = loadConfig(repoRoot);

  if (command === 'baseline') {
    const entries = Array.from(loadBaseline(repoRoot, config));
    for (const absPath of files) {
      // maxFindings Infinity: a baseline must record everything, not a top-5 sample.
      const { relPath, findings, skipped } = checkFile(absPath, {
        repoRoot, config, maxFindings: Infinity,
      });
      if (skipped) continue;
      const lines = fs.readFileSync(absPath, 'utf8').split(/\r?\n/);
      for (const f of findings) entries.push(fingerprint(f, relPath, lines[f.line - 1]));
    }
    writeBaseline(repoRoot, config, entries);
    process.stdout.write(`procoder: baseline recorded (${entries.length} accepted findings)\n`);
    return 0;
  }

  if (command === 'verify') {
    // The ratchet: accepted debt may shrink, never grow. Counts findings BEFORE
    // baseline suppression, so a fix and a fresh violation do not cancel out.
    const baseline = loadBaseline(repoRoot, config);
    let total = 0;
    for (const absPath of files) {
      const { findings, skipped } = checkFile(absPath, {
        repoRoot, config, maxFindings: Infinity, applyBaseline: false,
      });
      if (!skipped) total += findings.length;
    }
    const { ok, delta } = growthCheck(baseline, total);
    if (!ok) {
      process.stdout.write(
        `procoder: findings grew by ${delta} beyond the baseline (${baseline.size} accepted, ${total} present).\n` +
        'Fix the new findings, or run `procoder baseline <paths>` only if they are genuinely pre-existing.\n');
      return 1;
    }
    process.stdout.write(
      `procoder: ${total} findings against a baseline of ${baseline.size} — ratchet holds.\n`);
    return 0;
  }

  let total = 0;
  for (const absPath of files) {
    const { relPath, findings, skipped } = checkFile(absPath, {
      repoRoot, config, maxFindings: Infinity,
    });
    if (skipped || findings.length === 0) continue;
    total += findings.length;
    process.stdout.write(formatFindings(findings, relPath) + '\n');
  }

  if (total > 0) {
    process.stdout.write(`\nprocoder: ${total} finding${total === 1 ? '' : 's'}. ` +
      'Fix them, or run `procoder baseline <paths>` to accept pre-existing ones.\n');
    return 1;
  }
  return 0;
}

process.exit(main(process.argv.slice(2)));
