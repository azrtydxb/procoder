#!/usr/bin/env node
// procoder — external tooling detection and invocation.
//
// The ladder applies to procoder itself: if the project already has a linter
// configured, that linter's rules ARE the project's definition of correct for
// the OBVIOUS rung. Re-implementing its shape thresholds would create exactly
// the duplicate-rule rot rung 4 forbids. The SAFE rung is never deferred —
// see run.js.

const fs = require('fs');
const path = require('path');
const { execFileSync, spawnSync } = require('child_process');
const { toolFor } = require('./registry');

const WHICH = process.platform === 'win32' ? 'where' : 'which';

const toolCache = new Map();

function hasTool(name) {
  // Keyed by PATH as well as name: the answer is only valid for the PATH that
  // produced it, and a stale hit would outlive any change to it.
  const key = `${name}\0${process.env.PATH || ''}`;
  if (toolCache.has(key)) return toolCache.get(key);
  let found = false;
  try {
    execFileSync(WHICH, [name], { stdio: 'ignore', timeout: 1000 });
    found = true;
  } catch (e) {
    found = false;
  }
  toolCache.set(key, found);
  return found;
}

// Some entries in a tool's configFiles list are shared ecosystem manifests that
// exist in essentially every repo of their language — pyproject.toml, Cargo.toml
// — whether or not the linter is configured. Existence alone is no evidence, so
// those files must actually contain the tool's section. Files that exist for one
// linter and nothing else (ruff.toml, .eslintrc.json) are evidence by existence.
const EVIDENCE = {
  'pyproject.toml': /^\s*\[tool\.ruff\b/m,
  'setup.cfg': /^\s*\[(?:ruff|flake8)\]/m,
  'Cargo.toml': /^\s*\[(?:lints\.clippy|workspace\.lints\.clippy|package\.metadata\.clippy)\b/m,
};

const MAX_CONFIG_BYTES = 512 * 1024;

function hasEvidence(absFile, marker) {
  try {
    if (fs.statSync(absFile).size > MAX_CONFIG_BYTES) return false;
    return marker.test(fs.readFileSync(absFile, 'utf8'));
  } catch (e) {
    return false;
  }
}

function isConfigured(repoRoot, tool) {
  if (!tool || !tool.configFiles) return false;
  return tool.configFiles.some((file) => {
    const abs = path.join(repoRoot, file);
    if (!fs.existsSync(abs)) return false;
    const marker = EVIDENCE[path.basename(file)];
    return marker ? hasEvidence(abs, marker) : true;
  });
}

function resolveFor(relPath, { repoRoot }) {
  const tool = toolFor(relPath);
  if (!tool) return null;
  if (!isConfigured(repoRoot, tool)) return null;
  if (!hasTool(tool.name)) return null;
  return tool;
}

const MAX_TOOL_OUTPUT_BYTES = 4 * 1024 * 1024;

// Not every linter reports on stdout. cargo clippy writes every diagnostic to
// stderr and still exits 0, so a runner that reads stdout only sees an empty
// string from a clean exit — indistinguishable from "the crate is clean", and
// the caller then skips the built-in Rust pack as well. Configuring clippy
// would make procoder check LESS than not configuring it. A tool entry says
// which stream carries its findings; the other one is never even piped, so
// nothing a linter chatters on the stream we did not ask for can be mistaken
// for a report.
function reportingStdio(tool) {
  return tool.stream === 'stderr'
    ? ['ignore', 'ignore', 'pipe']
    : ['ignore', 'pipe', 'ignore'];
}

// Security: spawnSync with an argv array and shell:false (never exec, never a
// shell string) means no shell is involved, so no shell injection is possible.
// argv is built from the tool definition and a filesystem path only — never
// from file contents — so a repo cannot smuggle a command line through a
// file's content. timeout and maxBuffer bound the wall-clock and memory cost
// of a runaway linter inside the hook budget; maxBuffer is enforced per stream
// by spawnSync, so capturing stderr cannot grow unbounded either.
//
// spawnSync rather than execFileSync — which is spawnSync plus a throw, with
// the same no-shell guarantee — because execFileSync returns only stdout, and
// discards stderr entirely on a zero exit. That is exactly clippy's case, so
// stderr capture is unreachable through execFileSync.
function spawnTool(tool, { repoRoot, absPath, timeoutMs }) {
  const run = spawnSync(tool.name, tool.argv(absPath), {
    cwd: repoRoot,
    encoding: 'utf8',
    timeout: timeoutMs,
    maxBuffer: MAX_TOOL_OUTPUT_BYTES,
    shell: false,
    stdio: reportingStdio(tool),
  });
  return {
    // Linters exit non-zero when they find something — the output is still
    // there and still useful. A timeout, a missing binary or an overrun
    // maxBuffer leaves it empty or truncated, and sets run.error.
    output: String((tool.stream === 'stderr' ? run.stderr : run.stdout) || ''),
    exitedCleanly: !run.error && run.status === 0,
  };
}

// Returns { findings, ok }. `ok` is false when the run told us nothing rather
// than told us the file is clean — a timeout, a missing binary, a non-zero
// exit with no findings, or output the tool's parser could not read. The
// caller uses that to fall back to the built-in pack: a broken linter must
// degrade to less coverage, never to silence, which in a gate is
// indistinguishable from a pass.
//
// Clean and unreadable are told apart on two signals, never on emptiness of
// the finding list alone:
//   - nothing at all on the reporting stream AND a zero exit — the tool ran,
//     examined the file and had nothing to say. That is clean, ok:true, and
//     the shape rules stay deferred so the file is not double-reported.
//   - anything else — a crash, a timeout, or bytes the parser threw on — is
//     unreadable, ok:false, and the pack covers the file.
// A parser signals "I could not read this" by throwing. Returning [] for
// unparseable bytes would be the defect in miniature: it reads as a clean
// answer and takes the pack down with it.
function runToolResult(tool, { repoRoot, absPath, timeoutMs = 1500 }) {
  let run;
  try {
    run = spawnTool(tool, { repoRoot, absPath, timeoutMs });
  } catch (e) {
    return { findings: [], ok: false };
  }

  if (!run.output.trim()) return { findings: [], ok: run.exitedCleanly };

  try {
    const findings = tool.parse(run.output).filter((f) => f && f.line > 0);
    return { findings, ok: findings.length > 0 || run.exitedCleanly };
  } catch (e) {
    return { findings: [], ok: false };
  }
}

function runTool(tool, opts) {
  return runToolResult(tool, opts).findings;
}

module.exports = { hasTool, isConfigured, resolveFor, runTool, runToolResult };
