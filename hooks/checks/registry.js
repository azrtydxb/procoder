#!/usr/bin/env node
// procoder — extension → pack, and pack → preferred external tool.
//
// The tool entries describe how to INVOKE and PARSE each linter. Whether one is
// actually configured in the project is resolve.js's job.

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const { finding } = require('./finding');

// golangci-lint v2 removed `--out-format json` in favor of
// `--output.json.path <path>` (JSON schema on the wire is unchanged — only
// the flag that requests it moved). Passing the wrong flag for the
// installed major version makes the linter exit with a flag-parse error, so
// parse() silently returns [] no matter how good the regex is. Detecting
// the major version once (cached for the process lifetime) and picking the
// matching flag is cheap next to golangci-lint's own runtime, and it means
// both v1 and v2 installs stay working — no config, no guessing.
let golangciMajor = null;
function golangciMajorVersion() {
  if (golangciMajor !== null) return golangciMajor;
  try {
    const out = execFileSync('golangci-lint', ['--version'], { encoding: 'utf8', timeout: 1000 });
    const m = /\bv?(\d+)\./.exec(out);
    golangciMajor = m ? Number(m[1]) : 1;
  } catch (e) {
    golangciMajor = 1;
  }
  return golangciMajor;
}

// Picking the right flag was only half of it: golangci-lint v2 writes the JSON
// document to stdout and then appends a human-readable tally to the SAME
// stream ("1 issues:\n* typecheck: 1"), so JSON.parse over the whole stream
// throws and every v2 finding is dropped. v1 wrote nothing but the document.
// Both encode it with Go's json.Encoder, which emits exactly one line, so the
// first line that opens an object is the whole report. Output carrying no
// JSON document at all rethrows — unreadable must not read as clean.
function golangciReport(stdout) {
  try {
    return JSON.parse(stdout);
  } catch (e) {
    const document = String(stdout).split('\n').find((line) => line.startsWith('{'));
    if (document === undefined) throw e;
    return JSON.parse(document);
  }
}



// External findings land on rung TRUE: a configured linter's rules are the
// project's own definition of correct, and procoder defers to them.
//
// The id carries the tool's own rule id (e.g. `true/eslint:no-eval`), not
// just the tool name. Two different rules firing on the same line must not
// collapse into one baseline fingerprint — that would let baselining one
// rule's hit silently suppress a different rule's hit at the same location
// later, exactly what procoder's own inline-suppression doctrine forbids.
// Which rung an analyzer's rule belongs to.
//
// This matters for more than sort order. The level (pragmatic/strict/paranoid)
// decides which rungs BLOCK, and it does that by rung number — so if every
// analyzer finding arrived as SAFE or TRUE, `pragmatic` and `strict` would be
// the same gate and the level would be decoration. Linters do report all four
// kinds; this is the mapping that keeps the distinction real:
//
//   1 SAFE     a weakness — flake8-bandit S###, eslint-plugin-security, gosec,
//              and every semgrep rule (only security rulesets are ever enabled)
//   2 TRUE     correctness — a swallowed error, an unchecked return, a bug
//   3 OBVIOUS  readability — complexity, nesting depth, length, parameter count
//   4 ALONE    something left behind — an unused import, a dead binding
//
// Unmatched rules stay at TRUE. That direction is deliberate: a rule wrongly
// called OBVIOUS stops blocking at pragmatic, while one wrongly called TRUE is
// still reported and still fixed.
const RUNG_PATTERNS = [
  ['SAFE', /^(S\d{3}|G\d{3})$|^security\/|gosec|bandit|injection|hardcoded|crypto|csrf|xss|ssrf|traversal|insecure/i],
  ['ALONE', /unused|^F401$|^F841$|deadcode|unparam|ineffassign|no-unreachable|redundant/i],
  ['OBVIOUS', /complexity|cyclo|cognitive|max-lines|max-depth|max-params|too-many|nesting|长/i],
];

