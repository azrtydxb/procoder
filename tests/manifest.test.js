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
  // PostToolUse is deliberately absent: it runs procoder-check.js, which the
  // check-engine plan ships. Declaring it now would spawn a missing script on
  // every Write/Edit. That plan re-adds this declaration with the script.
  assert.deepStrictEqual(
    events.sort(),
    ['SessionStart', 'SubagentStart', 'UserPromptSubmit']
  );
  for (const event of events) {
    for (const group of hooks.hooks[event]) {
      for (const h of group.hooks) {
        assert.match(h.command, /\$\{CLAUDE_PLUGIN_ROOT\}\/hooks\/procoder-[a-z-]+\.js/);
        assert.ok(h.timeout > 0 && h.timeout <= 5,
          `${event} hook timeout ${h.timeout} exceeds its 5s budget`);
        const script = /hooks\/(procoder-[a-z-]+\.js)/.exec(h.command)[1];
        assert.ok(fs.existsSync(path.join(root, 'hooks', script)),
          `${event} declares ${script}, which does not ship`);
      }
    }
  }
});

test('package.json declares zero runtime dependencies', () => {
  const pkg = readJson('package.json');
  assert.strictEqual(pkg.dependencies, undefined);
  assert.strictEqual(pkg.devDependencies, undefined);
});
