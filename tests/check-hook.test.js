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
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.strictEqual(out.decision, undefined);
  assert.strictEqual(out.permissionDecision, undefined);
  assert.ok(!out.hookSpecificOutput.permissionDecision);
});

test('PROCODER_NO_HOOK disables the hook', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'), { PROCODER_NO_HOOK: '1' });
  assert.deepStrictEqual(out, {});
});

test('malformed input exits cleanly', () => {
  const repo = repoWith({});
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8', cwd: repo, input: 'not json',
    env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  }));
});

test('the hook completes within its 2s budget on a large file', () => {
  const repo = repoWith({ 'big.ts': 'const x = 1;\n'.repeat(20000) });
  const started = Date.now();
  runHook(repo, path.join(repo, 'big.ts'));
  assert.ok(Date.now() - started < 2000, 'hook exceeded its budget');
});

test('the hook completes within its 2s budget on a minified file', () => {
  let line = '';
  while (line.length < 200 * 1024) line += `function f${line.length}(a,b){return a&&b?a:b;}`;
  const repo = repoWith({ 'min.ts': line });
  const started = Date.now();
  runHook(repo, path.join(repo, 'min.ts'));
  const elapsed = Date.now() - started;
  assert.ok(elapsed < 2000, `hook took ${elapsed}ms on a 200KB single-line file`);
});

test('an Edit reports only the region it touched', () => {
  const repo = repoWith({
    'a.ts': `eval(old);\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,
  });
  const out = runHook(repo, path.join(repo, 'a.ts'), {}, {
    tool_name: 'Edit',
    tool_input: { old_string: 'nothing;', new_string: 'eval(fresh);' },
  });
  assert.match(contextOf(out), /a\.ts:42/);
  assert.ok(!/a\.ts:1\s/.test(contextOf(out)), 'reported an untouched line of the file');
});

test('a Write reports the whole file it wrote', () => {
  const repo = repoWith({
    'a.ts': `eval(old);\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,
  });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.match(contextOf(out), /a\.ts:1\b/);
  assert.match(contextOf(out), /a\.ts:42/);
});

test('a secret outside the edited region is still reported', () => {
  const repo = repoWith({
    'a.ts': `const k = "AKIAIOSFODNN7EXAMPLE";\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,
  });
  const out = runHook(repo, path.join(repo, 'a.ts'), {}, {
    tool_name: 'Edit',
    tool_input: { old_string: 'nothing;', new_string: 'eval(fresh);' },
  });
  assert.match(contextOf(out), /a\.ts:1\b/);
});
