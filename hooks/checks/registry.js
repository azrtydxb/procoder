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

const ts = require('./lang/ts');
const py = require('./lang/py');
const go = require('./lang/go');
const rust = require('./lang/rust');
const jvm = require('./lang/jvm');
const dotnet = require('./lang/dotnet');

const PACKS = [ts, py, go, rust, jvm, dotnet];

// External findings land on rung TRUE: a configured linter's rules are the
// project's own definition of correct, and procoder defers to them.
//
// The id carries the tool's own rule id (e.g. `true/eslint:no-eval`), not
// just the tool name. Two different rules firing on the same line must not
// collapse into one baseline fingerprint — that would let baselining one
// rule's hit silently suppress a different rule's hit at the same location
// later, exactly what procoder's own inline-suppression doctrine forbids.
function externalFinding(line, message, tool, ruleId) {
  const id = ruleId ? `true/${tool}:${ruleId}` : `true/${tool}`;
  return finding({
    rung: 'TRUE', id, line,
    message: String(message).slice(0, 120),
    fix: `resolve the ${tool} finding`,
  });
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

// One `file:line:col: warning: text` diagnostic, already matched.
//
// `--message-format short` does NOT print the lint name: clippy 0.1.93 emits
// ``crate-a/src/lib.rs:6:5: warning: unneeded `return` statement`` and nothing
// more, so this match fails and the id is the bare `true/clippy`. Only
// `--message-format json` carries `message.code.code`, and cargo writes that
// to STDOUT — moving to it would give up the stderr reading this integration
// exists to guarantee. The trailing-`[rule]` form is still matched because
// rustc prints it in its long format; the gap is recorded, not papered over.
function clippyFinding(m) {
  const ruleMatch = /\[([\w:-]+)\]\s*$/.exec(m[3]);
  return externalFinding(Number(m[2]), m[3], 'clippy', ruleMatch && ruleMatch[1]);
}

const TOOLS = {
  py: {
    name: 'ruff',
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
    argv: (file) => ['check', '--output-format', 'json', file],
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
    configFiles: ['.golangci.yml', '.golangci.yaml', '.golangci.toml'],
    argv: (file) => golangciMajorVersion() >= 2
      ? ['run', '--output.json.path', 'stdout', file]
      : ['run', '--out-format', 'json', file],
    parse: (stdout) => (golangciReport(stdout).Issues || []).map((issue) =>
      externalFinding(issue.Pos && issue.Pos.Line, `${issue.FromLinter}: ${issue.Text}`, 'golangci-lint', issue.FromLinter)),
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
    const DIAGNOSTIC = /^([^:]+):(\d+):\d+:\s*(?:warning|error):\s*(.+)$/;
    const sameFile = (reported, absPath) => {
      const norm = (s) => String(s).replace(/\\/g, '/');
      const r = norm(reported);
      const a = norm(absPath);
      return a === r || a.endsWith(`/${r}`);
    };
    return {
      name: 'cargo',
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
        const diagnostics = text.split('\n').map((line) => DIAGNOSTIC.exec(line)).filter(Boolean);
        // `--quiet` means a clean crate prints nothing at all, so any non-empty
        // output carrying no diagnostic is output we could not read: a compile
        // error, a lock-wait notice, a panic. Throwing says so; returning []
        // would claim the crate is clean and skip the pack.
        if (!diagnostics.length && text.trim()) throw new Error('no clippy diagnostic in output');
        return diagnostics
          .filter((m) => !rustTarget || sameFile(m[1], rustTarget))
          .map(clippyFinding);
      },
    };
  })(),
};

const EXT_TO_TOOL = new Map();
for (const ext of ts.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.ts);
for (const ext of py.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.py);
for (const ext of go.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.go);
for (const ext of rust.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.rust);
// jvm and dotnet have no fast single-file linter worth a 1.5s budget; their
// built-in packs always run instead.

const EXT_TO_PACK = new Map();
for (const pack of PACKS) {
  for (const ext of pack.EXTENSIONS) EXT_TO_PACK.set(ext, pack);
}

function packFor(relPath) {
  return EXT_TO_PACK.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

function toolFor(relPath) {
  return EXT_TO_TOOL.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

module.exports = { PACKS, TOOLS, packFor, toolFor };
