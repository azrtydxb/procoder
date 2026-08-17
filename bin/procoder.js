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
const { spawnSync } = require('child_process');
const {
  loadConfig, findRepoRoot, excludingPattern, unusedPathExclusions, unusedIgnorePatterns,
  levelFor, excludeReason,
} = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { runToolBatches } = require('../hooks/checks/resolve');
const { scanFiles, defaultJobs } = require('../hooks/checks/scan');
const { formatFindings } = require('../hooks/checks/finding');
const { FORMATS, jsonReport, sarifReport } = require('../hooks/checks/report');
const { buildIndex, deadExports } = require('../hooks/checks/exports');
const { readLevel } = require('../hooks/procoder-runtime');
const { getClaudeDir } = require('../hooks/procoder-config');
const {
  fingerprintsFor, writeBaseline, loadBaseline, growthCheck, baselinePath, agingEntries,
  BASELINE_VERSION, UNKNOWN_DATE,
} = require('../hooks/checks/baseline');

const USAGE = `usage: procoder <check|baseline|verify> [options] <paths...>
       procoder init [--baseline]
       procoder rot <paths...>
       procoder check [--format text|json|sarif] [--since <ref>] [paths...]
       procoder statusline <install|uninstall|status> [--append] [--force]

  check     report findings not present in the baseline; exit 1 if any of them
            blocks at the active level (at pragmatic, OBVIOUS and ALONE are
            reported but do not block; every other level gates all six rungs;
            [levels] in .procoder.toml pins a level to the paths that earn it)
  init      write a starter .procoder.toml, and say what it did. Nothing is
            overwritten: a file that already exists is left exactly as it is.
            --baseline also records today's findings as accepted, which is what
            makes an existing repository green on the first run.
  baseline  record every current finding as accepted, so only new code is gated
  verify    exit 1 if any finding present today is not in the baseline — the CI ratchet
  rot       index every export in the scan and report the ones nothing else
            mentions — rung 4 over the tree rather than over one file. Reports
            candidates and exits 0: a name reached by reflection, a route table
            or another repository looks identical to a dead one from here, so
            the ones this can see a string mention for are marked "needs
            confirmation" and nothing is ever deleted.

  --unused-exclusions  (verify only) also fail if a [exclude] rules entry
                        suppressed nothing in this run, a [exclude] paths entry
                        holds nothing back — its path is gone, it matches no
                        file, or every file it covers is clean — or a
                        .procoderignore pattern ignores nothing: every tracked
                        file it covers is clean, or it matches nothing at all.
                        A stale suppression left behind after what it silenced
                        was fixed. All but the first are judged only when the
                        run's targets covered the whole repository.
  --aging <days>       (verify only) also fail if an accepted finding has been
                        in the baseline longer than <days>, naming the oldest
                        with its date, path and rule. A baseline entry is a
                        suppression, and a suppression with no end is rot —
                        this is the removal trigger the file cannot carry.
  --jobs <n>           scan with n worker processes. Defaults to one per core
                        (max 8), and to 1 — this process, no workers — for a run
                        of fewer than 250 files, where forking costs more than
                        it saves. The report is identical either way: slices are
                        reassembled in input order.
  --format <fmt>       (check only) text (default), json, or sarif. json is
                        procoder's own versioned shape; sarif is SARIF 2.1.0 for
                        code-scanning dashboards, carrying the same fingerprint
                        the ratchet uses so a moved line is not a new finding.
                        Findings go to stdout; every skip notice stays on stderr,
                        so the document stays parseable. Both formats name the
                        files that were skipped — sarif as invocation
                        notifications — so an unread file cannot read as a clean
                        one.
  --since <ref>        check the files changed since <ref> —
                        'git diff --name-only --diff-filter=ACMRT <ref>...HEAD'
                        plus anything uncommitted. Renames, copies and type
                        changes are included at their new path; deletions are
                        not, having nothing left to read. Paths may still be
                        given to narrow it further, and --unused-exclusions and
                        --aging still apply, judged over what this run read. A
                        git failure exits 2 rather than checking nothing quietly.
  --no-ignore           check files a .procoderignore covers anyway — answers
                        "why is this file not being checked?". [exclude] paths
                        in .procoder.toml still applies, and every file it holds
                        back is reported by count, or by name if you named it.

  statusline install    add procoder's statusLine to your Claude Code settings
  statusline uninstall  remove it again, restoring any statusLine it composed with
  statusline status     print what statusLine is configured today

  --append (statusline install only) keep the statusLine already configured and
                        print the badge after it, instead of replacing it
  --force  (statusline install only) replace a statusLine that is not procoder's
`;

const SKIP_DIRS = new Set(['.git', 'node_modules']);

// Read, not hardcoded: a tool that reports its own version has to report the
// one it was installed as.
const VERSION = (() => {
  try {
    return JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8')).version;
  } catch (e) {
    return '0.0.0';
  }
})();

function expandDirectory(abs) {
  return fs.readdirSync(abs)
    .filter((entry) => !SKIP_DIRS.has(entry))
    .flatMap((entry) => expand([path.join(abs, entry)]));
}

// A file the user typed themselves, as opposed to one a directory walk found.
// The difference is the whole of how a skip is reported: a directory walk that
// steps over vendor/ is the config working, and one line per file would bury
// the findings; a file named on the command line is a direct question, and
// answering it with silence reads as a pass.
const namedOnCommandLine = new Set();

function expand(targets, explicit = false) {
  const files = [];
  for (const target of targets) {
    const abs = path.resolve(target);
    let stat = null;
    try { stat = fs.statSync(abs); } catch (e) { stat = null; }
    if (stat === null) continue;
    if (stat.isDirectory()) {
      files.push(...expandDirectory(abs));
    } else {
      files.push(abs);
      if (explicit) namedOnCommandLine.add(abs);
    }
  }
  return files;
}

// Every skip this process makes, reported through one place so no subcommand
// can forget. `check` said so all along; `baseline` and `verify` did not, and
// verify is where it costs the most — the ratchet compares present findings
// against the baseline, so a file nothing looked at contributes nothing and the
// build goes green over an unchecked file. Deduplicated by path because a
// verify walks the same files twice (see unusedRuleExclusions) and one skip is
// one piece of news, not two.
const skipsReported = new Set();
const ignoredCounts = new Map();
const excludedCounts = new Map();
let uncheckedFiles = 0;