function rungFor(tool, ruleId) {
  // semgrep runs only security rulesets here, so every finding it has is rung 1
  // whatever the rule is called.
  if (tool === 'semgrep') return 'SAFE';
  const id = String(ruleId || '');
  if (!id) return 'TRUE';
  for (const [rung, pattern] of RUNG_PATTERNS) {
    if (pattern.test(id)) return rung;
  }
  return 'TRUE';
}

function externalFinding(line, message, tool, ruleId) {
  const rung = rungFor(tool, ruleId);
  const id = ruleId
    ? `${rung.toLowerCase()}/${tool}:${ruleId}`
    : `${rung.toLowerCase()}/${tool}`;
  return finding({
    rung, id, line,
    message: String(message).slice(0, 120),
    fix: `resolve the ${tool} finding`,
  });
}

// The two decline conditions eslint signals, as a reason string or null. Split
// out of parse() so the batch path can ask the same question per result without
// throwing: in a batch, a throw would take every other file down with the one
// the linter declined.
function declineReason(messages) {
  for (const m of messages) {
    if (m && m.fatal) return `eslint could not parse it: ${m.message}`;
    if (m && m.ruleId == null && /^File ignored\b/.test(String(m.message || ''))) {
      return 'eslint ignores it';
    }
  }
  return null;
}

// A flat list of tool items, grouped into one entry per file. `fileOf` reads
// the path an item names, `findingOf` converts it, and `declineOf` (optional)
// says whether an item means the linter did not actually lint that file.
//
// An item naming no file is dropped rather than attributed to a guess: a
// finding on the wrong file is worse than a finding nobody sees, because the
// author goes looking at code that is fine.
function groupByFile(items, fileOf, findingOf, declineOf = () => null) {
  const byFile = new Map();
  for (const item of items) {
    const file = fileOf(item);
    if (!file) continue;
    if (!byFile.has(file)) byFile.set(file, { file, declined: null, findings: [] });
    const entry = byFile.get(file);
    const decline = declineOf(item);
    if (decline) { entry.declined = decline; continue; }
    entry.findings.push(findingOf(item));
  }
  return Array.from(byFile.values());
}

// A linter can decline a file instead of judging it: eslint says so out loud
// ("File ignored because of a matching ignore pattern", exit 0), ruff used to
// say so only by printing an empty result set. Either way the tool linted
// nothing, so treating it as "answered, nothing found" hands the file to a
// judge that never looked at it and takes the pack's obvious/* rules down with
// it — the same coverage-deleting shape as the clippy and golangci-lint bugs.
// A parser signals it by throwing, which resolve.js reads as not-ok and the
// caller answers by running the full pack.
function declined(what) {
  throw new Error(`the linter did not lint this file: ${what}`);
}

// Verified against cargo 1.93.1 / clippy 0.1.93: `cargo clippy` cannot be
// scoped to a single FILE — it compiles and lints whole crates, and in a
// workspace plain `cargo clippy` lints every member (editing crate-a reported
// crate-b's warnings too). It CAN be scoped to a single PACKAGE, and that is
// the bound worth taking inside a 1.5s budget: `-p <name>` compiles only the
// member that owns the file. The package is read by walking up from the file
// to the nearest Cargo.toml carrying a [package] name — filesystem only, no
// second subprocess. No manifest, no name, or a name cargo rejects all fall
// back to a run that still answers or still fails loudly; none of them can
// produce a false clean.
const MAX_MANIFEST_BYTES = 512 * 1024;
const MAX_MANIFEST_WALK = 40;

function readManifest(file) {
  try {
    if (fs.statSync(file).size > MAX_MANIFEST_BYTES) return null;
    return fs.readFileSync(file, 'utf8');
  } catch (e) {
    // There is no manifest at this level, or it cannot be read. Either way
    // there is no package name here and the walk continues upward; an
    // unreadable manifest must not stop the search, only fail to answer it.
    return null;
  }
}

