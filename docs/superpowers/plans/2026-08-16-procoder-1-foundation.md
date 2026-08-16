# procoder Plan 1 — Foundation & Doctrine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working Claude Code plugin that injects the procoder doctrine at session start, persists an intensity level, shows it in the statusline, and generates every other platform's rule file from one canonical source.

**Architecture:** A `SessionStart` hook reads the active level and writes the doctrine into session context, exactly as ponytail does. The doctrine lives once in `skills/procoder/SKILL.md` with level-tagged blocks; `hooks/procoder-instructions.js` filters it by level at runtime, and `scripts/sync-rules.js` renders it into the nine other platform rule files at build time with a CI drift check.

**Tech Stack:** Node.js ≥18, CommonJS, zero runtime dependencies. Tests via `node --test`. Bash + PowerShell for statusline.

**Spec:** [docs/superpowers/specs/2026-08-16-procoder-design.md](../specs/2026-08-16-procoder-design.md)

## Global Constraints

- Node.js ≥18. CommonJS (`require`/`module.exports`) — matches the ponytail hooks these must sit alongside.
- **Zero runtime dependencies.** No package may be added to `dependencies`. Dev-time is also empty; tests use `node --test`.
- Levels are exactly `off | pragmatic | strict | paranoid`. Default `strict`.
- Level file: `$CLAUDE_CONFIG_DIR/.procoder-active`, falling back to `~/.claude/.procoder-active`.
- Env overrides: `PROCODER_DEFAULT_LEVEL` (level), `PROCODER_NO_HOOK=1` (disable all hooks).
- Every hook fails **silently**. A thrown exception in a hook must never block a session. Wrap all I/O in try/catch.
- Hook timeout budget: 5s for SessionStart, 2s for PostToolUse.
- `skills/procoder/SKILL.md` is the ONLY place doctrine text is authored. Every other rule file is generated. Editing a generated file by hand is a rung-4 violation and CI fails on it.
- Rung names are exactly `SAFE`, `TRUE`, `OBVIOUS`, `ALONE` in that order, everywhere.

---

## File Structure

| File | Responsibility |
|---|---|
| `.claude-plugin/plugin.json` | Claude Code plugin manifest; points at the hooks config |
| `.claude-plugin/marketplace.json` | Marketplace entry for `claude plugin install` |
| `hooks/claude-hooks.json` | Declares SessionStart / SubagentStart / UserPromptSubmit / PostToolUse |
| `hooks/procoder-config.js` | Level resolution, paths, validation. No side effects. |
| `hooks/procoder-runtime.js` | Level file read/write, host detection, hook stdout protocol |
| `hooks/procoder-instructions.js` | Loads `SKILL.md`, strips level-tagged blocks above the active level |
| `hooks/procoder-activate.js` | SessionStart entry point |
| `hooks/procoder-subagent.js` | SubagentStart entry point |
| `hooks/procoder-mode-tracker.js` | UserPromptSubmit: catches `/procoder <level>` and "stop procoder" |
| `hooks/procoder-statusline.sh` / `.ps1` | `[PROCODER:STRICT]` badge |
| `skills/procoder/SKILL.md` | THE doctrine — canonical source |
| `skills/procoder-help/SKILL.md` | Usage reference |
| `commands/procoder.toml`, `procoder-help.toml` | Slash commands for this plan's surface |
| `scripts/sync-rules.js` | Renders SKILL.md → all platform rule files |
| `tests/*.test.js` | `node --test` suites |
| `package.json` | Name, files, `npm test` |

---

## Task 1: Repo skeleton and manifests

**Files:**
- Create: `package.json`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `hooks/claude-hooks.json`, `.gitignore`
- Test: `tests/manifest.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces: `hooks/claude-hooks.json` declaring hook command paths that Tasks 4–7 implement; `plugin.json` field `hooks: "./hooks/claude-hooks.json"`.

- [ ] **Step 1: Write the failing test**

```js
// tests/manifest.test.js
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

test('every hook command references a file that will exist', () => {
  const hooks = readJson('hooks/claude-hooks.json');
  const events = Object.keys(hooks.hooks);
  assert.deepStrictEqual(
    events.sort(),
    ['PostToolUse', 'SessionStart', 'SubagentStart', 'UserPromptSubmit']
  );
  for (const event of events) {
    for (const group of hooks.hooks[event]) {
      for (const h of group.hooks) {
        assert.match(h.command, /\$\{CLAUDE_PLUGIN_ROOT\}\/hooks\/procoder-[a-z-]+\.js/);
        assert.ok(h.timeout > 0 && h.timeout <= 5);
      }
    }
  }
});