// maxFindings Infinity throughout: the CLI reports and records everything,
// unlike the hook, which shows a top-5 sample inside its time budget.
// Linter answers for this whole run, obtained in one spawn per tool rather than
// one per file. Empty until a command fills it: `check`, `baseline` and `verify`
// all walk many files, and each of them pays the same cold start otherwise.
let toolAnswers = new Map();

// Results a parallel pre-pass already computed, keyed by absolute path. Filled
// only for the one pass of a command that reads every file the same way; the
// second passes verify makes (with rules cleared, or with one path exclusion
// lifted) deliberately bypass it, because they are asking a different question
// of the same files.
let scanned = new Map();

// The exact config object and baseline setting the pre-pass ran under. Identity,
// not equality: `verify` re-walks the same files with `rules` cleared, and again
// with one path exclusion lifted, and each of those is a DIFFERENT question
// about the same file. Answering them from the cache reported a live exclusion
// as unused — caught by the test that exists for exactly that.
let scannedConfig = null;
let scannedBaseline = null;

function findingsFor(absPath, repoRoot, config, applyBaseline = true) {
  const cached = config === scannedConfig && applyBaseline === scannedBaseline
    ? scanned.get(absPath)
    : null;
  if (cached) {
    if (cached.skipped && !skipsReported.has(cached.relPath)) {
      skipsReported.add(cached.relPath);
      reportSkip(cached.relPath, cached.skipped, absPath, config);
    }
    return cached;
  }
  const out = checkFile(absPath, {
    repoRoot, config, maxFindings: Infinity, applyBaseline,
    toolAnswer: toolAnswers.get(path.normalize(absPath)) || null,
  });
  if (out.skipped && !skipsReported.has(out.relPath)) {
    skipsReported.add(out.relPath);
    reportSkip(out.relPath, out.skipped, absPath, config);
  }
  return out;
}

// Every accepted finding is recorded with the rule it silenced and the file it
// sits in, not as a bare hash. writeBaseline stamps the date, and keeps the one
// an entry already had — re-running this must not reset the clock on debt that
// has been sitting there for a year.
function runBaseline(files, repoRoot, config) {
  const entries = (loadBaseline(repoRoot, config).accepted || []).slice();
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, config);
    if (skipped) continue;
    const lines = fs.readFileSync(absPath, 'utf8').split(/\r?\n/);
    const fps = fingerprintsFor(findings, relPath, lines);
    findings.forEach((f, i) => entries.push({ fp: fps[i], id: f.id, path: relPath }));
  }
  reportSkipped();
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
// with its exact (path, id) still turns up. Path exclusions are judged by a
// different test — see unusedPathExclusions in config.js — because re-running
// cannot answer for them. Only files the run actually covered
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

