const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');
const readJson = (p) => JSON.parse(fs.readFileSync(path.join(root, p), 'utf8'));

test('plugin.json points at the hooks config that exists', () => {
  const plugin = readJson('.claude-plugin/plugin.json');
  assert.strictEqual(plugin.name, 'procoder');
  assert.strictEqual(plugin.hooks, './hooks/claude-hooks.json');
  assert.ok(fs.existsSync(path.join(root, 'hooks/claude-hooks.json')));
});

test('every hook command references a script that ships', () => {
  const hooks = readJson('hooks/claude-hooks.json');
  const events = Object.keys(hooks.hooks);
  assert.deepStrictEqual(
    events.sort(),
    ['PostToolUse', 'SessionStart', 'SubagentStart', 'UserPromptSubmit']
  );
  const declared = events.flatMap((event) =>
    hooks.hooks[event].flatMap((group) => group.hooks.map((h) => ({ event, ...h }))));
  for (const h of declared) {
    assert.match(h.command, /\$\{CLAUDE_PLUGIN_ROOT\}\/hooks\/procoder-[a-z-]+\.js/);
    assert.ok(h.timeout > 0 && h.timeout <= 5,
      `${h.event} hook timeout ${h.timeout} exceeds its 5s budget`);
    const script = /hooks\/(procoder-[a-z-]+\.js)/.exec(h.command)[1];
    assert.ok(fs.existsSync(path.join(root, 'hooks', script)),
      `${h.event} declares ${script}, which does not ship`);
  }
});

// The invariant is that INSTALLING procoder installs nothing else: it is a
// plugin, and a hook that had to npm-install before it could answer would blow
// its 5s budget on the first write of every session.
//
// devDependencies are a different question and are now non-empty on purpose.
// procoder mandates analyzers (see hooks/checks/toolchain.js) and gates its own
// source with them, so the analyzers it demands of a JavaScript project are the
// ones it has to have installed for itself. Anything else appearing here is the
// thing this test is really watching for.
test('procoder installs nothing at runtime, and dev-depends only on its own analyzers', () => {
  const pkg = readJson('package.json');
  assert.strictEqual(pkg.dependencies, undefined,
    'a runtime dependency would have to be installed before a hook could run');
  assert.deepStrictEqual(
    Object.keys(pkg.devDependencies || {}).sort(),
    ['eslint', 'eslint-plugin-security'],
    'the only dev dependencies are the analyzers procoder mandates for JavaScript');
});
