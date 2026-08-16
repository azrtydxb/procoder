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

readHookInput().then((input) => {
  const level = readLevel();
  if (level === 'off') return;

  const filePath = (input.tool_input && input.tool_input.file_path) || '';
  if (!filePath) return;

  const absPath = path.isAbsolute(filePath)
    ? filePath
    : path.resolve(input.cwd || process.cwd(), filePath);

  const repoRoot = findRepoRoot(path.dirname(absPath));
  const config = loadConfig(repoRoot);

  const { relPath, findings, skipped } = checkFile(absPath, {
    repoRoot, config, touched: touchedTexts(input.tool_input),
  });
  if (skipped || findings.length === 0) return;

  const header = `procoder [${level}] — ${findings.length} finding${findings.length === 1 ? '' : 's'} in ${relPath}. Fix these before moving on:`;
  writeHookOutput('PostToolUse', level, header + '\n' + formatFindings(findings, relPath));
}).catch(() => process.exit(0));