// `.procoderignore` staleness is judged over TRACKED files only — see
// unusedIgnorePatterns. git is the only thing that knows which those are, and
// it is asked once per whole-tree verify with a fixed argv and no shell. No git
// (not installed, not a repository, a checkout too broken to answer) means no
// tracked list, and the ignore rules then judge nothing at all rather than
// treating every untracked scratch file as repository content.
function trackedFiles(root) {
  const r = spawnSync('git', ['ls-files', '-z'], {
    cwd: root, encoding: 'utf8', timeout: 10000, maxBuffer: 64 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  if (r.status !== 0 || r.error) return null;
  return String(r.stdout || '').split('\0').filter(Boolean);
}

// The audit `unusedPathExclusions` and `unusedIgnorePatterns` need to judge a
// pattern on more than whether its path still exists: the tree's repo-relative
// paths, which of them the repository actually tracks, and a scan of one of
// them with that one pattern — a `[exclude] paths` entry or a single
// `.procoderignore` line — lifted.
//
// Only built for a run whose targets covered the whole repository, and that is
// the whole of why it is affordable. "This glob matches nothing" and "nothing
// under this directory has a finding" are claims about the tree; a run over one
// file cannot make either, so a partial run passes no audit and those rules
// go quiet — the same restraint an out-of-run rule exclusion gets. And because
// the run already walked the tree, `files` IS the tree: the audit adds no walk,
// only the re-scan of the excluded files themselves.
//
// It never runs in the hook. The hook calls checkFile directly and has never
// called this file's exclusion audit at all, so its 2s budget is untouched.
function treeAudit(files, repoRoot, config, wholeTree) {
  if (!wholeTree) return undefined;
  const abs = new Map(files.map((f) => [path.relative(config.root, f).replace(/\\/g, '/'), f]));
  // One config per lifted pattern, cached: each carries its own
  // .procoderignore cache, so rebuilding it per file would re-read every
  // ignore file in the tree.
  const lifted = new Map();
  const cachedConfig = (key, build) => {
    if (!lifted.has(key)) lifted.set(key, build());
    return lifted.get(key);
  };
  const scan = (rel, liftedConfig) => {
    const out = checkFile(abs.get(rel),
      { repoRoot, config: liftedConfig, maxFindings: Infinity, applyBaseline: false });
    return out.skipped ? null : out.findings.length;
  };
  const tracked = trackedFiles(config.root);
  return {
    files: Array.from(abs.keys()),
    tracked: tracked && tracked.filter((rel) => abs.has(rel)),
    findings: (rel, pattern) => scan(rel, cachedConfig(`p\0${pattern}`, () => ({ ...config,
      exclude: { ...config.exclude, paths: config.exclude.paths.filter((p) => p !== pattern) } }))),
    ignoreFindings: (rel, rule) => scan(rel, cachedConfig(`i\0${rule.base}\0${rule.line}`,
      () => ({ ...config, ignoreDrop: { base: rule.base, line: rule.line } }))),
  };
}

// Reported under plain `verify` too — a stale exclusion is rot worth seeing —
// but it only fails the build under the dedicated flag: the honest case (the
// underlying finding got fixed) must not turn into a CI failure by default.
function reportUnusedExclusions(files, repoRoot, config, { unusedExclusions, wholeTree }) {
  const stale = unusedRuleExclusions(files, repoRoot, config);
  const audit = treeAudit(files, repoRoot, config, wholeTree);
  const gone = unusedPathExclusions(config, audit);
  const ignores = unusedIgnorePatterns(config, audit);
  if (stale.length > 0) {
    process.stdout.write(
      `procoder: ${stale.length} exclusion rule${stale.length === 1 ? '' : 's'} suppressed nothing ` +
      'in this run:\n' + stale.map((r) => `  ${r.path}:${r.id}\n`).join('') +
      'Remove them from [exclude] rules in .procoder.toml, or note why they still apply.\n');
  }
  if (gone.length > 0) {
    process.stdout.write(
      `procoder: ${gone.length} path exclusion${gone.length === 1 ? ' excludes' : 's exclude'} nothing:\n` +
      gone.map((p) => `  ${p.pattern} — ${p.reason}\n`).join('') +
      'Remove them from [exclude] paths in .procoder.toml, or say why they still apply. Left in ' +
      'place, they silently exclude whatever lands there next.\n');
  }
  if (ignores.length > 0) {
    process.stdout.write(
      `procoder: ${ignores.length} .procoderignore pattern${ignores.length === 1 ? '' : 's'} ` +
      `ignore${ignores.length === 1 ? 's' : ''} nothing:\n` +
      ignores.map((i) => `  ${i.file}:${i.line} "${i.pattern}" — ${i.reason}\n`).join('') +
      'Remove the line, or say why it still applies. Left in place, it silently ignores whatever ' +
      'lands there next.\n');
  }
  return unusedExclusions && stale.length + gone.length + ignores.length > 0 ? 1 : 0;
}

// Accepted debt with no expiry is a deprecation with no removal trigger, which
// is the rung-4 violation procoder refuses everywhere else. The baseline cannot
// carry a trigger — nobody writes one into a generated file — so it carries the
// date instead, and this is what turns that date into a question somebody has
// to answer.
//
// Reported and failing only under the flag, the same contract
// `--unused-exclusions` has: the honest case is a team that has not got to it
// yet, and a CI failure by default would make the flag the first thing anybody
// removes.
function reportAging(baseline, days) {
  const old = agingEntries(baseline, days);
  if (old.length === 0) {
    process.stdout.write(`procoder: no accepted finding is older than ${days} days.\n`);
    return 0;
  }
  process.stdout.write(
    `procoder: ${old.length} accepted finding${old.length === 1 ? '' : 's'} older than ${days} days:\n`
    + old.slice(0, SAMPLE_SIZE).map((e) => `  ${e.added}  ${e.path}  ${e.id}\n`).join('')
    + (old.length > SAMPLE_SIZE ? `  ...and ${old.length - SAMPLE_SIZE} more\n` : '')
    + 'Fix them, or say in the ticket why they stay. A baseline entry is a suppression, and a '
    + 'suppression with no end is rot.\n'
    + (old.some((e) => e.added === UNKNOWN_DATE)
      ? `(${UNKNOWN_DATE} means the entry predates dated baselines; the next `
        + '`procoder baseline` stamps it.)\n'
      : ''));
  return 1;
}

// The ratchet: accepted debt may shrink, never grow. Compares fingerprints,
// not counts, so fixing an old finding buys no room for a new one.
function runVerify(files, repoRoot, config,
  { unusedExclusions = false, wholeTree = false, aging = null } = {}) {
  const baseline = loadBaseline(repoRoot, config);
  const present = presentFindings(files, repoRoot, config);
  reportSkipped();

  const { ok, added, delta } = growthCheck(baseline, present.keys());
  if (!ok) {
    process.stdout.write(
      `procoder: ${delta} finding${delta === 1 ? '' : 's'} not in the baseline ` +
      `(${baseline.size} accepted, ${present.size} present).\n` + sample(added, present) +
      'Fix them, or run `procoder baseline <paths>` only if they are genuinely pre-existing.\n');
    return 1;
  }
  // A ratchet is a claim about what was looked at. `max_file_bytes` set too
  // low skips every file in the repo, and this line used to print "ratchet
  // holds" and exit 0 over a run that read nothing at all — a CI gate that
  // passes because it looked at nothing, which is the worst shape this project
  // has. Excluded and ignored paths are not this: they are a decision the
  // project made about scope, and the counts above say what they cost. A file
  // that was in scope and could not be read is a hole in the scope itself, so
  // verify stops at 2 — "cannot verify" — rather than 1, which would read as
  // "you added findings".
  if (uncheckedFiles > 0) {
    process.stdout.write(
      `procoder: ${uncheckedFiles} file${uncheckedFiles === 1 ? '' : 's'} could not be checked ` +
      '(see above) — the ratchet cannot hold over files nothing looked at. Raise or remove ' +
      '[limits] max_file_bytes, or exclude the path deliberately.\n');
    return 2;
  }
  process.stdout.write(
    `procoder: ${present.size} findings against a baseline of ${baseline.size} — ratchet holds.\n`);
  const stale = reportUnusedExclusions(files, repoRoot, config, { unusedExclusions, wholeTree });
  const old = aging === null ? 0 : reportAging(baseline, aging);
  return stale || old;
}

// Same rule the PostToolUse hook applies: at `pragmatic` the judgment rungs
// (OBVIOUS, ALONE — the ones the severity map marks `warn`) are flagged but do
// not gate. Every other level enforces all six, and so does a missing level
// file, which readLevel resolves to the default `strict` — CI, which has no
// user-level config at all, therefore stays strict.
function isBlocking(finding, level, config) {
  if (level !== 'pragmatic') return true;
  return config.rungs[finding.rung.toLowerCase()] !== 'warn';
}

function summarize(blocking, advisory, level, pinned) {
  const total = blocking + advisory;
  const counts = `${total} finding${total === 1 ? '' : 's'}` +
    (advisory > 0 ? ` — ${blocking} blocking, ${advisory} advisory at [${level}]` : '') +
    // Which level a finding was judged at decides whether it blocks, so a run
    // where some files answered to a different one has to say so — otherwise
    // the count above is explained by a level the reader never sees.
    (pinned > 0 ? ` (${pinned} file${pinned === 1 ? '' : 's'} at a [levels] pin)` : '');
  return blocking > 0
    ? `\nprocoder: ${counts}. Fix them, or run \`procoder baseline <paths>\` ` +
      'to accept pre-existing ones.\n'
    : `\nprocoder: ${counts} — advisory only at this level, not failing the run.\n`;
}

// Every narrowing of the gate is said out loud, and none of them is said once
// per file: the cases these instruments exist for are whole generated subtrees,
// and a line per file would bury the findings they were supposed to make room
// for. Both are counted here and reported in one line each by reportSkipped().
//
// `[exclude] paths` used to be the exception — deliberate config, therefore not
// news. That reasoning does not survive contact with the other half of it: a
// .procoderignore is deliberate config too and has always reported its count,
// and the asymmetry meant a project could lose whole directories of coverage
// and see output identical to a clean run. Coverage lost is news however
// deliberate it was; which pattern did it is what makes the news actionable.
//
// `too-large` and `unreadable` are a different kind again — a file that should
// have been gated was not — and they stay one line per file, and are counted so
// `verify` cannot claim a ratchet over them.
//
// All of it on stderr, so it survives a piped stdout.
function reportSkip(relPath, skipped, absPath, config) {
  if (skipped === 'excluded') {
    const pattern = excludingPattern(config, relPath) || '(unknown)';
    if (namedOnCommandLine.has(absPath)) {
      process.stderr.write(
        `procoder: skipped ${relPath} ([exclude] paths "${pattern}" in .procoder.toml) `
        + '— not checked.\n');
      return;
    }
    excludedCounts.set(pattern, (excludedCounts.get(pattern) || 0) + 1);
    return;
  }
  if (skipped.startsWith('ignored:')) {
    const file = skipped.slice('ignored:'.length);
    ignoredCounts.set(file, (ignoredCounts.get(file) || 0) + 1);
    return;
  }
  uncheckedFiles += 1;
  process.stderr.write(`procoder: skipped ${relPath} (${skipped}) — not checked.\n`);
}

function countLine(count, what) {
  return `procoder: ${count} file${count === 1 ? '' : 's'} skipped by ${what} — not checked.\n`;
}

function reportSkipped() {
  for (const [pattern, count] of excludedCounts) {
    process.stderr.write(countLine(count, `[exclude] paths "${pattern}" in .procoder.toml`));
  }
  for (const [file, count] of ignoredCounts) process.stderr.write(countLine(count, file));
  excludedCounts.clear();
  ignoredCounts.clear();
}

// One pass over the files, in one place, whatever the output format. The text
// path prints as it goes because a long run should show its findings before it
// finishes; the machine formats are one document and can only be written at the
// end, so both collect and only the printing differs.
function collectCheck(files, repoRoot, config, print) {
  const sessionLevel = readLevel();
  const counts = { blocking: 0, advisory: 0, pinned: 0 };
  const entries = [];
  const skippedFiles = [];
  for (const absPath of files) {
    const { relPath, findings, skipped } = findingsFor(absPath, repoRoot, config);
    if (skipped) { skippedFiles.push({ file: relPath, reason: skipped }); continue; }
    if (findings.length === 0) continue;
    const level = levelFor(config, relPath, sessionLevel);
    if (level !== sessionLevel) counts.pinned += 1;
    const gating = findings.filter((f) => isBlocking(f, level, config)).length;
    counts.blocking += gating;
    counts.advisory += findings.length - gating;
    if (print) process.stdout.write(formatFindings(findings, relPath) + '\n');
    else pushEntries(entries, { findings, relPath, absPath, level, config });
  }
  return { sessionLevel, counts, entries, skippedFiles };
}

// The machine formats carry two things the text line does not: whether the
// finding blocked at the level THIS file was judged at — which a [levels] pin
// can move — and the fingerprint the ratchet uses. The fingerprint is what
// stops a dashboard calling a moved line a new finding, and computing it here
// rather than inventing a second identity means the baseline and the dashboard
// always agree about what a finding is.
function pushEntries(entries, file) {
  const { findings, relPath, absPath, level, config } = file;
  let lines = [];
  try { lines = fs.readFileSync(absPath, 'utf8').split(/\r?\n/); } catch (e) { lines = []; }
  const fps = fingerprintsFor(findings, relPath, lines);
  findings.forEach((f, i) => entries.push({
    ...f, file: relPath, blocking: isBlocking(f, level, config), fingerprint: fps[i],
  }));
}

function runCheck(files, repoRoot, config, { format = 'text' } = {}) {
  const text = format === 'text';
  const { sessionLevel, counts, entries, skippedFiles } =
    collectCheck(files, repoRoot, config, text);
  const { blocking, advisory, pinned } = counts;

  // Skip notices are stderr on every path, so a JSON or SARIF document on
  // stdout stays parseable while the coverage it lost is still said out loud.
  reportSkipped();

  if (!text) {
    const summary = { findings: entries.length, blocking, advisory, pinned, level: sessionLevel };
    process.stdout.write(format === 'sarif'
      ? sarifReport({ findings: entries, version: VERSION, skipped: skippedFiles })
      : jsonReport({ findings: entries, level: sessionLevel, summary, skipped: skippedFiles }));
    return blocking > 0 ? 1 : 0;
  }

  if (blocking + advisory === 0) return 0;
  process.stdout.write(summarize(blocking, advisory, sessionLevel, pinned));
  return blocking > 0 ? 1 : 0;
}

// --- statusline ------------------------------------------------------------
//
// Wiring the statusline used to mean hand-editing ~/.claude/settings.json with
// an absolute path the user had to work out for themselves. These subcommands
// do it instead, and treat settings.json throughout as a file that belongs to
// somebody else: parsed rather than re-authored, backed up before any write,
// and replaced by rename so an interrupted run cannot truncate it.

const isWindows = process.platform === 'win32';

// Inside double quotes only these characters keep their meaning, so quoting is
// enough for the spaces and parentheses a real install path is full of — and is
// not enough for these. A path containing one is refused, never embedded.
const UNSAFE_IN_QUOTES = isWindows ? /["`$\r\n]/ : /["`$\\\r\n]/;

// __dirname, never a guess or a hardcoded path: the point of the command is
// that the user should not have to know where the plugin landed.
function statuslineScript() {
  return path.join(__dirname, '..', 'hooks',
    isWindows ? 'procoder-statusline.ps1' : 'procoder-statusline.sh');
}

// POSIX single quoting, which has no escape character: close the quote, emit an
// escaped quote, reopen. Anything at all can be passed through this safely,
// which is what lets the shell fragments below take a path — and, for compose,
// somebody else's whole command line — as an argument rather than as text
// spliced into the middle of a script.
const shQuote = (s) => `'${s.replace(/'/g, "'\\''")}'`;

// A plugin install lands under a version-named directory that the next
// `/procoder:update` replaces wholesale:
//   .../plugins/cache/procoder/procoder/0.2.0/hooks/procoder-statusline.sh
// Writing that path into settings.json pins the command to a directory with a
// known expiry date, and when it goes the script is merely absent — no error,
// no output, the badge just stops appearing. Silent failure is the worst shape
// this project has, so a versioned path is never written; the sibling of the
// version directory is, and the version is resolved at run time.
//
// Returns the parent of the version directory, or null for an install whose
// path has no version in it — a git clone, where pinning is correct and the
// simpler command is the better one.
const VERSION_DIR = /^\d+\.\d+\.\d+/;

function versionedBase(script) {
  const versionDir = path.dirname(path.dirname(script));
  return VERSION_DIR.test(path.basename(versionDir)) ? path.dirname(versionDir) : null;
}

// Picks the most recently written install among the versions present, rather
// than the highest version string: `sort -V` is not portable, lexical order
// puts 0.9.0 above 0.10.0, and the directory an update just wrote is the one
// the user is running. `[ -r ]` covers the no-match case too — an unmatched
// glob stays literal and is not readable — so an uninstalled plugin exits 0
// with no output instead of spraying a resolution error into the status bar.
const RESOLVE_VERSION = 'p=; for c in "$1"/*/hooks/procoder-statusline.sh; do '
  + '[ -r "$c" ] || continue; [ -z "$p" ] || [ "$c" -nt "$p" ] || continue; p=$c; done; '
  + '[ -n "$p" ] || exit 0; exec bash "$p"';

// procoder: no PowerShell equivalent of RESOLVE_VERSION — a Windows plugin
// install stays pinned to its version directory. Add one when procoder is
// actually shipped and tested as a plugin on Windows; an untested resolver in
// the statusline would trade a known gap for an unknown one.
function statuslineCommand(script) {
  if (isWindows) return `powershell -NoProfile -File "${script}"`;
  const base = versionedBase(script);
  return base
    ? `bash -c ${shQuote(RESOLVE_VERSION)} procoder-statusline ${shQuote(base)}`
    : `bash "${script}"`;
}

// Claude Code hands the statusline command its session JSON on stdin, and
// stdin can be read exactly once: `theirs | ours` or `theirs; ours` gives one
// of the two the session context and the other an empty pipe, which for a
// git-aware prompt reading `.cwd` means a statusline that quietly loses its
// directory. So stdin is read once, into a variable, and replayed into both.
//
// Both command lines arrive as arguments rather than spliced into this text:
// the existing one is the user's, of unknown shape, and shQuote is what makes
// any shape safe. `eval` runs each as the shell line it already was.
//
// Ours goes last — the badge is appended to their statusline, not the other way
// round — and the separating space appears only when both actually printed,
// because ours prints nothing at all when procoder is inactive.
const COMPOSE = 'i=$(cat); u=$(printf %s "$i" | eval "$1"); p=$(printf %s "$i" | eval "$2"); '
  + '[ -n "$u" ] && [ -n "$p" ] && u="$u "; printf "%s%s\\n" "$u" "$p"';

const composeCommand = (theirs, ours) =>
  `bash -c ${shQuote(COMPOSE)} procoder-statusline ${shQuote(theirs)} ${shQuote(ours)}`;

const settingsPath = () => path.join(getClaudeDir(), 'settings.json');

// Ours is any statusLine whose command names our script, at whatever path: an
// install that moved is still ours to update, and anything else is not ours to
// touch at all.
const isOurs = (entry) => !!entry && typeof entry === 'object'
  && String(entry.command || '').includes('procoder-statusline');

// The statusLine procoder wrapped, kept verbatim beside the composed command
// rather than reconstructed by parsing it back out: uninstall then restores the
// user's own entry as the object it was, and a wrapper this file learns to
// write differently later cannot strand a command nobody can un-wrap.
const wrapped = (entry) => (isOurs(entry) ? entry.procoderOriginal : undefined);

// Throws on anything this command will not write to. Callers run under
// runStatusline's catch, which turns the message into a clean non-zero exit.
function readSettings(file) {
  if (!fs.existsSync(file)) return {};
  const raw = fs.readFileSync(file, 'utf8');
  let data = null;
  try {
    data = JSON.parse(raw);
  } catch (e) {
    throw new Error(`${file} is not valid JSON (${e.message}). Left untouched — fix it by ` +
      'hand and re-run. Overwriting it would destroy settings you cannot get back.');
  }
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new Error(`${file} does not hold a JSON object. Left untouched.`);
  }
  return data;
}

