#!/usr/bin/env node
// procoder — one slice of a parallel scan.
//
// Forked by hooks/checks/scan.js, never run by a user. It reads a JSON payload
// on stdin, checks the files it names, and writes the results back as JSON.
//
// A child process rather than a worker thread, for one reason: the packs are
// synchronous CPU and the whole engine is `require`-time state, so a thread
// would share nothing useful and would share a heap that the 1MB file cap is
// sized against. A child is also the failure mode we want — if one dies, the
// parent has a whole result set missing rather than a corrupted one, and falls
// back to scanning that slice itself.

// This file decides NOTHING about how a file is checked. Everything that
// shapes a result — the config, the per-file budget, the baseline, the linter
// answers already collected — arrives in the payload, because the parent may
// have to scan this very slice itself (a worker that dies or wedges) and the
// two answers have to be the same answer.
//
// It used to own two of those decisions and both diverged. It re-read
// `.procoder.toml` from disk, so a caller-built config governed the parent's
// files and not the worker's; and it ran files at a 600,000ms budget against
// the parent's 2,000ms, which meant `true/budget-exhausted` fired on a
// sequential scan and effectively never on a parallel one — the faster path
// silently grinding where the slower one admitted it had given up. See
// SCAN_BUDGET_MS in scan.js for why the parent's number won.
const { loadConfig } = require('./config');
const { checkFile } = require('./run');

function readStdin() {
  return new Promise((resolve) => {
    let raw = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => { raw += chunk; });
    process.stdin.on('end', () => resolve(raw));
  });
}

function scan({ repoRoot, files, config, applyBaseline, budgetMs, toolAnswers }) {
  // The parent always sends its config. Loading one is the fallback for a
  // payload without it — an older parent, or a hand-built call — and never the
  // normal path.
  const settings = config || loadConfig(repoRoot);
  const answers = new Map(toolAnswers || []);
  return files.map((absPath) => {
    const out = checkFile(absPath, {
      repoRoot,
      config: settings,
      maxFindings: Infinity,
      applyBaseline,
      budgetMs,
      toolAnswer: answers.get(absPath) || null,
    });
    return { absPath, relPath: out.relPath, findings: out.findings, skipped: out.skipped };
  });
}

async function main() {
  const payload = JSON.parse(await readStdin());
  process.stdout.write(JSON.stringify(scan(payload)));
}

// A worker that throws must not print half a document: the parent reads its
// exit code, and a non-zero one means "scan this slice yourself".
main().catch((e) => {
  process.stderr.write(`procoder: scan worker failed: ${e.message}\n`);
  process.exit(1);
});
