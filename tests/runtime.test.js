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

test('Codex host emits systemMessage with level and optional hookSpecificOutput', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const saved = process.env.CLAUDE_CONFIG_DIR;
  const savedHost = process.env.PROCODER_HOST;
  process.env.CLAUDE_CONFIG_DIR = dir;
  process.env.PROCODER_HOST = 'codex';
  delete require.cache[require.resolve('../hooks/procoder-runtime')];
  delete require.cache[require.resolve('../hooks/procoder-config')];
  try {
    const rt = require('../hooks/procoder-runtime');
    const chunks = [];
    const original = process.stdout.write;
    process.stdout.write = (c) => { chunks.push(String(c)); return true; };
    try {
      rt.writeHookOutput('SessionStart', 'paranoid', '');
      rt.writeHookOutput('SubagentStart', 'pragmatic', 'CONTEXT');
    } finally {
      process.stdout.write = original;
    }
    const parsed1 = JSON.parse(chunks[0]);
    assert.strictEqual(parsed1.systemMessage, 'PROCODER:PARANOID');
    assert.ok(!parsed1.hookSpecificOutput);
    const parsed2 = JSON.parse(chunks[1]);
    assert.strictEqual(parsed2.systemMessage, 'PROCODER:PRAGMATIC');
    assert.strictEqual(parsed2.hookSpecificOutput.hookEventName, 'SubagentStart');
    assert.strictEqual(parsed2.hookSpecificOutput.additionalContext, 'CONTEXT');
  } finally {
    if (saved === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = saved;
    if (savedHost === undefined) delete process.env.PROCODER_HOST;
    else process.env.PROCODER_HOST = savedHost;
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('Copilot host emits additionalContext only for SessionStart with context', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const saved = process.env.CLAUDE_CONFIG_DIR;
  const savedHost = process.env.PROCODER_HOST;
  process.env.CLAUDE_CONFIG_DIR = dir;
  process.env.PROCODER_HOST = 'copilot';
  delete require.cache[require.resolve('../hooks/procoder-runtime')];
  delete require.cache[require.resolve('../hooks/procoder-config')];
  try {
    const rt = require('../hooks/procoder-runtime');
    const chunks = [];
    const original = process.stdout.write;
    process.stdout.write = (c) => { chunks.push(String(c)); return true; };
    try {
      rt.writeHookOutput('SessionStart', 'strict', 'DATA');
      rt.writeHookOutput('SessionStart', 'strict', '');
      rt.writeHookOutput('SubagentStart', 'strict', 'DATA');
    } finally {
      process.stdout.write = original;
    }
    const parsed1 = JSON.parse(chunks[0]);
    assert.deepStrictEqual(parsed1, { additionalContext: 'DATA' });
    const parsed2 = JSON.parse(chunks[1]);
    assert.deepStrictEqual(parsed2, {});
    const parsed3 = JSON.parse(chunks[2]);
    assert.deepStrictEqual(parsed3, {});
  } finally {
    if (saved === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = saved;
    if (savedHost === undefined) delete process.env.PROCODER_HOST;
    else process.env.PROCODER_HOST = savedHost;
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('Qoder host emits hookSpecificOutput only when context is non-empty', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  const saved = process.env.CLAUDE_CONFIG_DIR;
  const savedHost = process.env.PROCODER_HOST;
  process.env.CLAUDE_CONFIG_DIR = dir;
  process.env.PROCODER_HOST = 'qoder';
  delete require.cache[require.resolve('../hooks/procoder-runtime')];
  delete require.cache[require.resolve('../hooks/procoder-config')];
  try {
    const rt = require('../hooks/procoder-runtime');
    const chunks = [];
    const original = process.stdout.write;
    process.stdout.write = (c) => { chunks.push(String(c)); return true; };
    try {
      rt.writeHookOutput('SessionStart', 'strict', 'CONTEXT');
      rt.writeHookOutput('PostToolUse', 'strict', '');
    } finally {
      process.stdout.write = original;
    }
    const parsed1 = JSON.parse(chunks[0]);
    assert.strictEqual(parsed1.hookSpecificOutput.hookEventName, 'SessionStart');
    assert.strictEqual(parsed1.hookSpecificOutput.additionalContext, 'CONTEXT');
    const parsed2 = JSON.parse(chunks[1]);
    assert.deepStrictEqual(parsed2, {});
  } finally {
    if (saved === undefined) delete process.env.CLAUDE_CONFIG_DIR;
    else process.env.CLAUDE_CONFIG_DIR = saved;
    if (savedHost === undefined) delete process.env.PROCODER_HOST;
    else process.env.PROCODER_HOST = savedHost;
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