// Temp file then rename: rename is atomic within a directory, so a run killed
// mid-write leaves either the old settings or the new ones and never half a
// file. A truncated settings.json breaks the user's whole Claude Code setup,
// which is a far worse outcome than a statusline that did not get installed.
function saveSettings(file, data) {
  const backup = fs.existsSync(file) ? `${file}.backup-${Date.now()}` : null;
  if (backup) fs.copyFileSync(file, backup);

  const tmp = `${file}.procoder-tmp-${process.pid}`;
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(tmp, JSON.stringify(data, null, 2) + '\n');
  fs.renameSync(tmp, file);

  process.stdout.write(`procoder: wrote ${file}\n`
    + (backup ? `procoder: previous settings backed up to ${backup}\n` : ''));
}

const describe = (entry) => String(entry.command || JSON.stringify(entry));

function refuseClobber(current, desired) {
  process.stderr.write(
    'procoder: a statusLine is already configured, and it is not procoder\'s:\n'
    + `  ${describe(current)}\nprocoder would set:\n  ${desired.command}\n`
    + 'Left as it is. Re-run with --append to keep it and add the badge after it, '
    + 'or with --force to replace it.\n');
  return 1;
}

// Quoting cannot make these paths safe, and a command built around one would
// execute part of its own path. The snippet still gets printed: the user knows
// their own shell and can escape it, which is more than this command can do.
function refuseUnsafePath(script, file) {
  process.stderr.write(
    `procoder: this install path contains characters a shell would interpret:\n  ${script}\n`
    + `Refusing to build a command around it. Add this to ${file} by hand instead, with the\n`
    + 'path quoted or escaped to suit your shell:\n'
    + `  "statusLine": { "type": "command", "command": ${JSON.stringify(statuslineCommand(script))} }\n`);
  return 1;
}

