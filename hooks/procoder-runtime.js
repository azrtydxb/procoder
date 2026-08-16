// procoder — level persistence and the host-specific hook stdout protocol.

const fs = require('fs');
const path = require('path');
const tty = require('tty');
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
    // A payload that is valid JSON but not an object (`7`, `"x"`, `null`) is no
    // more usable than none, and callers reach straight for properties.
    const parsed = JSON.parse(data);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (e) {
    debugWarn('unparseable hook payload', e);
    return {};
  }
}

// Well inside the 2s PostToolUse budget. Only an fd 0 that answers EAGAIN gets
// anywhere near it; a normal pipe or file never waits at all.
const READ_DEADLINE_MS = 1000;

// Synchronous sleep, so an EAGAIN retry does not burn a core spinning. Node has
// no sleepSync; Atomics.wait on a buffer nobody else can notify is the stdlib
// spelling of one.
function sleepSync(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

// Drains fd 0 to EOF. readFileSync(0) would do this in one line but gives up
// everything it already read the moment fd 0 answers EAGAIN — which is exactly
// how a non-blocking stdin says "not yet", not "nothing". Reading it by hand is
// what makes a slow writer cost a retry instead of the whole payload.
function drainStdin() {
  const chunks = [];
  const buffer = Buffer.alloc(65536);
  const deadline = Date.now() + READ_DEADLINE_MS;
  for (;;) {
    let bytes;
    try {
      bytes = fs.readSync(0, buffer, 0, buffer.length, null);
    } catch (e) {
      // EOF is how Windows reports a closed pipe. EAGAIN means the writer has
      // not caught up yet; past the deadline we keep what already arrived
      // rather than hold the session open for a writer that may never come.
      // Anything else (EBADF on a closed fd 0) is the caller's {} to return.
      if (e.code === 'EOF') break;
      if (e.code !== 'EAGAIN') throw e;
      if (Date.now() >= deadline) break;
      sleepSync(5);
      continue;
    }
    if (bytes === 0) break;
    chunks.push(Buffer.from(buffer.subarray(0, bytes)));
  }
  return Buffer.concat(chunks).toString('utf8');
}

// Reads the hook payload Claude Code writes to stdin, and must be called by
// EVERY hook entry point before it can exit: the host writes the payload into a
// pipe, so a hook that exits with bytes still in it closes the read end under
// the writer and fails the caller's write with EPIPE. Reading and discarding is
// a complete use of the payload for a hook that has no use for its contents.
//
// Four states, one shape of answer — an object, never a throw:
//   payload present  parsed, whatever its size
//   stdin empty      EOF straight away  → {}
//   stdin closed     readSync EBADF     → {}
//   interactive tty  no payload is coming and reading would block the session
//                    (or throw EAGAIN once Node marks the tty non-blocking), so
//                    it is not read at all
function readHookInput() {
  try {
    if (tty.isatty(0)) return {};
    const raw = drainStdin();
    return raw.trim() ? parseHookPayload(raw) : {};
  } catch (e) {
    debugWarn('could not read hook input', e);
    return {};
  }
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
