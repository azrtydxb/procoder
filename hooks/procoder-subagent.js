#!/usr/bin/env node
// procoder — SubagentStart hook. A subagent inherits the rung-1 imperative;
// without this it writes code the main session would have gated.
//
// It used to inherit a digest of the whole doctrine — the six rungs, the trust
// boundary list, the sink table — trimmed to ~4.9 KB and paid for on every
// spawn, so a twelve-agent workflow paid it twelve times. Measured on CWEval,
// against the same doctrine cut down to its first sentence:
//
//   nothing                          61.6% func-sec@1
//   the sentence, via this hook      72.2%   (+10.6pp over nothing, p=0.0095)
//   the sentence + the 6.4 KB        74.8%   (+2.6pp over the sentence, p=0.58)
//
// The sentence is the whole effect. The kilobytes around it are inside the
// noise, and they were charged per spawn.
//
// One thing this hook cannot do: the same sentence delivered in the user turn
// rather than as hook context is worth 87.4% (+15.2pp over this, p=0.0005).
// There is no user turn in a subagent, so that gap is a property of the channel
// and not of the wording — five rephrasings and a task-binding framing all came
// back null. procoder-mode-tracker.js is where the good channel is used.

const { getSafeFirstImperative } = require('./procoder-instructions');
const { readHookInput, readLevel, writeHookOutput } = require('./procoder-runtime');

// Before anything that can exit — see procoder-activate.js. The imperative a
// subagent inherits is the session's, not a function of which subagent it is,
// so the payload is read and dropped.
readHookInput();

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

const level = readLevel();
if (level === 'off') process.exit(0);

writeHookOutput('SubagentStart', level, getSafeFirstImperative());