// What `install` would write, given what is there now.
//
// Composition is sticky: once procoder is wrapping somebody else's statusline,
// a later plain `install` re-wraps rather than dropping their command on the
// floor — that is how the version fix above reaches a composed entry too, and
// `uninstall` stays the only way back to theirs.
//
// An existing entry with no string command is not something to compose with, so
// it falls through to the plain entry and meets the usual clobber refusal.
function desiredEntry(script, current, append) {
  const ours = statuslineCommand(script);
  const theirs = wrapped(current) || (append && !isOurs(current) ? current : undefined);
  if (!theirs || typeof theirs.command !== 'string') return { type: 'command', command: ours };
  return {
    type: 'command',
    command: composeCommand(theirs.command, ours),
    procoderOriginal: theirs,
  };
}

const sameEntry = (a, b) => a.type === b.type && a.command === b.command
  && JSON.stringify(a.procoderOriginal) === JSON.stringify(b.procoderOriginal);

// The composed command is one long POSIX shell line, and PowerShell is not a
// POSIX shell. Refused rather than approximated: a wrapper that mangles the
// user's own statusline is worse than not offering the mode.
function refuseAppendOnWindows() {
  process.stderr.write('procoder: --append builds a POSIX shell command and is not supported on '
    + 'Windows. Install without it to replace the statusLine, or compose the two by hand.\n');
  return 1;
}

