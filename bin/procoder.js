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
const { getClaudeDir } = require('../hooks/procoder-config');
const {
  fingerprintsFor, writeBaseline, loadBaseline, growthCheck, baselinePath, BASELINE_VERSION,
} = require('../hooks/checks/baseline');

const USAGE = `usage: procoder <check|baseline|verify> [--unused-exclusions] [--no-ignore] <paths...>
       procoder statusline <install|uninstall|status> [--append] [--force]

  check     report findings not present in the baseline; exit 1 if any of them
            blocks at the active level (at pragmatic, OBVIOUS and ALONE are
            reported but do not block; every other level gates all four rungs)
  baseline  record every current finding as accepted, so only new code is gated
  verify    exit 1 if any finding present today is not in the baseline — the CI ratchet

  --unused-exclusions  (verify only) also fail if a [exclude] rules entry
                        suppressed nothing in this run — a stale suppression
                        left behind after the finding it silenced was fixed
  --no-ignore           check files a .procoderignore covers anyway — answers
                        "why is this file not being checked?". [exclude] paths
                        in .procoder.toml still applies.

  statusline install    add procoder's statusLine to your Claude Code settings
  statusline uninstall  remove it again, restoring any statusLine it composed with
  statusline status     print what statusLine is configured today

  --append (statusline install only) keep the statusLine already configured and
                        print the badge after it, instead of replacing it
  --force  (statusline install only) replace a statusLine that is not procoder's
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

// A Map, not an object literal: argv is user input, and `procoder constructor`
// must not find a method on Object.prototype and try to run it.
const COMMANDS = new Map([['check', runCheck], ['baseline', runBaseline], ['verify', runVerify]]);

// Isolated so main() can pull the flag out of argv without pushing the
// function that dispatches commands over the line-count threshold.
const FLAGS = ['--unused-exclusions', '--no-ignore'];

function parseFlags(argv) {
  return {
    unusedExclusions: argv.includes('--unused-exclusions'),
    noIgnore: argv.includes('--no-ignore'),
    rest: argv.filter((a) => !FLAGS.includes(a)),
  };
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

function main(argv) {
  const { unusedExclusions, noIgnore, rest } = parseFlags(argv);
  const [command, ...targets] = rest;
  // Its arguments are subcommands, not paths, so it branches off before the
  // path handling below.
  if (command === 'statusline') return runStatusline(targets);

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
  const config = { ...loadConfig(repoRoot), noIgnore };

  const halt = reportStaleBaseline(command, repoRoot, config);
  if (halt !== null) return halt;

  return run(files, repoRoot, config, { unusedExclusions });
}

process.exit(main(process.argv.slice(2)));
