const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-check.js');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-hook-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

function runHook(repo, filePath, env = {}, payload = {}) {
  const stdout = execFileSync('node', [HOOK], {
    encoding: 'utf8',
    cwd: repo,
    input: JSON.stringify({
      tool_name: 'Write',
      cwd: repo,
      ...payload,
      tool_input: { file_path: filePath, ...(payload.tool_input || {}) },
    }),
    env: { ...process.env, CLAUDE_CONFIG_DIR: repo, ...env },
  });
  return stdout.trim() ? JSON.parse(stdout) : {};
}

function contextOf(out) {
  return (out.hookSpecificOutput && out.hookSpecificOutput.additionalContext) || '';
}

test('emits findings as PostToolUse additionalContext', () => {
  const repo = repoWith({ 'a.ts': 'el.innerHTML = danger;\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.strictEqual(out.hookSpecificOutput.hookEventName, 'PostToolUse');
  assert.match(out.hookSpecificOutput.additionalContext, /SAFE/);
  assert.match(out.hookSpecificOutput.additionalContext, /a\.ts:1/);
});

test('emits nothing for a clean file', () => {
  const repo = repoWith({ 'a.ts': 'el.textContent = safe;\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.ok(!out.hookSpecificOutput || !out.hookSpecificOutput.additionalContext);
});

test('never blocks — no decision or permission field is ever emitted', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    const out = runHook(repo, path.join(repo, 'a.ts'), { PROCODER_DEFAULT_LEVEL: level });
    assert.strictEqual(out.decision, undefined);
    assert.strictEqual(out.permissionDecision, undefined);
    assert.ok(!out.hookSpecificOutput.permissionDecision);
  }
});

// Rungs 1-2 are enforced at every level; 3-4 are judgment, and at `pragmatic`
// the user asked to be told about them rather than told to fix them.
const MIXED_RUNGS = `eval(x);\nfunction big(a) {\n${'  const v = 1;\n'.repeat(45)}}\n`;

test('pragmatic presents SAFE as blocking and OBVIOUS as advisory', () => {
  const repo = repoWith({ 'a.ts': MIXED_RUNGS });
  const context = contextOf(runHook(repo, path.join(repo, 'a.ts'),
    { PROCODER_DEFAULT_LEVEL: 'pragmatic' }));
  const [blocking, advisory] = context.split(/^Flagged, not blocking:$/m);
  assert.ok(advisory !== undefined, `no advisory section:\n${context}`);
  assert.match(blocking, /Fix these before moving on:/);
  assert.match(blocking, /\[1 SAFE\]/);
  assert.ok(!/\[3 OBVIOUS\]/.test(blocking), 'OBVIOUS presented as must-fix at pragmatic');
  assert.match(advisory, /\[3 OBVIOUS\]/);
});

test('strict presents every rung as blocking', () => {
  const repo = repoWith({ 'a.ts': MIXED_RUNGS });
  const context = contextOf(runHook(repo, path.join(repo, 'a.ts'),
    { PROCODER_DEFAULT_LEVEL: 'strict' }));
  assert.ok(!/not blocking/.test(context), `strict emitted an advisory section:\n${context}`);
  assert.match(context, /Fix these before moving on:[\s\S]*\[1 SAFE\][\s\S]*\[3 OBVIOUS\]/);
});
