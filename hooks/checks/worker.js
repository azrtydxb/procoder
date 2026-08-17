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

const { loadConfig } = require('./config');
const { checkFile } = require('./run');

// The deadline exists to keep the PostToolUse hook inside its 2s window. A CLI
// slice is not that: it is a batch job whose findings are the point, and a
// budget that cut the language pack partway through a repository scan would
// make the report depend on how busy the machine was. Large enough to be no
// limit in practice, and still a limit.
const SLICE_BUDGET_MS = 10 * 60 * 1000;

function readStdin() {
  return new Promise((resolve) => {
    let raw = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => { raw += chunk; });
    process.stdin.on('end', () => resolve(raw));
  });
}

function scan({ repoRoot, files, noIgnore, applyBaseline, toolAnswers }) {
  const config = { ...loadConfig(repoRoot), noIgnore };
  const answers = new Map(toolAnswers || []);
  return files.map((absPath) => {
    const out = checkFile(absPath, {
      repoRoot,
      config,
      maxFindings: Infinity,
      applyBaseline,
      budgetMs: SLICE_BUDGET_MS,
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
