const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { BASELINE_VERSION } = require('../hooks/checks/baseline');

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
  const repo = repoWith({ 'a.ts': 'el.innerHTML = danger;\n' });  // procoder: literal safe/xss-sink the one-line fixture the hook is asked to report on
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
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  for (const level of ['pragmatic', 'strict', 'paranoid']) {
    const out = runHook(repo, path.join(repo, 'a.ts'), { PROCODER_DEFAULT_LEVEL: level });
    assert.strictEqual(out.decision, undefined);
    assert.strictEqual(out.permissionDecision, undefined);
    assert.ok(!out.hookSpecificOutput.permissionDecision);
  }
});

// Rungs 1-2 are enforced at every level; 3-4 are judgment, and at `pragmatic`
// the user asked to be told about them rather than told to fix them.
const MIXED_RUNGS = `eval(x);\nfunction big(a) {\n${'  const v = 1;\n'.repeat(45)}}\n`;  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it

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

test('a stale baseline is reported once, with the command that fixes it', () => {
  const repo = repoWith({
    'a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
    '.procoder-baseline.json': JSON.stringify({ version: 1, fingerprints: [] }),
  });
  assert.match(contextOf(runHook(repo, path.join(repo, 'a.ts'))), /procoder baseline/);
});

test('a current baseline draws no re-baseline notice', () => {
  const repo = repoWith({
    'a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
    '.procoder-baseline.json': JSON.stringify({ version: BASELINE_VERSION, fingerprints: [] }),
  });
  assert.ok(!/procoder baseline/.test(contextOf(runHook(repo, path.join(repo, 'a.ts')))));
});

test('PROCODER_NO_HOOK disables the hook', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
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

// --- timing ----------------------------------------------------------------
//
// The budget is 2s and a real run costs ~65ms, so asserting against 2s asserts
// almost nothing — except on a loaded machine, where the process spawn alone
// can breach it, and then it asserts a flake. One already fired this session.
//
// So each run is compared against a baseline measured here and now: the same
// hook, the same spawn, over a one-line file. Machine load moves baseline and
// measurement together; work that stops being linear moves only the
// measurement. Best of three on each side, because a scheduler stall hits one
// run of three, not all three.
//
// Catches: any regression that makes analysing a large file an order of
// magnitude dearer than the spawn it rides on — today ~34ms of analysis over a
// ~31ms baseline, and the bound allows six times the baseline. Does not catch:
// a uniform slowdown that costs the one-line run just as much (module load,
// config parsing), or a regression smaller than that multiple.
const SPAWN_MULTIPLE = 6;

function bestOf(runs, work) {
  let best = Infinity;
  for (let i = 0; i < runs; i += 1) {
    const started = Date.now();
    work();
    best = Math.min(best, Date.now() - started);
  }
  return best;
}

function assertNearBareHook(repo, file, what) {
  const bare = repoWith({ 'bare.ts': 'const x = 1;\n' });
  const baseline = bestOf(3, () => runHook(bare, path.join(bare, 'bare.ts')));
  const elapsed = bestOf(3, () => runHook(repo, file));
  assert.ok(elapsed <= baseline * SPAWN_MULTIPLE + 50,
    `${what}: ${elapsed}ms against a ${baseline}ms one-line-file baseline`);
}

test('the hook stays inside its budget on a large file', () => {
  const repo = repoWith({ 'big.ts': 'const x = 1;\n'.repeat(20000) });
  assertNearBareHook(repo, path.join(repo, 'big.ts'), '20k lines');
});

test('the hook stays inside its budget on a minified file', () => {
  let line = '';
  while (line.length < 200 * 1024) line += `function f${line.length}(a,b){return a&&b?a:b;}`;
  const repo = repoWith({ 'min.ts': line });
  assertNearBareHook(repo, path.join(repo, 'min.ts'), '200KB single line');
});

test('an Edit reports only the region it touched', () => {
  const repo = repoWith({
    'a.ts': `eval(old);\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = runHook(repo, path.join(repo, 'a.ts'), {}, {
    tool_name: 'Edit',
    tool_input: { old_string: 'nothing;', new_string: 'eval(fresh);' },  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  assert.match(contextOf(out), /a\.ts:42/);
  assert.ok(!/a\.ts:1\s/.test(contextOf(out)), 'reported an untouched line of the file');
});

test('a Write reports the whole file it wrote', () => {
  const repo = repoWith({
    'a.ts': `eval(old);\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.match(contextOf(out), /a\.ts:1\b/);
  assert.match(contextOf(out), /a\.ts:42/);
});

test('a secret outside the edited region is still reported', () => {
  const repo = repoWith({
    'a.ts': `const k = "AKIAIOSFODNN7EXAMPLE";\n${'const filler = 1;\n'.repeat(40)}eval(fresh);\n`,  // procoder: literal safe/hardcoded-secret, safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const out = runHook(repo, path.join(repo, 'a.ts'), {}, {
    tool_name: 'Edit',
    tool_input: { old_string: 'nothing;', new_string: 'eval(fresh);' },  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  assert.match(contextOf(out), /a\.ts:1\b/);
});
