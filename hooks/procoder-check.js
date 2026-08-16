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

  const { relPath, findings, skipped } = checkFile(absPath, { repoRoot, config });
  if (skipped || findings.length === 0) return;

  const header = `procoder [${level}] — ${findings.length} finding${findings.length === 1 ? '' : 's'} in ${relPath}. Fix these before moving on:`;
  writeHookOutput('PostToolUse', level, header + '\n' + formatFindings(findings, relPath));
}).catch(() => process.exit(0));