function runInstall({ force, append }) {
  const script = statuslineScript();
  const file = settingsPath();
  if (UNSAFE_IN_QUOTES.test(script)) return refuseUnsafePath(script, file);
  if (append && isWindows) return refuseAppendOnWindows();

  const data = readSettings(file);
  const current = data.statusLine;
  const desired = desiredEntry(script, current, append);
  if (current && !isOurs(current) && !desired.procoderOriginal && !force) {
    return refuseClobber(current, desired);
  }
  if (current && sameEntry(current, desired)) {
    process.stdout.write(`procoder: statusline already installed in ${file} — nothing to do.\n`);
    return 0;
  }

  // Spread, so an existing statusLine keeps its position in the file and every
  // other key keeps its value and its order.
  saveSettings(file, { ...data, statusLine: desired });
  return 0;
}

function runUninstall() {
  const file = settingsPath();
  const data = readSettings(file);
  const current = data.statusLine;
  if (!current) {
    process.stdout.write(`procoder: no statusLine configured in ${file} — nothing to remove.\n`);
    return 0;
  }
  if (!isOurs(current)) {
    process.stderr.write('procoder: the configured statusLine is not procoder\'s:\n'
      + `  ${describe(current)}\nLeft as it is.\n`);
    return 1;
  }

  // A composed entry is only half ours: removing it would take the user's own
  // statusline with it, so the recorded original goes back in its place — the
  // same object, so their command comes back byte for byte.
  const original = wrapped(current);
  if (original) {
    process.stdout.write(`procoder: restoring the statusLine procoder composed with:\n  ${describe(original)}\n`);
    saveSettings(file, { ...data, statusLine: original });
    return 0;
  }

  delete data.statusLine;
  saveSettings(file, data);
  return 0;
}

// Four states, said apart: nothing configured, ours, somebody else's, and ours
// wrapped around somebody else's. Collapsing the last two into "installed"
// would hide the one case where uninstall does something other than delete.
function describeState(current) {
  if (!current) return 'no statusLine configured — not installed';
  const original = wrapped(current);
  if (original) return `installed, composed with an existing statusLine: ${describe(original)}`;
  return `${isOurs(current) ? 'installed' : 'a statusLine that is not procoder\'s'}: ${describe(current)}`;
}

function runStatus() {
  const file = settingsPath();
  process.stdout.write(
    `procoder: ${file}\nprocoder: ${describeState(readSettings(file).statusLine)}\n`);
  return 0;
}

const STATUSLINE = new Map([
  ['install', runInstall], ['uninstall', runUninstall], ['status', runStatus],
]);

// Every failure here is a message and a non-zero exit, never a stack trace:
// this command's whole job is editing a file the user cares about, and an
// unhandled throw would leave them guessing how far it got.
function runStatusline(args) {
  const run = STATUSLINE.get(args.find((a) => !a.startsWith('--')));
  if (!run) {
    process.stderr.write(USAGE);
    return 2;
  }
  try {
    return run({ force: args.includes('--force'), append: args.includes('--append') });
  } catch (e) {
    process.stderr.write(`procoder: ${e.message}\n`);
    return 1;
  }
}

// The starter config lives in scripts/templates/ with the pre-commit hook and
// the CI workflow, and for the same reason: a config template written inline is
// a block of commented-out TOML inside a JavaScript file, which reads to rung 4
// exactly like commented-out code — because that is what it looks like.
function starterConfig() {
  return fs.readFileSync(path.join(__dirname, '..', 'scripts', 'templates', 'procoder.toml'), 'utf8');
}

// Writes what is missing and says so; never touches what is there. An init that
// overwrote a config would be the one command in this tool capable of deleting
// somebody's decisions.
function runInit(argv) {
  const repoRoot = findRepoRoot(process.cwd());
  const configPath = path.join(repoRoot, '.procoder.toml');
  if (fs.existsSync(configPath)) {
    process.stdout.write(`procoder: ${configPath} already exists — left as it is.\n`);
  } else {
    fs.writeFileSync(configPath, starterConfig());
    process.stdout.write(`procoder: wrote ${configPath}\n`);
  }

  if (!argv.includes('--baseline')) {
    process.stdout.write('procoder: run `procoder check .` to see where you stand, or '
      + '`procoder init --baseline` to accept today\'s findings and gate only new code.\n');
    return 0;
  }

  const config = { ...loadConfig(repoRoot), noIgnore: false };
  const code = runBaseline(expand([repoRoot]), repoRoot, config);
  process.stdout.write('procoder: from here, `procoder verify .` fails only on findings that '
    + 'are not in the baseline. `procoder verify --aging 90 .` names the ones that have been '
    + 'accepted longest.\n');
  return code;
}

