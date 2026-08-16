#!/usr/bin/env node
// procoder — UserPromptSubmit hook. Catches level switches and deactivation
// without needing a round-trip through the model.

const { parseLevelCommand, isDeactivationCommand } = require('./procoder-config');
const { readHookInput, setLevel, writeHookOutput } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

readHookInput().then((input) => {
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

  writeHookOutput('UserPromptSubmit', 'strict', '');
}).catch(() => process.exit(0));
