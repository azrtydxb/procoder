#!/usr/bin/env node
// procoder — SubagentStart hook. A subagent inherits the doctrine; without this
// it writes code the main session would have gated.

const { getProcoderInstructions } = require('./procoder-instructions');
const { readHookInput, readLevel, writeHookOutput } = require('./procoder-runtime');

// Before anything that can exit — see procoder-activate.js. The doctrine a
// subagent inherits is the session's, not a function of which subagent it is,
// so the payload is read and dropped.
readHookInput();

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

const level = readLevel();
if (level === 'off') process.exit(0);

writeHookOutput('SubagentStart', level, getProcoderInstructions(level));