test('package.json declares zero runtime dependencies', () => {
  const pkg = readJson('package.json');
  assert.strictEqual(pkg.dependencies, undefined);
  assert.strictEqual(pkg.devDependencies, undefined);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/manifest.test.js`
Expected: FAIL — `ENOENT` opening `.claude-plugin/plugin.json`.

- [ ] **Step 3: Write the manifests**

```json
// .claude-plugin/plugin.json
{
  "name": "procoder",
  "version": "0.1.0",
  "description": "Ship gate for AI-written code. Four rungs — SAFE, TRUE, OBVIOUS, ALONE — enforced every response: security at trust boundaries, handled errors, readable code, and nothing stale left behind.",
  "author": { "name": "Pascal Watteel" },
  "hooks": "./hooks/claude-hooks.json"
}
```

```json
// hooks/claude-hooks.json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "node \"${CLAUDE_PLUGIN_ROOT}/hooks/procoder-activate.js\"",
            "timeout": 5,
            "statusMessage": "Loading procoder doctrine..."
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "node \"${CLAUDE_PLUGIN_ROOT}/hooks/procoder-subagent.js\"",
            "timeout": 5,
            "statusMessage": "Loading procoder doctrine..."
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "node \"${CLAUDE_PLUGIN_ROOT}/hooks/procoder-mode-tracker.js\"",
            "timeout": 5,
            "statusMessage": "Tracking procoder level..."
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "node \"${CLAUDE_PLUGIN_ROOT}/hooks/procoder-check.js\"",
            "timeout": 2,
            "statusMessage": "procoder check..."
          }
        ]
      }
    ]
  }
}
```

```json
// .claude-plugin/marketplace.json
{
  "name": "procoder",
  "owner": { "name": "Pascal Watteel" },
  "plugins": [
    {
      "name": "procoder",
      "source": "./",
      "description": "Four-rung ship gate: SAFE, TRUE, OBVIOUS, ALONE."
    }
  ]
}
```

```json
// package.json
{
  "name": "procoder",
  "version": "0.1.0",
  "description": "Ship gate for AI-written code: security, correctness, readability, and nothing stale left behind.",
  "license": "MIT",
  "main": "./hooks/procoder-runtime.js",
  "files": ["hooks/", "skills/", "commands/", "scripts/", "AGENTS.md", "LICENSE"],
  "scripts": {
    "test": "node --test tests/*.test.js",
    "sync": "node scripts/sync-rules.js",
    "sync:check": "node scripts/sync-rules.js --check"
  }
}
```

```
# .gitignore
node_modules/
.procoder-baseline.json
*.log
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/manifest.test.js`
Expected: PASS (3 tests). The PostToolUse command references `procoder-check.js`, delivered in Plan 2 — the test asserts the *pattern*, not the file's existence, deliberately.

- [ ] **Step 5: Commit**

```bash
git add package.json .gitignore .claude-plugin hooks/claude-hooks.json tests/manifest.test.js
git commit -m "feat: plugin manifests and hook declarations"
```

---

## Task 2: Level resolution (`procoder-config.js`)

**Files:**
- Create: `hooks/procoder-config.js`
- Test: `tests/config.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `LEVELS = ['off','pragmatic','strict','paranoid']`
  - `DEFAULT_LEVEL = 'strict'`
  - `normalizeLevel(value) → string|null`
  - `getDefaultLevel() → string`
  - `getClaudeDir() → string`
  - `getLevelFilePath() → string`
  - `isDeactivationCommand(text) → boolean`
  - `isShellSafe(p) → boolean`
  - `parseLevelCommand(text) → string|null`

- [ ] **Step 1: Write the failing test**

```js
// tests/config.test.js
const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const cfg = require('../hooks/procoder-config');

test('normalizeLevel accepts valid levels case-insensitively, rejects junk', () => {
  assert.strictEqual(cfg.normalizeLevel('STRICT'), 'strict');
  assert.strictEqual(cfg.normalizeLevel('  paranoid '), 'paranoid');
  assert.strictEqual(cfg.normalizeLevel('ultra'), null);
  assert.strictEqual(cfg.normalizeLevel(''), null);
  assert.strictEqual(cfg.normalizeLevel(undefined), null);
  assert.strictEqual(cfg.normalizeLevel(42), null);
});

test('getDefaultLevel prefers env var, falls back to strict', () => {
  const saved = process.env.PROCODER_DEFAULT_LEVEL;
  try {
    delete process.env.PROCODER_DEFAULT_LEVEL;
    assert.strictEqual(cfg.getDefaultLevel(), 'strict');
    process.env.PROCODER_DEFAULT_LEVEL = 'paranoid';
    assert.strictEqual(cfg.getDefaultLevel(), 'paranoid');
    process.env.PROCODER_DEFAULT_LEVEL = 'nonsense';
    assert.strictEqual(cfg.getDefaultLevel(), 'strict');
  } finally {
    if (saved === undefined) delete process.env.PROCODER_DEFAULT_LEVEL;
    else process.env.PROCODER_DEFAULT_LEVEL = saved;
  }
});

test('getClaudeDir honours CLAUDE_CONFIG_DIR', () => {
  const saved = process.env.CLAUDE_CONFIG_DIR;
  try {
    process.env.CLAUDE_CONFIG_DIR = '/tmp/fake-claude';
    assert.strictEqual(cfg.getClaudeDir(), '/tmp/fake-claude');
    assert.strictEqual(cfg.getLevelFilePath(), path.join('/tmp/fake-claude', '.procoder-active'));
  } finally {
    if (saved === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = saved;
  }
});

test('isDeactivationCommand matches only the standalone phrase', () => {
  assert.ok(cfg.isDeactivationCommand('stop procoder'));
  assert.ok(cfg.isDeactivationCommand('  Stop Procoder.  '));
  assert.ok(cfg.isDeactivationCommand('normal mode'));
  // must NOT fire mid-task on ordinary requests
  assert.ok(!cfg.isDeactivationCommand('add a normal mode toggle to settings'));
  assert.ok(!cfg.isDeactivationCommand('why did stop procoder not work'));
});

test('parseLevelCommand extracts the level from a slash command', () => {
  assert.strictEqual(cfg.parseLevelCommand('/procoder paranoid'), 'paranoid');
  assert.strictEqual(cfg.parseLevelCommand('/procoder'), null);
  assert.strictEqual(cfg.parseLevelCommand('/procoder bogus'), null);
  assert.strictEqual(cfg.parseLevelCommand('tell me about /procoder strict'), null);
});

test('isShellSafe allows ordinary paths, rejects metacharacters', () => {
  assert.ok(cfg.isShellSafe('/Users/pascal/.claude/plugins/procoder/hooks/x.sh'));
  assert.ok(cfg.isShellSafe('C:\\Users\\pascal\\procoder\\x.ps1'));
  assert.ok(!cfg.isShellSafe('/tmp/evil$(whoami)/x.sh'));
  assert.ok(!cfg.isShellSafe('/tmp/a;rm -rf b/x.sh'));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/config.test.js`
Expected: FAIL — `Cannot find module '../hooks/procoder-config'`.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — level resolution and shared path helpers.
// Pure: no file writes, no stdout. Safe to require from any hook.

const os = require('os');
const path = require('path');

const LEVELS = ['off', 'pragmatic', 'strict', 'paranoid'];
const DEFAULT_LEVEL = 'strict';

function normalizeLevel(value) {
  if (typeof value !== 'string') return null;
  const normalized = value.trim().toLowerCase();
  return LEVELS.includes(normalized) ? normalized : null;
}

function getDefaultLevel() {
  return normalizeLevel(process.env.PROCODER_DEFAULT_LEVEL) || DEFAULT_LEVEL;
}

function getClaudeDir() {
  return process.env.CLAUDE_CONFIG_DIR || path.join(os.homedir(), '.claude');
}

function getLevelFilePath() {
  return path.join(getClaudeDir(), '.procoder-active');
}

