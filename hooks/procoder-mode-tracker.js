#!/usr/bin/env node
// procoder — UserPromptSubmit hook. Catches level switches and deactivation
// without needing a round-trip through the model.

const { parseLevelCommand, isDeactivationCommand } = require('./procoder-config');
const { readHookInput, readLevel, setLevel, writeHookOutput } = require('./procoder-runtime');
const { getSafeFirstImperative } = require('./procoder-instructions');

// Read first, exit second: PROCODER_NO_HOOK used to exit with the payload still
// in the pipe, which fails the host's write with EPIPE. See procoder-runtime.js.
const input = readHookInput();

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

function trackMode() {
  const prompt = input.prompt || '';

  // Deactivation must be PERSISTED as the literal level 'off', not deleted:
  // readLevel() treats a missing file as "use the default", so clearLevel()
  // here would silently re-activate procoder at the default level.
  if (isDeactivationCommand(prompt)) {
    setLevel('off');
    writeHookOutput('UserPromptSubmit', 'off', '');
    return;
  }

  const requested = parseLevelCommand(prompt);
  if (requested === 'off') {
    setLevel('off');
    writeHookOutput('UserPromptSubmit', 'off', '');
    return;
  }
  if (requested) {
    setLevel(requested);
    writeHookOutput('UserPromptSubmit', requested,
      `procoder level is now ${requested}.`);
    return;
  }

  // Ordinary prompt: nothing changes, but hosts that render the level from
  // hook output (Codex) must be told the real one, not a hardcoded guess.
  // The rung-1 imperative rides along so it sits next to the request rather
  // than only in the session preamble — worth ~4pp of secure code on CWEval,
  // for ~70 tokens a turn.
  const level = readLevel();
  writeHookOutput('UserPromptSubmit', level,
    level === 'off' ? '' : getSafeFirstImperative());
}

// A hook that throws is a hook that breaks the user's session, whatever the
// payload said.
try {
  trackMode();
} catch (e) {
  process.exit(0);
}