function cargoPackageOf(absFile) {
  let dir = path.dirname(String(absFile || ''));
  for (let up = 0; up < MAX_MANIFEST_WALK; up += 1) {
    const text = readManifest(path.join(dir, 'Cargo.toml'));
    const section = text && /^[ \t]*\[package\][^\n]*\n([\s\S]*?)(?=^[ \t]*\[|$)/m.exec(text);
    const name = section && /^[ \t]*name[ \t]*=[ \t]*["']([^"'\n]+)["']/m.exec(section[1]);
    if (name) return name[1];
    const parent = path.dirname(dir);
    if (parent === dir) return null;
    dir = parent;
  }
  return null;
}

// One diagnostic, as {file, line, message}.
//
// `--message-format short` does NOT print the lint name: clippy 0.1.93 emits
// ``crate-a/src/lib.rs:6:5: warning: unneeded `return` statement`` and nothing
// more, so this match fails and the id is the bare `true/clippy`. Only
// `--message-format json` carries `message.code.code`, and cargo writes that
// to STDOUT — moving to it would give up the stderr reading this integration
// exists to guarantee. The trailing-`[rule]` form is still matched because
// rustc prints it in its long format; the gap is recorded, not papered over.
function clippyFinding(d) {
  const ruleMatch = /\[([\w:-]+)\]\s*$/.exec(d.message);
  return externalFinding(d.line, d.message, 'clippy', ruleMatch && ruleMatch[1]);
}

// `--message-format short`: the whole diagnostic on one line.
const CLIPPY_SHORT = /^([^:]+):(\d+):\d+:\s*(?:warning|error):\s*(.+)$/;
// rustc's long rendering: the message, then ` --> file:line:col` under it.
// Neither pattern reads `error[E0425]:` as a diagnostic — a crate that does not
// compile is a crate clippy did not lint, so that output must stay unreadable
// rather than answer for it.
const CLIPPY_LONG_MESSAGE = /^(?:warning|error):\s*(.+)$/;
const CLIPPY_LONG_LOCATION = /^\s*-->\s+(\S+?):(\d+):\d+\s*$/;

// Every diagnostic in clippy's output as {file, line, message}, in WHICHEVER
// of the two formats cargo printed it — because asking for one format is not
// the same as getting it. cargo caches each compilation unit's diagnostics and,
// for a unit it considers fresh, REPLAYS them in the format that originally
// compiled it: `--message-format short` does not re-render a cache some other
// run filled. Verified against cargo 1.93.1 / clippy 0.1.93 — after a plain
// `cargo clippy` by hand, procoder's own run gets rustc's long rendering on
// stderr instead. Reading only the short form meant running your own linter
// silently blinded procoder: either the whole output became unreadable and
// every clippy finding was dropped, or — with one recompiled unit emitting a
// short line for another file — the run read as answered and clean, deleting
// the Rust pack's obvious/* rules for a file that had a warning all along.
// Forcing a fresh compile instead would rebuild the crate on every edit, which
// no 1.5s hook budget can pay.
//
// A message line with no location under it is not a diagnostic: that is how
// cargo's own "warning: `x` (lib) generated 1 warning" tally stays out.
function clippyDiagnostics(lines) {
  const found = [];
  for (let i = 0; i < lines.length; i += 1) {
    const short = CLIPPY_SHORT.exec(lines[i]);
    if (short) {
      found.push({ file: short[1], line: Number(short[2]), message: short[3] });
      continue;
    }
    const head = CLIPPY_LONG_MESSAGE.exec(lines[i]);
    const at = head && CLIPPY_LONG_LOCATION.exec(lines[i + 1] || '');
    if (at) found.push({ file: at[1], line: Number(at[2]), message: head[1] });
  }
  return found;
}

// A crate that did not compile is a crate clippy did not lint, and the run is
// therefore not an answer — whatever else was on the stream.
//
// Neither diagnostic pattern above reads a compile error as a finding, so an
// output carrying ONLY compile errors already fell through to the "no
// diagnostic in output" throw. The gap this closes is the mixed one, verified
// against cargo 1.93.1 / clippy 0.1.93 on a package whose lib unit is fresh and
// whose bin unit does not compile:
//
//   src/lib.rs:2:5: warning: unneeded `return` statement
//   src/main.rs:2:13: error[E0425]: cannot find value `missing_value` ...
//   error: could not compile `demo` (bin "demo") due to 1 previous error
//
// The warning is a cached replay attributed to the file under inspection, so
// parse() returned one finding, and resolve.js reads a non-empty finding list
// as an answer — deleting the Rust pack's obvious/* rules for a crate clippy
// never finished analysing. Exit 101 did not save it either: linters exit
// non-zero when they find something, so the exit code cannot tell the two apart.
//
// Two markers, because rustc's diagnostic codes do not cover every compile
// error — a syntax error has no `E####` — and cargo's summary line does:
//   - `error[E####]:` in either rendering (the short form prefixes it with
//     `file:line:col: `, the long form starts the line with it);
//   - cargo's own `error: could not compile` / rustc's `error: aborting due to`.
//
// The one thing this over-reads: a crate built with `-D warnings` whose denied
// lint fails the build prints "could not compile" too, and clippy DID lint it.
// That crate now falls back to the pack and can see a shape finding twice. It
// is the right way round to be wrong — the invariant is that a tool's failure
// must never REDUCE coverage, and duplication is not a reduction. A clean crate
// under `-D warnings` prints nothing at all and is still answered.
const COMPILE_ERROR = /(?:^|\s)error\[[A-Z]\d+\]:|^error: (?:could not compile|aborting due to)\b/m;

// Whether a path clippy reported names the file under inspection. Reported
// paths are relative to the crate root; the target is absolute.
function sameFile(reported, absPath) {
  const norm = (s) => String(s).replace(/\\/g, '/');
  const r = norm(reported);
  const a = norm(absPath);
  return a === r || a.endsWith(`/${r}`);
}

const TOOLS = {
  // semgrep runs on EVERY source file, alongside whatever language-specific
  // analyzer also claims it. It is the only cross-language security analyzer in
  // the set, and for a long moment it was declared mandatory in toolchain.js,
  // probed for on every check, reported as a gap when absent — and never
  // actually executed, because this registry mapped one tool per extension and
  // the language tool always won. A Go file was checked by golangci-lint, a
  // Python file by ruff, and the analyzer carrying the CWE rules never ran.
  //
  // Only security rulesets are enabled, which is why every semgrep finding is
  // rung 1 in rungFor.
  semgrep: {
    name: 'semgrep',
    // Measured at 1.76s to answer about ONE file, twice in a row, warm. That is
    // rule-loading, not analysis, and it is most of a 2s write-hook budget
    // before it has read a line. So semgrep is a commit-time analyzer here: the
    // CLI (`procoder check`, the pre-commit hook, CI) gives it the time it
    // needs, and the write hook skips it and says so in `unchecked` rather than
    // spawning it to be killed and reporting the corpse as "did not answer".
    //
    // So semgrep is a COMMIT-tier analyzer, declared rather than discovered: the
    // write hook does not run it and does not complain about not running it,
    // because "this file was only partly checked" on every single write is noise
    // that teaches people to stop reading the gate. `procoder check` and CI run
    // it over everything, in one process, with the time it needs.
    tier: 'commit',
    argv: (file) => [
      '--config=p/security-audit', '--config=p/cwe-top-25', '--config=p/secrets',
      '--json', '--quiet', '--no-git-ignore', '--metrics=off', file,
    ],
    argvMany: (files) => [
      '--config=p/security-audit', '--config=p/cwe-top-25', '--config=p/secrets',
      '--json', '--quiet', '--no-git-ignore', '--metrics=off', ...files,
    ],
    parse: (stdout) => {
      const doc = JSON.parse(stdout);
      // A file semgrep could not parse is not a clean file. It names them in
      // `errors`, so this is the one place we can tell the two apart.
      if ((doc.errors || []).length && !(doc.results || []).length) {
        declined('semgrep could not parse it');
      }
      return (doc.results || []).map((r) => externalFinding(
        (r.start && r.start.line) || 0,
        `${r.check_id}: ${(r.extra && r.extra.message) || ''}`.slice(0, 160),
        'semgrep', r.check_id));
    },
    parseMany: (stdout) => {
      const doc = JSON.parse(stdout);
      return groupByFile(doc.results || [], (r) => r.path,
        (r) => externalFinding((r.start && r.start.line) || 0,
          `${r.check_id}: ${(r.extra && r.extra.message) || ''}`.slice(0, 160),
          'semgrep', r.check_id));
    },
  },
  py: {
    name: 'ruff',
    tier: 'write',
    // ruff reads pyproject.toml, ruff.toml and .ruff.toml — and nothing else.
    // Verified against ruff 0.16.3: a setup.cfg carrying `[ruff] line-length =
    // 20` changes nothing about the run. Counting setup.cfg as evidence made
    // procoder defer its obvious/* rules to a ruff running on defaults, whose
    // default rule set (E4, E7, E9, F) contains no shape rule at all — a
    // flake8-configured repo was checked LESS than a repo with no linter.
    configFiles: ['ruff.toml', '.ruff.toml', 'pyproject.toml'],
    // No --force-exclude. Verified against ruff 0.16.3: with it, a file the
    // project's `exclude` covers is answered with `[]` and exit 0 — byte for
    // byte what a clean file produces — so procoder read "clean", deferred and
    // dropped the pack's obvious/* rules for a file ruff never opened. Without
    // it, an explicitly named path is always linted, so `[]` means clean and
    // nothing else. A path the project excludes from ruff is procoder's to
    // exclude in .procoder.toml, not ruff's to silence procoder with.
    // S is flake8-bandit — the security rule set. Without it ruff runs E4/E7/E9/F
    // and reports no security finding of any kind, which is how an analyzer can
    // be installed, configured, green, and blind. See toolchain.js, COMPLETE.
    argv: (file) => ['check', '--select', 'S,B,E,F', '--output-format', 'json', file],
    // Many files, one process. Every ruff item names its own `filename`, so a
    // batch is attributable without guessing — see parseMany below and
    // runToolBatch in resolve.js for why the CLI wants this and the hook does
    // not.
    argvMany: (files) => ['check', '--select', 'S,B,E,F', '--output-format', 'json', ...files],
    parseMany: (stdout) => groupByFile(JSON.parse(stdout), (item) => item.filename,
      (item) => externalFinding(item.location && item.location.row,
        `${item.code}: ${item.message}`, 'ruff', item.code),
      (item) => (item && item.code === 'invalid-syntax' ? 'ruff could not parse it' : null)),
    // parse() must THROW on output it cannot read, never return []. Returning
    // [] tells resolve.js the tool answered and found nothing, which skips the
    // built-in pack too — so a linter that printed a flag error would delete
    // the rung instead of falling back to it. A genuinely empty result set is
    // still `[]` on the wire and still parses to no findings.
    parse: (stdout) => JSON.parse(stdout).map((item) => {
      // ruff 0.16.3 reports a file it could not parse as a single
      // `invalid-syntax` item and lints nothing else in it. Reporting that one
      // item and calling the run answered would defer the shape rules to a run
      // that never happened; the pack's regex rules still read a broken file.
      if (item && item.code === 'invalid-syntax') declined('ruff could not parse it');
      return externalFinding(item.location && item.location.row, `${item.code}: ${item.message}`, 'ruff', item.code);
    }),
  },
  ts: {
    name: 'eslint',
    tier: 'write',
    // Flat config in every extension eslint loads it from — eslint 10 reads
    // .ts/.mts/.cts config natively, and dropped .eslintrc entirely. The
    // eslintrc names stay: on eslint 8 they are still the config, and on
    // eslint 10 a repo that has only one of them makes eslint exit 2 with an
    // empty stdout, which resolve.js already reads as not-ok — the pack runs.
    configFiles: [
      'eslint.config.js', 'eslint.config.mjs', 'eslint.config.cjs',
      'eslint.config.ts', 'eslint.config.mts', 'eslint.config.cts',
      '.eslintrc', '.eslintrc.json', '.eslintrc.cjs', '.eslintrc.js',
      '.eslintrc.yml', '.eslintrc.yaml',
    ],
    argv: (file) => ['--format', 'json', file],
    argvMany: (files) => ['--format', 'json', ...files],
    // eslint's JSON is already grouped: one result object per file, carrying
    // `filePath`. The decline rules are the single-file ones, applied per
    // result — a file eslint ignores must take itself out of the batch, and
    // nothing else with it.
    parseMany: (stdout) => JSON.parse(stdout).map((result) => ({
      file: result.filePath,
      declined: declineReason(result.messages || []),
      findings: (result.messages || []).map((m) =>
        externalFinding(m.line, `${m.ruleId || 'eslint'}: ${m.message}`, 'eslint', m.ruleId)),
    })),
    parse: (stdout) => JSON.parse(stdout).flatMap((result) => {
      const messages = result.messages || [];
      // Verified against eslint 10.8.1. Two answers mean eslint linted nothing:
      //   - an ignored file: one warning, ruleId null, NO `line` field, exit 0.
      //     The line-less finding was dropped by the runner's `line > 0` filter
      //     and the zero exit read as clean, so every project with an `ignores`
      //     list silently lost the pack's obvious/* rules on those files.
      //   - a file that does not parse: one `fatal` message and no lint results.
      for (const m of messages) {
        if (m && m.fatal) declined(`eslint could not parse it: ${m.message}`);
        if (m && m.ruleId == null && /^File ignored\b/.test(String(m.message || ''))) {
          declined('eslint ignores it');
        }
      }
      return messages.map((m) =>
        externalFinding(m.line, `${m.ruleId || 'eslint'}: ${m.message}`, 'eslint', m.ruleId));
    }),
  },
  go: {
    name: 'golangci-lint',
    tier: 'write',
    configFiles: ['.golangci.yml', '.golangci.yaml', '.golangci.toml'],
    // --enable=gosec, always. gosec is golangci-lint's ONLY security linter and
    // it is not in the default set — the defaults are errcheck, govet,
    // ineffassign, staticcheck and unused, none of which reports a weakness. A
    // Go file checked without this flag is checked by a green, blind gate:
    // measured on CWEval, `md5.Sum` in a hashing function produced no finding at
    // all, and gosec reports it as G401/G501 at HIGH confidence. Same failure
    // ruff has without --select S, and the same fix. See toolchain.js, COMPLETE.
    argv: (file) => golangciMajorVersion() >= 2
      ? ['run', '--enable=gosec', '--output.json.path', 'stdout', file]
      : ['run', '--enable=gosec', '--out-format', 'json', file],
    parse: (stdout) => (golangciReport(stdout).Issues || []).map((issue) =>
      externalFinding(issue.Pos && issue.Pos.Line, `${issue.FromLinter}: ${issue.Text}`, 'golangci-lint', issue.FromLinter)),
    argvMany: (files) => (golangciMajorVersion() >= 2
      ? ['run', '--enable=gosec', '--output.json.path', 'stdout', ...files]
      : ['run', '--enable=gosec', '--out-format', 'json', ...files]),
    parseMany: (stdout) => groupByFile(golangciReport(stdout).Issues || [],
      (issue) => issue.Pos && issue.Pos.Filename,
      (issue) => externalFinding(issue.Pos && issue.Pos.Line,
        `${issue.FromLinter}: ${issue.Text}`, 'golangci-lint', issue.FromLinter)),
  },
  // There is no standalone `clippy` binary on a normal PATH — only
  // `cargo-clippy`, which `cargo clippy` dispatches to. The binary to
  // invoke (and to probe for with `which`, in resolve.js) is `cargo`, with
  // `clippy` as its first argument.
  //
  // cargo clippy has no per-file scoping: unlike eslint/ruff/golangci-lint,
  // which take a single file as their argv target, clippy always compiles
  // and lints whole crates. Verified against cargo 1.93.1 / clippy 0.1.93,
  // and it is not merely cosmetic: in a two-member workspace, plain
  // `cargo clippy` reported crate-b's warnings while crate-a was the file
  // being written. It CAN be scoped to a package, so argv() names one with
  // `-p` — see cargoPackageOf. That bounds the compile to the member that
  // owns the file instead of the whole workspace.
  //
  // Attribution is belt to that brace: rustTarget records the absolute path
  // argv() was called for, and parse() discards every finding whose reported
  // path isn't that file, so a warning in src/other.rs is never presented as
  // a fact about the file actually being written.
  rust: (() => {
    let rustTarget = null;
    return {
      name: 'cargo',
    tier: 'write',
      // cargo clippy writes its diagnostics to stderr, not stdout, and exits 0
      // even when it found something. Read on stdout it looks like a clean
      // crate, which would silently take the Rust pack's obvious/* rules down
      // with it — see resolve.js.
      stream: 'stderr',
      configFiles: ['clippy.toml', '.clippy.toml', 'Cargo.toml'],
      argv: (file) => {
        rustTarget = file;
        const pkg = cargoPackageOf(file);
        const scope = pkg ? ['-p', pkg] : [];
        return ['clippy', ...scope, '--message-format', 'short', '--quiet'];
      },
      parse: (output) => {
        const text = String(output);
        // Checked before the diagnostics, and regardless of them: a warning
        // replayed from cache next to a compile error is still not an answer.
        if (COMPILE_ERROR.test(text)) declined('the crate did not compile');
        const diagnostics = clippyDiagnostics(text.split('\n'));
        // `--quiet` means a clean crate prints nothing at all, so any non-empty
        // output carrying no diagnostic is output we could not read: a compile
        // error, a lock-wait notice, a panic. Throwing says so; returning []
        // would claim the crate is clean and skip the pack.
        if (!diagnostics.length && text.trim()) throw new Error('no clippy diagnostic in output');
        return diagnostics
          .filter((d) => !rustTarget || sameFile(d.file, rustTarget))
          .map(clippyFinding);
      },
    };
  })(),
};

// The extensions each analyzer answers for. These used to be read off the
// language packs; the packs are gone, so they are stated here, which is where a
// reader looks for them anyway.
const TOOL_EXTENSIONS = [
  [TOOLS.ts, ['.ts', '.tsx', '.mts', '.cts', '.js', '.jsx', '.mjs', '.cjs']],
  [TOOLS.py, ['.py', '.pyi']],
  [TOOLS.go, ['.go']],
  [TOOLS.rust, ['.rs']],
];

const EXT_TO_TOOL = new Map();
for (const [tool, exts] of TOOL_EXTENSIONS) {
  for (const ext of exts) EXT_TO_TOOL.set(ext, tool);
}
// Java, Kotlin and C# have no single-file analyzer fast enough for the budget,
// and C/C++ have none that proves anything — toolchain.js UNGATED says so to the
// user rather than letting the silence read as a pass.

function toolFor(relPath) {
  return EXT_TO_TOOL.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

// Every analyzer that answers for this file, not just the language-specific one.
// semgrep is appended for any extension procoder treats as source, because its
// rules are the cross-language security set and no language tool replaces them.
function toolsFor(relPath) {
  const out = [];
  const lang = toolFor(relPath);
  if (lang) out.push(lang);
  if (EXT_TO_TOOL.has(path.extname(String(relPath || '')).toLowerCase()) || SEMGREP_EXT.has(path.extname(String(relPath || '')).toLowerCase())) {
    out.push(TOOLS.semgrep);
  }
  return out;
}

// The extensions semgrep is worth spawning for. Broader than the language tools
// above, because semgrep covers languages procoder has no other analyzer for.
const SEMGREP_EXT = new Set([
  '.py', '.pyi', '.js', '.jsx', '.mjs', '.cjs', '.ts', '.tsx', '.mts', '.cts',
  '.go', '.rs', '.java', '.kt', '.cs', '.rb', '.php', '.c', '.h', '.cpp', '.cc',
  '.cxx', '.hpp', '.scala', '.swift',
]);

module.exports = { TOOLS, toolFor, toolsFor, SEMGREP_EXT };
