#!/usr/bin/env node
// procoder — extension → pack, and pack → preferred external tool.
//
// The tool entries describe how to INVOKE and PARSE each linter. Whether one is
// actually configured in the project is resolve.js's job.

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

const TOOLS = {
  py: {
    name: 'ruff',
    configFiles: ['ruff.toml', '.ruff.toml', 'pyproject.toml', 'setup.cfg'],
    argv: (file) => ['check', '--output-format', 'json', '--force-exclude', file],
    // parse() must THROW on output it cannot read, never return []. Returning
    // [] tells resolve.js the tool answered and found nothing, which skips the
    // built-in pack too — so a linter that printed a flag error would delete
    // the rung instead of falling back to it. A genuinely empty result set is
    // still `[]` on the wire and still parses to no findings.
    parse: (stdout) => JSON.parse(stdout).map((item) =>
      externalFinding(item.location && item.location.row, `${item.code}: ${item.message}`, 'ruff', item.code)),
  },
  ts: {
    name: 'eslint',
    configFiles: ['eslint.config.js', 'eslint.config.mjs', '.eslintrc', '.eslintrc.json', '.eslintrc.cjs', '.eslintrc.js'],
    argv: (file) => ['--format', 'json', file],
    parse: (stdout) => JSON.parse(stdout).flatMap((result) => (result.messages || []).map((m) =>
      externalFinding(m.line, `${m.ruleId || 'eslint'}: ${m.message}`, 'eslint', m.ruleId))),
  },
  go: {
    name: 'golangci-lint',
    configFiles: ['.golangci.yml', '.golangci.yaml', '.golangci.toml'],
    argv: (file) => golangciMajorVersion() >= 2
      ? ['run', '--output.json.path', 'stdout', file]
      : ['run', '--out-format', 'json', file],
    parse: (stdout) => (JSON.parse(stdout).Issues || []).map((issue) =>
      externalFinding(issue.Pos && issue.Pos.Line, `${issue.FromLinter}: ${issue.Text}`, 'golangci-lint', issue.FromLinter)),
  },
  // There is no standalone `clippy` binary on a normal PATH — only
  // `cargo-clippy`, which `cargo clippy` dispatches to. The binary to
  // invoke (and to probe for with `which`, in resolve.js) is `cargo`, with
  // `clippy` as its first argument.
  //
  // cargo clippy has no per-file scoping: unlike eslint/ruff/golangci-lint,
  // which take a single file as their argv target, clippy always compiles
  // and lints the whole crate. That is a real limitation this fix does not
  // remove — see the report for why the entry stays enabled anyway. What it
  // DOES fix is attribution: rustTarget records the absolute path argv() was
  // called for, and parse() discards every finding whose reported path
  // isn't that file, so a warning in src/other.rs is never presented as a
  // fact about the file actually being written.
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
        return ['clippy', '--message-format', 'short', '--quiet'];
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
          .map((m) => {
            const ruleMatch = /\[([\w:-]+)\]\s*$/.exec(m[3]);
            return externalFinding(Number(m[2]), m[3], 'clippy', ruleMatch && ruleMatch[1]);
          });
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
