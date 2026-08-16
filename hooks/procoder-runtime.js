// procoder — level persistence and the host-specific hook stdout protocol.

const fs = require('fs');
const path = require('path');
const { getLevelFilePath, normalizeLevel, getDefaultLevel } = require('./procoder-config');

// Host detection mirrors ponytail: the same hook scripts are reused across
// agents that each read hook output differently.
const isCodex = !!process.env.CODEX_HOME || process.env.PROCODER_HOST === 'codex';
const isCopilot = process.env.PROCODER_HOST === 'copilot';
const isQoder = process.env.PROCODER_HOST === 'qoder';

// Every failure this module can hit is recoverable — a read-only config dir, a
// stdout the host closed early — and none of them may break the session. That
// is not a reason for them to vanish: PROCODER_DEBUG=1 surfaces them, so a
// broken install is diagnosable instead of merely quiet.
function debugWarn(what, error) {
  if (process.env.PROCODER_DEBUG) {
    process.stderr.write(`procoder: ${what}: ${(error && error.message) || error}\n`);
  }
}

// A closed stdout surfaces as an async 'error' event, not a synchronous throw —
// try/catch alone cannot catch it, and an uncaught EPIPE would crash the hook.
// Marked on the stream because tests re-require this module, and an unguarded
// listener per load would cross Node's maxListeners and print a warning.
if (process.stdout && !process.stdout.__procoderEpipeGuarded) {
  process.stdout.__procoderEpipeGuarded = true;
  process.stdout.on('error', (e) => debugWarn('stdout error', e));
}

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
    // A read-only config dir must not break the session; the level simply does
    // not persist past it, and the default applies on the next read.
    debugWarn('could not persist level', e);
  }
}

function clearLevel() {
  try {
    fs.unlinkSync(getLevelFilePath());
  } catch (e) {
    // Already gone is success. Anything else left the file in place, which the
    // next readLevel will report for itself.
    if (e.code !== 'ENOENT') debugWarn('could not clear level', e);
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
    // EPIPE at hook exit must not surface as a hook failure: the host has
    // stopped listening, so there is no one left to deliver this to.
    debugWarn('could not write hook output', e);
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
