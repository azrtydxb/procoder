#!/usr/bin/env node
// procoder — level persistence and the host-specific hook stdout protocol.

const fs = require('fs');
const path = require('path');
const { getLevelFilePath, normalizeLevel, getDefaultLevel } = require('./procoder-config');

// Host detection mirrors ponytail: the same hook scripts are reused across
// agents that each read hook output differently.
const isCodex = !!process.env.CODEX_HOME || process.env.PROCODER_HOST === 'codex';
const isCopilot = process.env.PROCODER_HOST === 'copilot';
const isQoder = process.env.PROCODER_HOST === 'qoder';

// A closed stdout (the host exiting first) surfaces as an async 'error' event,
// not a synchronous throw — try/catch alone cannot catch it, and an uncaught
// EPIPE would crash the hook. Both guards are needed.
try { process.stdout.on('error', () => {}); } catch (e) { /* best-effort */ }

function readLevel() {
  try {
    const raw = fs.readFileSync(getLevelFilePath(), 'utf8');
    return normalizeLevel(raw) || getDefaultLevel();
  } catch (e) {
    return getDefaultLevel();
  }
}

function setLevel(level) {
  const normalized = normalizeLevel(level);
  if (!normalized) return;
  try {
    const file = getLevelFilePath();
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, normalized + '\n');
  } catch (e) {
    // Best-effort: a read-only config dir must not break the session.
  }
}

function clearLevel() {
  try {
    fs.unlinkSync(getLevelFilePath());
  } catch (e) {
    // Already gone, or unwritable. Either way there is nothing to do.
  }
}

// Reads the hook payload Claude Code writes to stdin. Any malformed or absent
// input yields {} so callers can use plain property access without guards.
function readHookInput() {
  return new Promise((resolve) => {
    let data = '';
    const done = (value) => resolve(value);
    try {
      process.stdin.setEncoding('utf8');
      process.stdin.on('data', (chunk) => { data += chunk; });
      process.stdin.on('end', () => {
        try { done(JSON.parse(data) || {}); } catch (e) { done({}); }
      });
      process.stdin.on('error', () => done({}));
    } catch (e) {
      done({});
    }
  });
}

function writeHookOutput(event, level, context = '') {
  try {
    if (isCopilot) {
      process.stdout.write(JSON.stringify(
        event === 'SessionStart' && context ? { additionalContext: context } : {}));
      return;
    }
    if (isCodex) {
      const output = { systemMessage: `PROCODER:${String(level).toUpperCase()}` };
      if (context) {
        output.hookSpecificOutput = { hookEventName: event, additionalContext: context };
      }
      process.stdout.write(JSON.stringify(output));
      return;
    }
    if (isQoder) {
      const output = context
        ? { hookSpecificOutput: { hookEventName: event, additionalContext: context } }
        : {};
      process.stdout.write(JSON.stringify(output));
      return;
    }
    // Native Claude Code: SessionStart accepts raw stdout, but SubagentStart and
    // PostToolUse drop the context unless it is wrapped in hookSpecificOutput.
    if (event === 'SessionStart') {
      process.stdout.write(context);
      return;
    }
    process.stdout.write(JSON.stringify(
      { hookSpecificOutput: { hookEventName: event, additionalContext: context } }));
  } catch (e) {
    // EPIPE at hook exit must not surface as a hook failure.
  }
}

module.exports = {
  isCodex,
  isCopilot,
  isQoder,
  readLevel,
  setLevel,
  clearLevel,
  readHookInput,
  writeHookOutput,
};
