const test = require('node:test');
const assert = require('node:assert');
const { execFileSync, spawnSync } = require('child_process');
const { MAX_FILE_BYTES, BUDGET_MS } = require('../hooks/checks/run');
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

function runHookRaw(repo, filePath, env = {}, payload = {}) {
  const res = spawnSync('node', [HOOK], {
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
  return { stdout: res.stdout || '', stderr: res.stderr || '', status: res.status };
}

function runHook(repo, filePath, env = {}, payload = {}) {
  const { stdout } = runHookRaw(repo, filePath, env, payload);
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
// can breach it, and then it asserts a flake.
//
// So each run is compared against a baseline measured here and now: the same
// hook, the same spawn, over a one-line file. Work that stops being linear
// moves the measurement and not the baseline.
//
// Catches: any regression that makes analysing a large file an order of
// magnitude dearer than the spawn it rides on — today ~34ms of analysis over a
// ~31ms baseline, and the bound allows six times the baseline. Does not catch:
// a uniform slowdown that costs the one-line run just as much (module load,
// config parsing), or a regression smaller than that multiple.
const SPAWN_MULTIPLE = 6;

// CPU time, not wall-clock — the same correction tests/perf-guard.js made, for
// the same reason, and these three assertions were the ones it did not reach.
//
// A ratio of two wall-clock spawns was assumed to be load-proof because load
// "moves both sides together". It does not. The baseline spawn is almost
// entirely fixed startup — fork, node boot, module load — which under
// contention inflates by scheduler queueing, a delay that has nothing to do
// with how much work is queued behind it. The measured spawn is that same
// startup PLUS the analysis. Two draws from a heavy-tailed distribution, best
// of three each, and the ratio between them is not stable: measured under 20
// spinners on a 10-core box, three of six runs of this file failed.
//
// A child's CPU is not the parent's to measure — `process.cpuUsage()` reads
// RUSAGE_SELF, and a waited-for child lands in RUSAGE_CHILDREN — so the child
// measures itself. `cpuProbe` is a wrapper that installs an exit handler and
// then requires the hook, so the number covers the real hook process end to
// end, module load included, and is reported on a stream nothing else parses.
// Other processes steal wall time from the hook; they cannot spend its CPU.
const CPU_MARK = 'PROCODER_CPU_MS';
const cpuProbe = (() => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cpu-'));
  const file = path.join(dir, 'probe.js');
  // The hook exits via process.exit() on its top-level catch, so this has to be
  // an 'exit' listener rather than code after the require.
  fs.writeFileSync(file, 'process.on("exit", () => {\n'
    + '  const u = process.cpuUsage();\n'
    + `  require("fs").writeSync(2, "\\n${CPU_MARK} " + (u.user + u.system) / 1000 + "\\n");\n`
    + '});\n'
    + `require(${JSON.stringify(HOOK)});\n`);
  return file;
})();

function hookCpuMs(repo, filePath) {
  const res = spawnSync('node', [cpuProbe], {
    encoding: 'utf8',
    cwd: repo,
    input: JSON.stringify({ tool_name: 'Write', cwd: repo, tool_input: { file_path: filePath } }),
    env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  });
  const at = new RegExp(`${CPU_MARK} ([\\d.]+)`).exec(res.stderr || '');
  assert.ok(at, `the hook process did not report its CPU time:\n${res.stderr}`);
  return Number(at[1]);
}

// Best of N: a GC pause or a page fault is the hook's own CPU time and only
// best-of drops it.
function bestOf(runs, work) {
  let best = Infinity;
  for (let i = 0; i < runs; i += 1) best = Math.min(best, work());
  return best;
}

function assertNearBareHook(repo, file, what) {
  const bare = repoWith({ 'bare.ts': 'const x = 1;\n' });
  const baseline = bestOf(3, () => hookCpuMs(bare, path.join(bare, 'bare.ts')));
  const elapsed = bestOf(3, () => hookCpuMs(repo, file));
  assert.ok(elapsed <= baseline * SPAWN_MULTIPLE + 50,
    `${what}: ${elapsed}ms of CPU against a ${baseline}ms one-line-file baseline`);
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

// The largest input the engine now accepts, run through the real hook process
// the way the harness runs it.
//
// Two assertions, because they fail for different reasons. The first is
// relative: a file at the cap against a quarter of the cap, same machine, same
// spawn, so linear work lands near 4x and anything super-linear runs away from
// it. The second is the requirement itself, and BUDGET_MS is an engine constant
// rather than a machine one; it is worth asserting here because the margin is
// real (measured ~230ms against 2000ms), unlike the one-line runs where 65ms
// against 2000ms asserted nothing.
//
// Both in CPU milliseconds. The budget it defends is a wall-clock one, and that
// is exactly why: on an idle machine the two numbers are the same, and on a
// machine oversubscribed 3:1 the wall-clock figure is how long the scheduler
// made the hook wait, which is the runner's property and not the engine's. A
// bound that fires because someone else's build was running proves nothing and
// trains everyone to re-run CI. What the engine owes is that the work it does
// fits the budget; what a host owes is to run it.
test('the hook stays inside its budget on a file at the size cap', () => {
  const unit = 'export function f(a, b) { if (a) { return b; } return a; }\n';
  const fill = (bytes) => unit.repeat(Math.ceil(bytes / unit.length)).slice(0, bytes);
  const repo = repoWith({
    'quarter.ts': fill(MAX_FILE_BYTES / 4),
    'atcap.ts': fill(MAX_FILE_BYTES),
  });

  const quarter = bestOf(3, () => hookCpuMs(repo, path.join(repo, 'quarter.ts')));
  const atCap = bestOf(3, () => hookCpuMs(repo, path.join(repo, 'atcap.ts')));

  assert.ok(atCap < quarter * 6,
    `a file at the cap cost ${atCap}ms of CPU against ${quarter}ms for a quarter of it — `
    + 'the size-dependent work is no longer linear');
  assert.ok(atCap < BUDGET_MS,
    `the largest permitted input took ${atCap}ms of CPU of a ${BUDGET_MS}ms budget`);
});

// A skipped file must say why. The hook emits no context for it — there are no
// findings to report — so silence on stdout is correct and silence everywhere
// is not: "too large to check" and "clean" are opposite answers, and the user
// gets no way to tell them apart. stderr is the channel that reaches the
// transcript without becoming model context on every excluded file.
//
// RED: the hook returned on `skipped` without writing anything anywhere.
test('a skipped file says why on stderr rather than passing as clean', () => {
  const repo = repoWith({ 'huge.ts': 'const x = 1;\n'.repeat(MAX_FILE_BYTES / 6) });
  const { stdout, stderr } = runHookRaw(repo, path.join(repo, 'huge.ts'));
  assert.strictEqual(stdout.trim(), '', 'a skipped file must not emit findings');
  assert.match(stderr, /huge\.ts/);
  assert.match(stderr, /too-large/);
});

test('an excluded file also says why, naming the reason', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["generated/"]\n',
    'generated/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const { stderr } = runHookRaw(repo, path.join(repo, 'generated/a.ts'));
  assert.match(stderr, /excluded/);
});

test('a clean file stays silent on both channels', () => {
  const repo = repoWith({ 'a.ts': 'export const x = 1;\n' });
  const { stdout, stderr } = runHookRaw(repo, path.join(repo, 'a.ts'));
  assert.strictEqual(stdout.trim(), '');
  assert.strictEqual(stderr.trim(), '', 'a clean file must not be announced as skipped');
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
