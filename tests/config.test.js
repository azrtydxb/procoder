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
