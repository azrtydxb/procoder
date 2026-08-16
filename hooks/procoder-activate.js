#!/usr/bin/env node
// procoder — SessionStart activation hook.
//   1. Resolves the active level (env > persisted)
//   2. Persists it so the statusline can read it
//   3. Emits the level-filtered doctrine as session context

const { normalizeLevel } = require('./procoder-config');
const { getProcoderInstructions } = require('./procoder-instructions');
const { clearLevel, setLevel, readLevel, writeHookOutput } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

// An explicit env level wins over whatever the last session persisted.
const envLevel = normalizeLevel(process.env.PROCODER_DEFAULT_LEVEL);

// PROCODER_DEFAULT_LEVEL=off: the environment is authoritative, so a level
// persisted by an earlier session is stale — drop it.
if (envLevel === 'off') {
  clearLevel();
  writeHookOutput('SessionStart', 'off', '');
  process.exit(0);
}

// readLevel() already falls back to the default level internally, so it
// never returns a falsy value.
const level = envLevel || readLevel();

// 'off' read from the file is the user's own "stop procoder". Leave the file
// alone: clearing it would make readLevel() fall back to the default and
// silently re-activate procoder on the next session.
if (level === 'off') {
  writeHookOutput('SessionStart', 'off', '');
  process.exit(0);
}

setLevel(level);
writeHookOutput('SessionStart', level, getProcoderInstructions(level));
