#!/usr/bin/env node
// procoder — PostToolUse hook.
//
// Runs on the single file Claude just wrote. Emits findings as context, never
// a block: the model fixes them in the same turn, which is cheaper than a
// rejected write and does not strand the user behind a false positive.

const path = require('path');
const { loadConfig, findRepoRoot } = require('./checks/config');
const { checkFile } = require('./checks/run');
const { formatFindings } = require('./checks/finding');
const { readHookInput, writeHookOutput, readLevel } = require('./procoder-runtime');

// Read first, exit second: PROCODER_NO_HOOK used to exit with the payload still
// in the pipe, which fails the host's write with EPIPE. See procoder-runtime.js.
const input = readHookInput();

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

// Spec §3.2: keep only findings inside the region the tool call touched. An
// Edit names the text it wrote, so the region is that text's span; a Write
// wrote the whole file and every line is touched. Any other tool shape whose
// payload does not name what it wrote falls back to the whole file — the
// mitigation for false positives must not itself hide a real one.
function touchedTexts(toolInput) {
  if (!toolInput) return null;
  if (Array.isArray(toolInput.edits)) {
    return toolInput.edits.map((e) => e && (e.new_string || e.new_source)).filter(Boolean);
  }
  const written = toolInput.new_string || toolInput.new_source;
  return written ? [written] : null;
}

// Spec §2.8: `pragmatic` enforces rungs 1-2 and merely flags 3-4. The severity
// map says which rungs are judgment calls; the level says whether this session
// wants judgment calls treated as must-fix. strict and paranoid always do.
function isBlocking(finding, level, config) {
  if (level !== 'pragmatic') return true;
  return config.rungs[finding.rung.toLowerCase()] !== 'warn';
}

function buildMessage({ level, config, relPath, findings, staleBaseline }) {
  const blocking = findings.filter((f) => isBlocking(f, level, config));
  const advisory = findings.filter((f) => !isBlocking(f, level, config));

  const lines = [];
  if (blocking.length) {
    lines.push(`procoder [${level}] — ${blocking.length} finding${blocking.length === 1 ? '' : 's'} in ${relPath}. Fix these before moving on:`);
    lines.push(formatFindings(blocking, relPath));
  }
  if (advisory.length) {
    lines.push('Flagged, not blocking:');
    lines.push(formatFindings(advisory, relPath));
  }
  if (staleBaseline) {
    lines.push(`Baseline format changed (v${staleBaseline}) — pre-existing findings are no longer suppressed; run \`procoder baseline ${relPath}\` to restore it.`);
  }
  return lines.join('\n');
}

function checkWrittenFile() {
  const level = readLevel();
  if (level === 'off') return;

  const filePath = (input.tool_input && input.tool_input.file_path) || '';
  if (!filePath) return;

  const absPath = path.isAbsolute(filePath)
    ? filePath
    : path.resolve(input.cwd || process.cwd(), filePath);

  const repoRoot = findRepoRoot(path.dirname(absPath));
  const config = loadConfig(repoRoot);

  const { relPath, findings, skipped, staleBaseline } = checkFile(absPath, {
    repoRoot, config, touched: touchedTexts(input.tool_input),
  });

  // A skipped file has no findings, and so is indistinguishable from a clean one
  // on the channel the model reads — "too large to check" and "nothing wrong"
  // are opposite answers arriving as the same silence. The reason goes to
  // stderr: it reaches the transcript for anyone wondering why a file stopped
  // being gated, without spending model context on every deliberately excluded
  // file. Findings themselves never travel this channel.
  if (skipped) {
    process.stderr.write(`procoder: skipped ${relPath} (${skipped})\n`);
    return;
  }

  // Findings are the only trigger for output, which also keeps the stale-baseline
  // notice off clean writes instead of on every write forever.
  if (findings.length === 0) return;

  writeHookOutput('PostToolUse', level,
    buildMessage({ level, config, relPath, findings, staleBaseline }));
}

// A hook that throws is a hook that breaks the user's session, whatever the
// payload said or the file under it contained.
try {
  checkWrittenFile();
} catch (e) {
  process.exit(0);
}
