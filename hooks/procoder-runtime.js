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

// A hook may be invoked with no stdin at all, or with a partial write. Neither
// is worth failing the session over; callers treat {} as "no payload".
function parseHookPayload(data) {
  try {
    return JSON.parse(data) || {};
  } catch (e) {
    debugWarn('unparseable hook payload', e);
    return {};
  }
}

// Reads the hook payload Claude Code writes to stdin. Any malformed or absent
// input yields {} so callers can use plain property access without guards.
function readHookInput() {
  return new Promise((resolve) => {
    const chunks = [];
    try {
      process.stdin.setEncoding('utf8');
      process.stdin.on('data', (chunk) => chunks.push(chunk));
      process.stdin.on('end', () => resolve(parseHookPayload(chunks.join(''))));
      process.stdin.on('error', () => resolve({}));
    } catch (e) {
      debugWarn('could not read hook input', e);
      resolve({});
    }
  });
}

const hookSpecificOutput = (event, context) =>
  ({ hookEventName: event, additionalContext: context });

const wrapped = (event, context) =>
  ({ hookSpecificOutput: hookSpecificOutput(event, context) });

function codexPayload(event, level, context) {
  const output = { systemMessage: `PROCODER:${String(level).toUpperCase()}` };
  if (context) output.hookSpecificOutput = hookSpecificOutput(event, context);
  return output;
}

// Each host reads hook output differently. This picks the text; writeHookOutput
// does the writing.
function hookOutputText(event, level, context) {
  if (isCopilot) {
    return JSON.stringify(event === 'SessionStart' && context ? { additionalContext: context } : {});
  }
  if (isCodex) return JSON.stringify(codexPayload(event, level, context));
  if (isQoder) return JSON.stringify(context ? wrapped(event, context) : {});
  // Native Claude Code: SessionStart accepts raw stdout, but SubagentStart and
  // PostToolUse drop the context unless it is wrapped in hookSpecificOutput.
  if (event === 'SessionStart') return context;
  return JSON.stringify(wrapped(event, context));
}

function writeHookOutput(event, level, context = '') {
  try {
    process.stdout.write(hookOutputText(event, level, context));
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
