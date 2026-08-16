#!/usr/bin/env node
// procoder — SubagentStart hook. A subagent inherits the doctrine; without this
// it writes code the main session would have gated.

const { getProcoderInstructions } = require('./procoder-instructions');
const { readLevel, writeHookOutput } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

const level = readLevel();
if (level === 'off') process.exit(0);

writeHookOutput('SubagentStart', level, getProcoderInstructions(level));
