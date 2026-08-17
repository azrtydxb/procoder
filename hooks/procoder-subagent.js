#!/usr/bin/env node
// procoder — SubagentStart hook. A subagent inherits the doctrine; without this
// it writes code the main session would have gated.
//
// It inherits the digest, not the whole text. The session pays for the doctrine
// once; this hook pays per subagent, so a twelve-agent workflow pays twelve
// times, and that multiplier is the only place trimming is worth anything.
//
// Three kinds of text are marked `digest:skip` in the doctrine, and the reason
// is the same each time — a subagent cannot act on it:
//
//   session mechanics   how to switch level, what the levels mean, the
//                       cross-turn self-correction rule. A subagent runs one
//                       turn and is told its level by the line above this text.
//   engine-computed     the rung-3 shape thresholds and the per-language
//                       suppression spellings. The PostToolUse hook runs on the
//                       subagent's own writes and reports those with real
//                       numbers and the rule id, which beats a table it read
//                       beforehand.
//   reporting format    the finding layout and the ponytail tie-breakers. Every
//                       skill that reports findings carries the format in its
//                       own prompt; an implementation subagent never emits it.
//
// Every rung, and every rule the engine cannot compute, is in the digest. The
// marker polarity guarantees it: a rule added tomorrow is included unless
// somebody explicitly marks it out. ~24% smaller than the full text.

const { getProcoderInstructions } = require('./procoder-instructions');
const { readHookInput, readLevel, writeHookOutput } = require('./procoder-runtime');

// Before anything that can exit — see procoder-activate.js. The doctrine a
// subagent inherits is the session's, not a function of which subagent it is,
// so the payload is read and dropped.
readHookInput();

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

const level = readLevel();
if (level === 'off') process.exit(0);

writeHookOutput('SubagentStart', level, getProcoderInstructions(level, { digest: true }));
