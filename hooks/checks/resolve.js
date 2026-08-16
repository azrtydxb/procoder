#!/usr/bin/env node
// procoder — external tooling detection and invocation.
//
// The ladder applies to procoder itself: if the project already has a linter
// configured, that linter's rules ARE the project's definition of correct.
// Re-implementing them would create exactly the duplicate-rule rot rung 4
// forbids. The built-in packs exist only for projects with nothing configured.

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const { toolFor } = require('./registry');

const WHICH = process.platform === 'win32' ? 'where' : 'which';

const toolCache = new Map();

function hasTool(name) {
  if (toolCache.has(name)) return toolCache.get(name);
  let found = false;
  try {
    execFileSync(WHICH, [name], { stdio: 'ignore', timeout: 1000 });
    found = true;
  } catch (e) {
    found = false;
  }
  toolCache.set(name, found);
  return found;
}

function isConfigured(repoRoot, tool) {
  if (!tool || !tool.configFiles) return false;
  return tool.configFiles.some((file) => fs.existsSync(path.join(repoRoot, file)));
}

function resolveFor(relPath, { repoRoot }) {
  const tool = toolFor(relPath);
  if (!tool) return null;
  if (!isConfigured(repoRoot, tool)) return null;
  if (!hasTool(tool.name)) return null;
  return tool;
}

// Security: execFileSync (never exec) means no shell is involved, so no shell
// injection is possible. argv is built from the tool definition and a
// filesystem path only — never from file contents — so a repo cannot smuggle
// a command line through a file's content. timeout and maxBuffer bound both
// the wall-clock and memory cost of a runaway linter inside the hook budget.
function runTool(tool, { repoRoot, absPath, timeoutMs = 1500 }) {
  let stdout = '';
  try {
    stdout = execFileSync(tool.name, tool.argv(absPath), {
      cwd: repoRoot,
      encoding: 'utf8',
      timeout: timeoutMs,
      maxBuffer: 4 * 1024 * 1024,
      stdio: ['ignore', 'pipe', 'ignore'],
    });
  } catch (e) {
    // Linters exit non-zero when they find something — the output is still on
    // stdout and still useful. A timeout or missing binary leaves it empty.
    stdout = (e && e.stdout) ? String(e.stdout) : '';
  }

  if (!stdout.trim()) return [];

  try {
    return tool.parse(stdout).filter((f) => f && f.line > 0);
  } catch (e) {
    return [];
  }
}

module.exports = { hasTool, isConfigured, resolveFor, runTool };
