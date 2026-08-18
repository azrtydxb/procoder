#!/usr/bin/env node
// procoder — SessionStart activation hook.
//   1. Resolves the active level (env > persisted)
//   2. Persists it so the statusline can read it
//   3. Emits the rung-1 imperative as session context, with an "update
//      available" line in front of it when there is one
//
// It used to emit the whole doctrine, ~6.4 KB of it. Measured on CWEval against
// the same doctrine reduced to its first sentence, the 6.4 KB was worth nothing
// the sentence was not already worth (+2.6pp, p=0.58, n=151). Two of those
// kilobytes per session buy less than the line below.

const { normalizeLevel } = require('./procoder-config');
const { getSafeFirstImperative } = require('./procoder-instructions');
const { clearLevel, setLevel, readLevel, readHookInput, writeHookOutput } = require('./procoder-runtime');
const { updateNotice } = require('./procoder-update-check');

// Before anything that can exit: the host writes the SessionStart payload into
// this process's stdin, and every early return below would otherwise close the
// read end under it and fail its write with EPIPE. Nothing in the payload
// changes what this hook emits — `source` is already filtered by the matcher in
// claude-hooks.json — so it is read and dropped.
readHookInput();

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

// The notice rides in the session context rather than on stderr: stderr from a
// SessionStart hook is not shown to the user unless the hook fails, so a notice
// there would be seen by nobody. In the context, Claude can mention it once, in
// the session it applies to. updateNotice() reads a cache and cannot throw, so
// it adds no latency and cannot cost the session its doctrine.
const notice = updateNotice();
writeHookOutput('SessionStart', level,
  (notice ? notice + '\n\n' : '') + getSafeFirstImperative());