// Deactivation must be the WHOLE message. Matching the phrase anywhere turned
// procoder off mid-task on requests like "add a normal mode toggle".
function isDeactivationCommand(text) {
  const t = String(text || '').trim().toLowerCase().replace(/[.!?\s]+$/, '');
  return t === 'stop procoder' || t === 'normal mode';
}

// Only a message that STARTS with the command counts, so discussing the command
// in prose does not silently change the level.
function parseLevelCommand(text) {
  const m = /^\/procoder\s+(\S+)\s*$/i.exec(String(text || '').trim());
  return m ? normalizeLevel(m[1]) : null;
}

// Allowlist beats escaping every shell's metacharacters. A clone path with
// quotes, $, backtick or ; falls back to manual statusline setup instead.
function isShellSafe(p) {
  return typeof p === 'string' && /^[A-Za-z0-9 _.\-:/\\~]+$/.test(p);
}

module.exports = {
  LEVELS,
  DEFAULT_LEVEL,
  normalizeLevel,
  getDefaultLevel,
  getClaudeDir,
  getLevelFilePath,
  isDeactivationCommand,
  parseLevelCommand,
  isShellSafe,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/config.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-config.js tests/config.test.js
git commit -m "feat: level resolution and path helpers"
```

---

## Task 3: Runtime — level persistence and hook stdout protocol

**Files:**
- Create: `hooks/procoder-runtime.js`
- Test: `tests/runtime.test.js`

**Interfaces:**
- Consumes: `procoder-config` (`getLevelFilePath`, `normalizeLevel`, `getDefaultLevel`).
- Produces:
  - `readLevel() → string` (persisted level, else default)
  - `setLevel(level) → void`
  - `clearLevel() → void`
  - `writeHookOutput(event, level, context) → void`
  - `readHookInput() → Promise<object>` (parses stdin JSON, `{}` on any failure)
  - `isCodex`, `isCopilot`, `isQoder` booleans

- [ ] **Step 1: Write the failing test**

```js
// tests/runtime.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

function withTempClaudeDir(fn) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const saved = process.env.CLAUDE_CONFIG_DIR;
  process.env.CLAUDE_CONFIG_DIR = dir;
  delete require.cache[require.resolve('../hooks/procoder-runtime')];
  delete require.cache[require.resolve('../hooks/procoder-config')];
  try {
    return fn(require('../hooks/procoder-runtime'), dir);
  } finally {
    if (saved === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = saved;
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test('setLevel then readLevel round-trips', () => {
  withTempClaudeDir((rt, dir) => {
    rt.setLevel('paranoid');
    assert.strictEqual(fs.readFileSync(path.join(dir, '.procoder-active'), 'utf8').trim(), 'paranoid');
    assert.strictEqual(rt.readLevel(), 'paranoid');
  });
});

test('readLevel falls back to strict when no file exists', () => {
  withTempClaudeDir((rt) => assert.strictEqual(rt.readLevel(), 'strict'));
});

test('readLevel ignores a corrupted level file', () => {
  withTempClaudeDir((rt, dir) => {
    fs.writeFileSync(path.join(dir, '.procoder-active'), 'garbage\x00bytes');
    assert.strictEqual(rt.readLevel(), 'strict');
  });
});

test('clearLevel removes the file and does not throw when absent', () => {
  withTempClaudeDir((rt, dir) => {
    rt.setLevel('strict');
    rt.clearLevel();
    assert.ok(!fs.existsSync(path.join(dir, '.procoder-active')));
    assert.doesNotThrow(() => rt.clearLevel());
  });
});

test('setLevel never throws on an unwritable directory', () => {
  const saved = process.env.CLAUDE_CONFIG_DIR;
  process.env.CLAUDE_CONFIG_DIR = '/proc/nonexistent-procoder-dir';
  delete require.cache[require.resolve('../hooks/procoder-runtime')];
  delete require.cache[require.resolve('../hooks/procoder-config')];
  try {
    const rt = require('../hooks/procoder-runtime');
    assert.doesNotThrow(() => rt.setLevel('strict'));
  } finally {
    if (saved === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = saved;
  }
});

test('writeHookOutput emits raw text for SessionStart, JSON for SubagentStart', () => {
  withTempClaudeDir((rt) => {
    const chunks = [];
    const original = process.stdout.write;
    process.stdout.write = (c) => { chunks.push(String(c)); return true; };
    try {
      rt.writeHookOutput('SessionStart', 'strict', 'DOCTRINE');
      rt.writeHookOutput('SubagentStart', 'strict', 'DOCTRINE');
    } finally {
      process.stdout.write = original;
    }
    assert.strictEqual(chunks[0], 'DOCTRINE');
    const parsed = JSON.parse(chunks[1]);
    assert.strictEqual(parsed.hookSpecificOutput.hookEventName, 'SubagentStart');
    assert.strictEqual(parsed.hookSpecificOutput.additionalContext, 'DOCTRINE');
  });
});

test('writeHookOutput emits PostToolUse as additionalContext JSON', () => {
  withTempClaudeDir((rt) => {
    const chunks = [];
    const original = process.stdout.write;
    process.stdout.write = (c) => { chunks.push(String(c)); return true; };
    try {
      rt.writeHookOutput('PostToolUse', 'strict', 'FINDINGS');
    } finally {
      process.stdout.write = original;
    }
    const parsed = JSON.parse(chunks[0]);
    assert.strictEqual(parsed.hookSpecificOutput.hookEventName, 'PostToolUse');
    assert.strictEqual(parsed.hookSpecificOutput.additionalContext, 'FINDINGS');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/runtime.test.js`
Expected: FAIL — `Cannot find module '../hooks/procoder-runtime'`.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — level persistence and the host-specific hook stdout protocol.

const fs = require('fs');
const path = require('path');
const { getLevelFilePath, normalizeLevel, getDefaultLevel } = require('./procoder-config');

// Host detection mirrors ponytail: the same hook scripts are reused across
// agents that each read hook output differently.
const isCodex = !!process.env.CODEX_HOME || process.env.PROCODER_HOST === 'codex';
const isCopilot = process.env.PROCODER_HOST === 'copilot';
const isQoder = process.env.PROCODER_HOST === 'qoder';

function readLevel() {
  try {
    const raw = fs.readFileSync(getLevelFilePath(), 'utf8');
    return normalizeLevel(raw) || getDefaultLevel();
  } catch (e) {
    return getDefaultLevel();
  }
}

function setLevel(level) {
  const normalized = normalizeLevel(level);
  if (!normalized) return;
  try {
    const file = getLevelFilePath();
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, normalized + '\n');
  } catch (e) {
    // Best-effort: a read-only config dir must not break the session.
  }
}

function clearLevel() {
  try {
    fs.unlinkSync(getLevelFilePath());
  } catch (e) {
    // Already gone, or unwritable. Either way there is nothing to do.
  }
}

// Reads the hook payload Claude Code writes to stdin. Any malformed or absent
// input yields {} so callers can use plain property access without guards.
function readHookInput() {
  return new Promise((resolve) => {
    let data = '';
    const done = (value) => resolve(value);
    try {
      process.stdin.setEncoding('utf8');
      process.stdin.on('data', (chunk) => { data += chunk; });
      process.stdin.on('end', () => {
        try { done(JSON.parse(data) || {}); } catch (e) { done({}); }
      });
      process.stdin.on('error', () => done({}));
    } catch (e) {
      done({});
    }
  });
}

function writeHookOutput(event, level, context = '') {
  try {
    if (isCopilot) {
      process.stdout.write(JSON.stringify(
        event === 'SessionStart' && context ? { additionalContext: context } : {}));
      return;
    }
    if (isCodex) {
      const output = { systemMessage: `PROCODER:${String(level).toUpperCase()}` };
      if (context) {
        output.hookSpecificOutput = { hookEventName: event, additionalContext: context };
      }
      process.stdout.write(JSON.stringify(output));
      return;
    }
    if (isQoder) {
      const output = context
        ? { hookSpecificOutput: { hookEventName: event, additionalContext: context } }
        : {};
      process.stdout.write(JSON.stringify(output));
      return;
    }
    // Native Claude Code: SessionStart accepts raw stdout, but SubagentStart and
    // PostToolUse drop the context unless it is wrapped in hookSpecificOutput.
    if (event === 'SessionStart') {
      process.stdout.write(context);
      return;
    }
    process.stdout.write(JSON.stringify(
      { hookSpecificOutput: { hookEventName: event, additionalContext: context } }));
  } catch (e) {
    // EPIPE at hook exit must not surface as a hook failure.
  }
}

module.exports = {
  isCodex,
  isCopilot,
  isQoder,
  readLevel,
  setLevel,
  clearLevel,
  readHookInput,
  writeHookOutput,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/runtime.test.js`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-runtime.js tests/runtime.test.js
git commit -m "feat: level persistence and hook output protocol"
```

---

## Task 4: The doctrine (`skills/procoder/SKILL.md`)

**Files:**
- Create: `skills/procoder/SKILL.md`
- Test: `tests/doctrine.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces: the canonical doctrine text. Level-gated blocks are delimited by
  `<!-- level:paranoid -->` … `<!-- /level -->` and `<!-- level:strict -->` … `<!-- /level -->`.
  Task 5 strips blocks above the active level; Task 10 renders this file to other platforms.

- [ ] **Step 1: Write the failing test**

```js
// tests/doctrine.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const doctrine = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder', 'SKILL.md'), 'utf8');

test('has valid skill frontmatter with name and description', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(doctrine);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: procoder$/m);
  assert.match(m[1], /^description: .{40,1024}$/m);
});

test('the four rungs appear in order within the first 2000 chars', () => {
  const head = doctrine.slice(0, 2000);
  const order = ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE'].map((r) => head.indexOf(r));
  assert.ok(order.every((i) => i > -1), 'a rung is missing from the first screen');
  assert.deepStrictEqual(order, [...order].sort((a, b) => a - b), 'rungs out of order');
});

test('level-gated blocks are balanced and use known levels', () => {
  const opens = [...doctrine.matchAll(/<!-- level:([a-z]+) -->/g)];
  const closes = [...doctrine.matchAll(/<!-- \/level -->/g)];
  assert.strictEqual(opens.length, closes.length, 'unbalanced level markers');
  for (const o of opens) {
    assert.ok(['pragmatic', 'strict', 'paranoid'].includes(o[1]), `bad level: ${o[1]}`);
  }
});

test('covers every spec requirement area', () => {
  for (const topic of [
    'trust boundar', 'parameterized', 'authorization', 'secret',
    'PII', 'dependenc', 'error', 'test', 'naming', 'why',
    'removal trigger', 'ponytail',
  ]) {
    assert.match(doctrine.toLowerCase(), new RegExp(topic.toLowerCase()), `missing: ${topic}`);
  }
});

test('doctrine stays under the context budget', () => {
  assert.ok(doctrine.length < 12000, `doctrine is ${doctrine.length} chars; budget is 12000`);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/doctrine.test.js`
Expected: FAIL — `ENOENT` on `skills/procoder/SKILL.md`.

- [ ] **Step 3: Write the doctrine**

Author `skills/procoder/SKILL.md` from spec §2. Structure, in this order — the ladder table must land on the first screen because a long doctrine gets skimmed:

1. Frontmatter: `name: procoder`, and a `description:` listing the trigger phrases (`procoder`, `is this safe to ship`, `review for security`, `clean this up`, `dead code`, `deprecated`) plus "Use on ANY coding task".
2. **Persona** — the two-sentence inherit-and-get-paged framing from spec §2.1, verbatim.
3. **Persistence** — active every response; off only on "stop procoder" / "normal mode"; default `strict`; `/procoder pragmatic|strict|paranoid`.
4. **The ladder** — the four-row table from spec §2.2, with the "gate, not a search" sentence directly above it.
5. **Rung sections** — spec §2.3–2.6 in full. Wrap the paranoid-only material (threat-model note on new boundaries; rung 4 applied to the whole file) in `<!-- level:paranoid -->` … `<!-- /level -->`. Wrap the rung 3 shape-threshold table and rung 4 detail in `<!-- level:strict -->` … `<!-- /level -->` so `pragmatic` sheds them.
6. **Interop with ponytail** — spec §2.7, verbatim. This section is never level-gated.
7. **When NOT to apply** — generated code, vendored code, throwaway spikes explicitly labelled as such, and files excluded by `.procoder.toml`.
8. **Output** — the finding format from spec §5, with the four-line worked example.

Keep the whole file under 12000 characters; the test enforces it. Prose is
reference material below the table, not argument — every paragraph defending a
rule is a paragraph the model skims past.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/doctrine.test.js`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder/SKILL.md tests/doctrine.test.js
git commit -m "feat: the procoder doctrine"
```

---

## Task 5: Level filtering (`procoder-instructions.js`)

**Files:**
- Create: `hooks/procoder-instructions.js`
- Test: `tests/instructions.test.js`

**Interfaces:**
- Consumes: `skills/procoder/SKILL.md`.
- Produces: `getProcoderInstructions(level) → string` — doctrine with frontmatter stripped and level-gated blocks removed when above the requested level. Returns `''` for `off`.

- [ ] **Step 1: Write the failing test**

```js
// tests/instructions.test.js
const test = require('node:test');
const assert = require('node:assert');
const { getProcoderInstructions, RANK } = require('../hooks/procoder-instructions');

test('off yields no instructions', () => {
  assert.strictEqual(getProcoderInstructions('off'), '');
});

test('frontmatter is stripped from every level', () => {
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    const out = getProcoderInstructions(level);
    assert.ok(!out.startsWith('---'), `${level} leaked frontmatter`);
    assert.ok(out.includes('SAFE'), `${level} lost the ladder`);
  }
});

test('level markers never appear in output', () => {
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    assert.ok(!/<!-- \/?level/.test(getProcoderInstructions(level)),
      `${level} leaked a level marker`);
  }
});

test('higher levels are supersets of lower ones', () => {
  const pragmatic = getProcoderInstructions('pragmatic').length;
  const strict = getProcoderInstructions('strict').length;
  const paranoid = getProcoderInstructions('paranoid').length;
  assert.ok(pragmatic < strict, 'strict must add content over pragmatic');
  assert.ok(strict < paranoid, 'paranoid must add content over strict');
});

test('an unknown level is treated as the default', () => {
  assert.strictEqual(
    getProcoderInstructions('bogus'),
    getProcoderInstructions('strict'));
});

test('RANK orders the levels', () => {
  assert.ok(RANK.pragmatic < RANK.strict && RANK.strict < RANK.paranoid);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/instructions.test.js`
Expected: FAIL — `Cannot find module '../hooks/procoder-instructions'`.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — renders the doctrine for a given intensity level.
//
// The doctrine is authored once in skills/procoder/SKILL.md. Blocks that only
// apply above a given level are wrapped in <!-- level:NAME --> ... <!-- /level -->
// and stripped here when NAME outranks the active level.

const fs = require('fs');
const path = require('path');
const { normalizeLevel, DEFAULT_LEVEL } = require('./procoder-config');

const RANK = { pragmatic: 1, strict: 2, paranoid: 3 };
const DOCTRINE_PATH = path.join(__dirname, '..', 'skills', 'procoder', 'SKILL.md');

const BLOCK = /<!-- level:([a-z]+) -->\n?([\s\S]*?)<!-- \/level -->\n?/g;

function getProcoderInstructions(level) {
  const active = normalizeLevel(level) || DEFAULT_LEVEL;
  if (active === 'off') return '';

  let doctrine;
  try {
    doctrine = fs.readFileSync(DOCTRINE_PATH, 'utf8');
  } catch (e) {
    // A missing doctrine file must not break the session: no context is better
    // than a crashed hook.
    return '';
  }

  const body = doctrine.replace(/^---\n[\s\S]*?\n---\n/, '');
  const activeRank = RANK[active] || RANK[DEFAULT_LEVEL];

  return body
    .replace(BLOCK, (_match, blockLevel, content) =>
      (RANK[blockLevel] || 0) <= activeRank ? content : '')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
}

module.exports = { getProcoderInstructions, RANK };
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/instructions.test.js`
Expected: PASS (6 tests). If "higher levels are supersets" fails, the doctrine from Task 4 is missing `<!-- level:strict -->` or `<!-- level:paranoid -->` blocks — add them there, not here.

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-instructions.js tests/instructions.test.js
git commit -m "feat: level-filtered doctrine rendering"
```

---

## Task 6: SessionStart and SubagentStart hooks

**Files:**
- Create: `hooks/procoder-activate.js`, `hooks/procoder-subagent.js`
- Test: `tests/activate.test.js`

**Interfaces:**
- Consumes: `procoder-config`, `procoder-runtime`, `procoder-instructions`.
- Produces: executable hook entry points. No exports — these are `node <file>` targets.

- [ ] **Step 1: Write the failing test**

```js
// tests/activate.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-activate.js');
const SUBAGENT = path.join(__dirname, '..', 'hooks', 'procoder-subagent.js');

function run(script, env = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    const stdout = execFileSync('node', [script], {
      encoding: 'utf8',
      input: '{}',
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir, ...env },
    });
    return { stdout, dir, levelFile: path.join(dir, '.procoder-active') };
  } finally {
    // dir is inspected by the caller before this runs only for sync assertions,
    // so clean up lazily via the OS temp reaper instead of removing it here.
  }
}

test('activate emits the doctrine and persists the level', () => {
  const { stdout, levelFile } = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'strict' });
  assert.match(stdout, /SAFE/);
  assert.match(stdout, /ALONE/);
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'strict');
});

test('paranoid emits strictly more than pragmatic', () => {
  const lean = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'pragmatic' }).stdout;
  const full = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'paranoid' }).stdout;
  assert.ok(full.length > lean.length);
});

test('off emits nothing and writes no level file', () => {
  const { stdout, levelFile } = run(HOOK, { PROCODER_DEFAULT_LEVEL: 'off' });
  assert.ok(!/SAFE/.test(stdout));
  assert.ok(!fs.existsSync(levelFile));
});

test('PROCODER_NO_HOOK disables activation entirely', () => {
  const { stdout } = run(HOOK, { PROCODER_NO_HOOK: '1' });
  assert.ok(!/SAFE/.test(stdout));
});

test('subagent hook wraps context in hookSpecificOutput', () => {
  const { stdout } = run(SUBAGENT, { PROCODER_DEFAULT_LEVEL: 'strict' });
  const parsed = JSON.parse(stdout);
  assert.strictEqual(parsed.hookSpecificOutput.hookEventName, 'SubagentStart');
  assert.match(parsed.hookSpecificOutput.additionalContext, /SAFE/);
});

test('hooks exit 0 even when the config dir is unwritable', () => {
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: '{}',
    env: { ...process.env, CLAUDE_CONFIG_DIR: '/proc/nope-procoder' },
  }));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/activate.test.js`
Expected: FAIL — `Cannot find module .../hooks/procoder-activate.js`.

- [ ] **Step 3: Write the implementations**

```js
#!/usr/bin/env node
// procoder — SessionStart activation hook.
//   1. Resolves the active level (env > persisted > default)
//   2. Persists it so the statusline can read it
//   3. Emits the level-filtered doctrine as session context

const { getDefaultLevel, normalizeLevel } = require('./procoder-config');
const { getProcoderInstructions } = require('./procoder-instructions');
const { clearLevel, setLevel, readLevel, writeHookOutput } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

// An explicit env level wins over whatever the last session persisted.
const level = normalizeLevel(process.env.PROCODER_DEFAULT_LEVEL) || readLevel() || getDefaultLevel();

if (level === 'off') {
  clearLevel();
  writeHookOutput('SessionStart', 'off', '');
  process.exit(0);
}

setLevel(level);
writeHookOutput('SessionStart', level, getProcoderInstructions(level));
```

```js
#!/usr/bin/env node
// procoder — SubagentStart hook. A subagent inherits the doctrine; without this
// it writes code the main session would have gated.

const { getProcoderInstructions } = require('./procoder-instructions');
const { readLevel, writeHookOutput } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

const level = readLevel();
if (level === 'off') process.exit(0);

writeHookOutput('SubagentStart', level, getProcoderInstructions(level));
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/activate.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-activate.js hooks/procoder-subagent.js tests/activate.test.js
git commit -m "feat: SessionStart and SubagentStart doctrine injection"
```

---

## Task 7: Mode tracker (UserPromptSubmit)

**Files:**
- Create: `hooks/procoder-mode-tracker.js`
- Test: `tests/mode-tracker.test.js`

**Interfaces:**
- Consumes: `procoder-config` (`parseLevelCommand`, `isDeactivationCommand`), `procoder-runtime` (`readHookInput`, `setLevel`, `clearLevel`, `writeHookOutput`).
- Produces: hook entry point. Reads `{ prompt: string }` from stdin.

- [ ] **Step 1: Write the failing test**

```js
// tests/mode-tracker.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-mode-tracker.js');

function run(prompt) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const stdout = execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: JSON.stringify({ prompt }),
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
  });
  const levelFile = path.join(dir, '.procoder-active');
  return { stdout, level: fs.existsSync(levelFile) ? fs.readFileSync(levelFile, 'utf8').trim() : null };
}

test('/procoder paranoid switches the level', () => {
  assert.strictEqual(run('/procoder paranoid').level, 'paranoid');
});

test('/procoder with no argument leaves the level alone', () => {
  assert.strictEqual(run('/procoder').level, null);
});

test('"stop procoder" clears the level', () => {
  assert.strictEqual(run('stop procoder').level, null);
});

test('an ordinary prompt mentioning the phrase does not deactivate', () => {
  assert.strictEqual(run('add a normal mode toggle to the settings page').level, null);
});

test('malformed stdin does not crash the hook', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8',
    input: 'not json at all',
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
  }));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/mode-tracker.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — UserPromptSubmit hook. Catches level switches and deactivation
// without needing a round-trip through the model.

const { parseLevelCommand, isDeactivationCommand } = require('./procoder-config');
const { readHookInput, setLevel, clearLevel, writeHookOutput } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

readHookInput().then((input) => {
  const prompt = input.prompt || '';

  if (isDeactivationCommand(prompt)) {
    clearLevel();
    writeHookOutput('UserPromptSubmit', 'off', '');
    return;
  }

  const requested = parseLevelCommand(prompt);
  if (requested === 'off') {
    clearLevel();
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/mode-tracker.test.js`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-mode-tracker.js tests/mode-tracker.test.js
git commit -m "feat: level switching via prompt"
```

---

## Task 8: Statusline badge

**Files:**
- Create: `hooks/procoder-statusline.sh`, `hooks/procoder-statusline.ps1`
- Test: `tests/statusline.test.js`

**Interfaces:**
- Consumes: the level file written by Task 3.
- Produces: a single line on stdout, e.g. `[PROCODER:STRICT]`. Empty when no level file exists.

- [ ] **Step 1: Write the failing test**

```js
// tests/statusline.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const SCRIPT = path.join(__dirname, '..', 'hooks', 'procoder-statusline.sh');

function run(level) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  if (level) fs.writeFileSync(path.join(dir, '.procoder-active'), level + '\n');
  return execFileSync('bash', [SCRIPT], {
    encoding: 'utf8',
    env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
  }).trim();
}

test('renders the active level in caps', () => {
  assert.strictEqual(run('strict'), '[PROCODER:STRICT]');
  assert.strictEqual(run('paranoid'), '[PROCODER:PARANOID]');
});

test('renders nothing when no level file exists', () => {
  assert.strictEqual(run(null), '');
});

test('ignores a corrupted level file', () => {
  assert.strictEqual(run('garbage'), '');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/statusline.test.js`
Expected: FAIL — `ENOENT` on `procoder-statusline.sh`.

- [ ] **Step 3: Write the scripts**

```bash
#!/usr/bin/env bash
# procoder — statusline badge. Prints nothing when procoder is inactive.
set -uo pipefail

config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
level_file="$config_dir/.procoder-active"

[ -r "$level_file" ] || exit 0

level="$(tr -d '[:space:]' < "$level_file" | tr '[:upper:]' '[:lower:]')"

case "$level" in
  pragmatic|strict|paranoid) printf '[PROCODER:%s]\n' "$(printf '%s' "$level" | tr '[:lower:]' '[:upper:]')" ;;
  *) exit 0 ;;
esac
```

```powershell
# procoder — statusline badge (Windows).
$ErrorActionPreference = 'SilentlyContinue'

$configDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME '.claude' }
$levelFile = Join-Path $configDir '.procoder-active'

if (-not (Test-Path $levelFile)) { exit 0 }

$level = (Get-Content $levelFile -Raw).Trim().ToLower()
if ($level -in @('pragmatic', 'strict', 'paranoid')) {
    Write-Output ("[PROCODER:{0}]" -f $level.ToUpper())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `chmod +x hooks/procoder-statusline.sh && node --test tests/statusline.test.js`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-statusline.sh hooks/procoder-statusline.ps1 tests/statusline.test.js
git commit -m "feat: statusline badge"
```

---

## Task 9: `/procoder` and `/procoder-help` commands

**Files:**
- Create: `commands/procoder.toml`, `commands/procoder-help.toml`, `skills/procoder-help/SKILL.md`
- Test: `tests/commands.test.js`

**Interfaces:**
- Consumes: nothing at runtime; commands are declarative.
- Produces: the `/procoder` and `/procoder-help` slash commands. Later plans add one `.toml` per additional command using this exact shape.

- [ ] **Step 1: Write the failing test**

```js
// tests/commands.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const dir = path.join(__dirname, '..', 'commands');

test('every command file declares a description and a prompt', () => {
  const files = fs.readdirSync(dir).filter((f) => f.endsWith('.toml'));
  assert.ok(files.length >= 2);
  for (const file of files) {
    const raw = fs.readFileSync(path.join(dir, file), 'utf8');
    assert.match(raw, /^description\s*=\s*"/m, `${file} missing description`);
    assert.match(raw, /^prompt\s*=\s*"""/m, `${file} missing prompt`);
  }
});

test('command names match their filenames', () => {
  for (const file of fs.readdirSync(dir).filter((f) => f.endsWith('.toml'))) {
    const raw = fs.readFileSync(path.join(dir, file), 'utf8');
    const expected = path.basename(file, '.toml');
    assert.ok(raw.includes(expected), `${file} does not reference ${expected}`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/commands.test.js`
Expected: FAIL — `ENOENT` reading the `commands` directory.

- [ ] **Step 3: Write the commands**

```toml
# commands/procoder.toml
description = "Set the procoder intensity level: pragmatic, strict, or paranoid."
prompt = """
The user invoked /procoder with arguments: $ARGUMENTS

If the arguments name a level (pragmatic, strict, paranoid, or off), the
UserPromptSubmit hook has already persisted it — confirm the new level in one
line and state what changed:
- pragmatic: rungs SAFE and TRUE enforced; OBVIOUS and ALONE flagged only.
- strict: all four rungs enforced on code touched this session.
- paranoid: strict, plus a threat-model note on every new trust boundary, and
  ALONE applied to whole files rather than just the diff.

If no level was given, report the current level and list the options. One line
each. Do not restate the doctrine.
"""
```

```toml
# commands/procoder-help.toml
description = "Show procoder's rungs, levels, and commands."
prompt = """
Use the procoder-help skill to show: the four rungs and what each one gates,
the three levels, and the full command list. Keep it to one screen — a table
for the rungs, a table for the commands. No prose beyond one line per row.
"""
```

Write `skills/procoder-help/SKILL.md` with frontmatter (`name: procoder-help`,
a description covering "procoder help", "what does procoder check", "procoder
commands") and a body containing the two tables plus the level list. It is
reference material — no persona, no argument.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/commands.test.js`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add commands skills/procoder-help tests/commands.test.js
git commit -m "feat: /procoder and /procoder-help"
```

---

## Task 10: Multi-platform rule generation (`scripts/sync-rules.js`)

**Files:**
- Create: `scripts/sync-rules.js`, `.github/workflows/ci.yml`
- Generate: `AGENTS.md`, `.cursor/rules/procoder.mdc`, `.windsurf/rules/procoder.md`, `.clinerules/procoder.md`, `.kiro/steering/procoder.md`, `.qoder/rules/procoder.md`, `.agents/rules/procoder.md`, `.opencode/command/procoder.md`, `.openclaw/skills/procoder/SKILL.md`
- Test: `tests/sync-rules.test.js`

**Interfaces:**
- Consumes: `procoder-instructions` (`getProcoderInstructions`).
- Produces: `render() → Map<relativePath, content>`, and a CLI supporting `--check` (exit 1 on drift) and no-arg (write files).

- [ ] **Step 1: Write the failing test**

```js
// tests/sync-rules.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const { render, TARGETS } = require('../scripts/sync-rules');

const root = path.join(__dirname, '..');

test('renders a file for every declared target', () => {
  const out = render();
  assert.strictEqual(out.size, TARGETS.length);
  for (const target of TARGETS) {
    assert.ok(out.has(target.path), `missing ${target.path}`);
  }
});

test('every rendered file carries the generated-file warning and the ladder', () => {
  for (const [file, content] of render()) {
    assert.match(content, /DO NOT EDIT/, `${file} missing warning`);
    assert.match(content, /skills\/procoder\/SKILL\.md/, `${file} missing source pointer`);
    assert.match(content, /SAFE/, `${file} missing the ladder`);
  }
});

test('cursor target gets .mdc frontmatter, others do not', () => {
  const out = render();
  assert.match(out.get('.cursor/rules/procoder.mdc'), /^---\nalwaysApply: true\n/);
  assert.ok(!out.get('.clinerules/procoder.md').startsWith('---\nalwaysApply'));
});

test('--check exits 0 when files are in sync', () => {
  execFileSync('node', [path.join(root, 'scripts/sync-rules.js')], { cwd: root });
  assert.doesNotThrow(() => execFileSync(
    'node', [path.join(root, 'scripts/sync-rules.js'), '--check'], { cwd: root }));
});

test('--check exits non-zero after a generated file drifts', () => {
  const victim = path.join(root, '.clinerules', 'procoder.md');
  const saved = fs.readFileSync(victim, 'utf8');
  try {
    fs.writeFileSync(victim, saved + '\nhand-edited drift\n');
    assert.throws(() => execFileSync(
      'node', [path.join(root, 'scripts/sync-rules.js'), '--check'], { cwd: root, stdio: 'pipe' }));
  } finally {
    fs.writeFileSync(victim, saved);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/sync-rules.test.js`
Expected: FAIL — `Cannot find module '../scripts/sync-rules'`.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — renders the doctrine into every platform's rule file.
//
// skills/procoder/SKILL.md is the single source. Ponytail hand-maintains ten
// copies of its doctrine; procoder generates them, because a doctrine that
// forbids stale twins cannot ship with nine of them.
//
// Usage: node scripts/sync-rules.js [--check]

const fs = require('fs');
const path = require('path');
const { getProcoderInstructions } = require('../hooks/procoder-instructions');

const ROOT = path.join(__dirname, '..');
const SOURCE = 'skills/procoder/SKILL.md';

const WARNING = [
  '<!-- DO NOT EDIT. Generated from ' + SOURCE + ' by scripts/sync-rules.js.',
  '     Hand edits are overwritten and fail CI. Edit the source instead. -->',
].join('\n');

const TARGETS = [
  { path: 'AGENTS.md', level: 'strict' },
  { path: '.cursor/rules/procoder.mdc', level: 'strict', frontmatter: '---\nalwaysApply: true\ndescription: procoder — four-rung ship gate\n---\n' },
  { path: '.windsurf/rules/procoder.md', level: 'strict' },
  { path: '.clinerules/procoder.md', level: 'strict' },
  { path: '.kiro/steering/procoder.md', level: 'strict' },
  { path: '.qoder/rules/procoder.md', level: 'strict' },
  { path: '.agents/rules/procoder.md', level: 'strict' },
  { path: '.opencode/command/procoder.md', level: 'strict' },
  { path: '.openclaw/skills/procoder/SKILL.md', level: 'strict' },
];

function render() {
  const out = new Map();
  for (const target of TARGETS) {
    const body = getProcoderInstructions(target.level);
    out.set(target.path, (target.frontmatter || '') + WARNING + '\n\n' + body + '\n');
  }
  return out;
}

function main() {
  const check = process.argv.includes('--check');
  const drifted = [];

  for (const [rel, content] of render()) {
    const abs = path.join(ROOT, rel);
    let current = null;
    try { current = fs.readFileSync(abs, 'utf8'); } catch (e) { /* absent counts as drift */ }

    if (current === content) continue;
    if (check) { drifted.push(rel); continue; }

    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, content);
    process.stdout.write(`wrote ${rel}\n`);
  }

  if (check && drifted.length) {
    process.stderr.write(
      'procoder: generated rule files are out of sync with ' + SOURCE + ':\n' +
      drifted.map((f) => '  ' + f).join('\n') +
      '\nRun: npm run sync\n');
    process.exit(1);
  }
}

if (require.main === module) main();

module.exports = { render, TARGETS, WARNING };
```

```yaml
# .github/workflows/ci.yml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm test
      - name: Generated rule files must match the doctrine
        run: npm run sync:check
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm run sync && node --test tests/sync-rules.test.js`
Expected: PASS (5 tests), and nine generated files plus `AGENTS.md` on disk.

- [ ] **Step 5: Commit**

```bash
npm run sync
git add scripts/sync-rules.js tests/sync-rules.test.js .github/workflows/ci.yml \
        AGENTS.md .cursor .windsurf .clinerules .kiro .qoder .agents .opencode .openclaw
git commit -m "feat: generate platform rule files from one doctrine source"
```

---

## Task 11: End-to-end smoke test and README

**Files:**
- Create: `tests/e2e.test.js`, `README.md`
- Test: `tests/e2e.test.js`

**Interfaces:**
- Consumes: every module from Tasks 1–10.
- Produces: nothing further; this task closes Plan 1.

- [ ] **Step 1: Write the failing test**

```js
// tests/e2e.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.join(__dirname, '..');
const hook = (name) => path.join(root, 'hooks', name);

test('a full session lifecycle: activate, switch level, deactivate', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-e2e-'));
  const env = { ...process.env, CLAUDE_CONFIG_DIR: dir };
  const levelFile = path.join(dir, '.procoder-active');

  const start = execFileSync('node', [hook('procoder-activate.js')],
    { encoding: 'utf8', input: '{}', env });
  assert.match(start, /SAFE/);
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'strict');

  execFileSync('node', [hook('procoder-mode-tracker.js')],
    { encoding: 'utf8', input: JSON.stringify({ prompt: '/procoder paranoid' }), env });
  assert.strictEqual(fs.readFileSync(levelFile, 'utf8').trim(), 'paranoid');

  const badge = execFileSync('bash', [hook('procoder-statusline.sh')], { encoding: 'utf8', env });
  assert.strictEqual(badge.trim(), '[PROCODER:PARANOID]');

  execFileSync('node', [hook('procoder-mode-tracker.js')],
    { encoding: 'utf8', input: JSON.stringify({ prompt: 'stop procoder' }), env });
  assert.ok(!fs.existsSync(levelFile));

  fs.rmSync(dir, { recursive: true, force: true });
});

test('README documents every level and command that exists', () => {
  const readme = fs.readFileSync(path.join(root, 'README.md'), 'utf8');
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    assert.match(readme, new RegExp(level), `README missing level: ${level}`);
  }
  for (const file of fs.readdirSync(path.join(root, 'commands'))) {
    const cmd = '/' + path.basename(file, '.toml');
    assert.ok(readme.includes(cmd), `README missing command: ${cmd}`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/e2e.test.js`
Expected: FAIL on the README test — `ENOENT` on `README.md`.

- [ ] **Step 3: Write the README**

`README.md` covering, in this order:

1. One-paragraph pitch: the four rungs as a ship gate, and the ponytail relationship.
2. The ladder table (copied from the doctrine — the one place duplication is
   accepted, because a README that makes the reader open another file fails at
   its job). The sync-rules drift check does not cover README; a comment in the
   doctrine notes the copy.
3. Install: `claude plugin marketplace add <repo>` then `claude plugin install procoder`.
4. Levels table.
5. Command table — one row per `commands/*.toml`, kept in step by the test above.
6. Configuration: `PROCODER_DEFAULT_LEVEL`, `PROCODER_NO_HOOK`, `CLAUDE_CONFIG_DIR`.
7. Statusline setup snippet.
8. "Other platforms" — the generated rule files and which agent reads which.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test`
Expected: PASS — every suite from Tasks 1–11 green.

- [ ] **Step 5: Commit**

```bash
git add README.md tests/e2e.test.js
git commit -m "feat: end-to-end lifecycle test and README"
```

---

## Done when

- `npm test` passes and `npm run sync:check` exits 0.
- Installing the plugin locally and starting a session shows `[PROCODER:STRICT]`
  in the statusline and the doctrine in session context.
- `/procoder paranoid` changes the badge without a restart.
- Every rule file under `.cursor/`, `.windsurf/`, `.clinerules/`, `.kiro/`,
  `.qoder/`, `.agents/`, `.opencode/`, `.openclaw/` and `AGENTS.md` is generated,
  and hand-editing any of them fails CI.

**Next:** Plan 2 — the check engine (`.procoder.toml`, tooling resolver, universal
pattern pack, ratchet baseline, six language packs, and the PostToolUse hook).