// Files a published package points at from outside its source: anything named
// in `bin`, `main`, `module` or `exports`. A name exported from one of those has
// callers no index of this repository can see, and reporting it as dead is the
// one failure that would make this command untrustworthy.
function declaredEntryFiles(repoRoot) {
  const entries = new Set();
  let manifest;
  try {
    manifest = JSON.parse(fs.readFileSync(path.join(repoRoot, 'package.json'), 'utf8'));
  } catch (e) {
    return entries;
  }
  const collect = (value) => {
    if (typeof value === 'string') entries.add(value.replace(/^\.\//, ''));
    else if (value && typeof value === 'object') Object.values(value).forEach(collect);
  };
  for (const key of ['bin', 'main', 'module', 'exports']) collect(manifest[key]);
  return entries;
}

// Rung 4 over the tree. Every other check in this engine answers for one file,
// which is why the rung procoder exists for had no engine behind it: "you left a
// twin behind" is a claim about the repository, and one file cannot make it.
//
// Exit 0 even with findings, deliberately. These are candidates — the five
// reachability routes the rot skill checks by hand (dynamic dispatch, entry
// points declared outside source, a published API, cross-language callers,
// constructed names) are exactly the ones an index of bare words cannot see. A
// command that failed a build on a guess about deletion would get switched off,
// and rightly.
function runRot(files, repoRoot, config) {
  const sources = new Map();
  let unread = 0;
  for (const absPath of files) {
    const rel = path.relative(repoRoot, absPath).replace(/\\/g, '/');
    if (excludeReason(config, rel)) continue;
    try {
      sources.set(rel, fs.readFileSync(absPath, 'utf8'));
    } catch (e) {
      // A file this pass could not read is not a file with nothing in it: its
      // mentions are missing from the index, and a missing mention is exactly
      // what turns a live symbol into a candidate for deletion. Counted and
      // reported, never swallowed.
      unread += 1;
      process.stderr.write(`procoder: could not read ${rel} (${e.code || e.message}) — `
        + 'its references are missing from this index.\n');
    }
  }

  const index = buildIndex(Array.from(sources.keys()), (rel) => sources.get(rel));
  const findings = deadExports(index, { entryFiles: declaredEntryFiles(repoRoot) });
  for (const f of findings) process.stdout.write(formatFindings([f], f.file) + '\n');

  const confirm = findings.filter((f) => f.needsConfirmation).length;
  process.stdout.write(`\nprocoder: ${index.exports.length} exports across ${sources.size} files, `
    + `${findings.length} with no mention elsewhere (${confirm} need confirmation). `
    + 'Nothing here is deleted — check reachability before removing any of them.\n'
    + (unread > 0 ? `procoder: ${unread} file${unread === 1 ? '' : 's'} could not be read, so `
      + 'this index is incomplete — treat every candidate above as unconfirmed.\n' : ''));
  return 0;
}

// A Map, not an object literal: argv is user input, and `procoder constructor`
// must not find a method on Object.prototype and try to run it.
const COMMANDS = new Map([
  ['check', runCheck], ['baseline', runBaseline], ['verify', runVerify], ['rot', runRot],
]);

// Isolated so main() can pull the flag out of argv without pushing the
// function that dispatches commands over the line-count threshold.
const FLAGS = ['--unused-exclusions', '--no-ignore'];
const VALUE_FLAGS = ['--format', '--since', '--aging', '--jobs'];

// Both spellings, `--format json` and `--format=json`, because a user who types
// the one this parser does not accept gets their argument treated as a path,
// and "no such path: json" is a worse answer than either.
function takeValues(argv) {
  const values = new Map();
  const rest = [];
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    const eq = VALUE_FLAGS.find((f) => arg.startsWith(`${f}=`));
    if (eq) { values.set(eq, arg.slice(eq.length + 1)); continue; }
    if (VALUE_FLAGS.includes(arg)) { values.set(arg, argv[i + 1]); i += 1; continue; }
    rest.push(arg);
  }
  return { values, rest };
}

function parseFlags(argv) {
  const { values, rest } = takeValues(argv);
  return {
    unusedExclusions: argv.includes('--unused-exclusions'),
    noIgnore: argv.includes('--no-ignore'),
    format: values.get('--format') || 'text',
    since: values.get('--since') || null,
    aging: values.has('--aging') ? Number(values.get('--aging')) : null,
    // Raw string, not Number(...): clampJobs echoes the value as written, and
    // converting here defeats that — `--jobs abc` warned about `NaN` and
    // `--jobs Infinity` about `null`, naming something the user never typed.
    jobs: values.has('--jobs') ? values.get('--jobs') : null,
    rest: rest.filter((a) => !FLAGS.includes(a)),
  };
}

// The files git says changed since <ref>, plus anything uncommitted — a review
// of "what I am about to push" that ignored the working tree would be checking
// the wrong thing.
//
// Every git failure is an error, never an empty list. The shipped CI template
// used to do this in shell with `|| true`, so a first push (whose "before" SHA
// is all zeros) made git fail, produced no files, and passed the build green
// having checked nothing. That is the exact silent coverage loss this project
// exists to refuse, so it exits 2 — "cannot check" — and says which git command
// failed.
// The filter is `ACMRT`, not `ACM`. `ACM` dropped `R`, and git reports a moved
// file as a rename rather than as a delete plus an add — so a commit that
// renamed a file carrying a live safe/sql-injection selected no files at all,
// printed "nothing to check" and exited 0. `C` (a copy) and `T` (a type change,
// including a symlink that became a regular file) are content arriving under a
// path nothing has checked, and belong for the same reason.
//
// `D` stays out, and that is the one narrowing here that is deliberate: a
// deleted file has no bytes to scan, and `--name-only` for a rename already
// names the DESTINATION, so the moved content is checked at the path it now
// lives at. Nothing is lost by leaving `D` out — unlike `R`, which took the
// surviving file with it.
function changedSince(ref, repoRoot) {
  const commands = [
    ['diff', '--name-only', '--diff-filter=ACMRT', `${ref}...HEAD`],
    ['diff', '--name-only', '--diff-filter=ACMRT', 'HEAD'],
    ['ls-files', '--others', '--exclude-standard'],
  ];
  const files = new Set();
  for (const argv of commands) {
    const r = spawnSync('git', argv, { cwd: repoRoot, encoding: 'utf8' });
    if (r.status !== 0) {
      throw new Error(`git ${argv.join(' ')} failed: ${String(r.stderr || '').trim()}`);
    }
    for (const line of String(r.stdout || '').split('\n')) if (line) files.add(line);
  }
  return Array.from(files)
    .map((rel) => path.join(repoRoot, rel))
    .filter((abs) => fs.existsSync(abs));
}

// `--since` needs a repository before it can ask git anything, and the normal
// path derives the root from the first target. Resolved from the working
// directory instead, which is the same root for any path inside it.
function sinceTargets(since, targets) {
  const repoRoot = findRepoRoot(process.cwd());
  const changed = changedSince(since, repoRoot);
  if (targets.length === 0) return changed;
  // Paths given alongside --since narrow it: the intersection, so `--since main
  // src/` means "what changed under src/".
  const under = targets.map((t) => path.resolve(t));
  return changed.filter((abs) => under.some((t) => abs === t || abs.startsWith(`${t}${path.sep}`)));
}

// "Nothing changed" is a real answer and is said out loud. Exiting 0 in silence
// is indistinguishable from a clean check of a hundred files, which is how a CI
// gate quietly stops covering anything.
//
// `--unused-exclusions` and `--aging` are honoured here rather than refused.
// Both used to be accepted and silently dropped, which is the same lie in
// miniature — a flag that does nothing is how somebody concludes the check ran.
// Honouring them is what they already mean elsewhere: `--aging` judges the
// baseline, which no file selection changes, and `--unused-exclusions` behaves
// exactly as it does for any other partial-scope run — `wholeTree` is false, so
// the claims that need to have seen the tree stay unmade, and the ones about
// the files this run did read are enforced.
async function runSince(since, targets,
  { command, noIgnore, format, jobs, unusedExclusions = false, aging = null }) {
  let files = [];
  try {
    files = sinceTargets(since, targets);
  } catch (e) {
    process.stderr.write(`procoder: ${e.message}\n`);
    return 2;
  }
  if (files.length === 0) {
    process.stdout.write(`procoder: no files changed since ${since} — nothing to check.\n`);
    return 0;
  }
  const repoRoot = findRepoRoot(path.dirname(files[0]));
  const config = { ...loadConfig(repoRoot), noIgnore };
  const halt = reportStaleBaseline(command, repoRoot, config);
  if (halt !== null) return halt;
  toolAnswers = runToolBatches(files, { repoRoot });
  scanned = await prescan({ command, files, repoRoot, config, jobs });
  return COMMANDS.get(command)(files, repoRoot, config,
    { format, unusedExclusions, aging, wholeTree: false });
}

// The commands that read every file once, and how they read it. `rot` builds
// its own index and `init` writes a file; neither goes through findingsFor.
//
// verify's main pass takes findings BEFORE baseline suppression — the ratchet
// compares the full picture against what was accepted — so its pre-pass has to
// ask for the same thing, or the cache would answer a different question from
// the one presentFindings asks.
const SCAN_PASS = new Map([
  ['check', { applyBaseline: true }],
  ['baseline', { applyBaseline: true }],
  ['verify', { applyBaseline: false }],
]);

// One parallel pass over the file list, before the command walks it. Sequential
// under the threshold, and sequential if anything goes wrong — see scan.js.
async function prescan({ command, files, repoRoot, config, jobs }) {
  const pass = SCAN_PASS.get(command);
  if (!pass) return new Map();
  const results = await scanFiles(files,
    { repoRoot, config, applyBaseline: pass.applyBaseline, toolAnswers, jobs }, checkFile);
  scannedConfig = config;
  scannedBaseline = pass.applyBaseline;
  return new Map(results.map((r) => [r.absPath, r]));
}

// A baseline from an older procoder suppresses nothing, so a legacy repo would
// light up red with its whole backlog and no explanation. `verify` stops at 2
// — "cannot verify, re-baseline required" — rather than exiting 1 with a
// findings count, which would blame the user for debt they did not add.
// Returns an exit code to stop on, or null to carry on.
function reportStaleBaseline(command, repoRoot, config) {
  const stale = loadBaseline(repoRoot, config).staleVersion;
  if (stale === undefined || command === 'baseline') return null;
  process.stderr.write(
    `procoder: ${baselinePath(repoRoot, config)} is format v${stale}, this procoder writes ` +
    `v${BASELINE_VERSION}. The fingerprint format changed and old entries cannot be migrated; ` +
    'nothing is suppressed until you re-run `procoder baseline <paths>`.\n');
  return command === 'verify' ? 2 : null;
}

// Everything that can refuse a command line before any file is read. Returns
// an exit code to stop on, or null to carry on — the same shape
// reportStaleBaseline uses, so main() reads as one list of refusals.
// A flag that belongs to one subcommand and was typed at another. Refused
// rather than ignored: a flag that silently does nothing is how somebody
// concludes the feature is broken, or worse, believes a check ran.
function refuseMisplacedFlag({ command, format, aging }) {
  if (aging !== null && (command !== 'verify' || !Number.isFinite(aging) || aging < 0)) {
    process.stderr.write('procoder: --aging takes a number of days, on `verify` only\n');
    return 2;
  }
  if (!FORMATS.has(format) || (format !== 'text' && command !== 'check')) {
    process.stderr.write(`procoder: --format ${format} is not available for ${command || 'this'} `
      + '— text, json and sarif, on `check` only\n');
    return 2;
  }
  return null;
}

function refuseArguments({ command, targets, format, since, aging }) {
  const misplaced = refuseMisplacedFlag({ command, format, aging });
  if (misplaced !== null) return misplaced;
  // `--since` supplies the targets, so it is the one shape where naming none is
  // not a usage error.
  if (!COMMANDS.get(command) || (targets.length === 0 && since === null)) {
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
  return null;
}

async function main(argv) {
  const { unusedExclusions, noIgnore, format, since, aging, jobs, rest } = parseFlags(argv);
  const [command, ...targets] = rest;
  // Its arguments are subcommands, not paths, so it branches off before the
  // path handling below.
  if (command === 'statusline') return runStatusline(targets);
  if (command === 'init') return runInit(targets);

  const refused = refuseArguments({ command, targets, format, since, aging });
  if (refused !== null) return refused;
  if (since !== null) {
    return runSince(since, targets,
      { command, noIgnore, format, jobs, unusedExclusions, aging });
  }

  const files = expand(targets, true);
  if (files.length === 0) return 0;

  const repoRoot = findRepoRoot(path.dirname(files[0]));
  const config = { ...loadConfig(repoRoot), noIgnore };

  const halt = reportStaleBaseline(command, repoRoot, config);
  if (halt !== null) return halt;

  // One spawn per linter for the whole run, instead of one per file. The
  // budget is generous — this is the CLI, not the 2s hook — and a tool that
  // times out or crashes simply answers for nothing, which leaves every file it
  // owns to the built-in pack exactly as a per-file timeout does.
  toolAnswers = runToolBatches(files, { repoRoot });
  scanned = await prescan({ command, files, repoRoot, config, jobs });

  // Whether this run's targets covered the whole repository. Two of the three
  // path-exclusion staleness rules are claims about the tree and are only made
  // when the run actually saw it — see pathAudit.
  const wholeTree = targets.some((t) => path.resolve(t) === repoRoot);

  return COMMANDS.get(command)(files, repoRoot, config,
    { unusedExclusions, wholeTree, format, aging });
}

// The scan is asynchronous now, so the exit code arrives as a promise. A throw
// here would exit 0 through an unhandled rejection on some Node versions, which
// in a gate reads as a pass — so it is caught and turned into 2, "cannot check".
main(process.argv.slice(2)).then(
  (code) => process.exit(code),
  (e) => {
    process.stderr.write(`procoder: ${e && e.stack ? e.stack : e}\n`);
    process.exit(2);
  },
);
