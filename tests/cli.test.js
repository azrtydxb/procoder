const test = require('node:test');
const assert = require('node:assert');
const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { MAX_FILE_BYTES } = require('../hooks/checks/run');

const CLI = path.join(__dirname, '..', 'bin', 'procoder.js');

// Every test builds its own throwaway repo under the OS temp dir; without
// cleanup a run leaves dozens of them behind. Tracked centrally and swept in
// one `after` hook rather than a try/finally per test, so a test that adds a
// new repoWith() call later can't forget the teardown.
const tempDirs = [];
test.after(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true });
});

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cli-'));
  tempDirs.push(dir);
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

// CLAUDE_CONFIG_DIR defaults to the throwaway repo, which holds no level file:
// the CLI then sees the default level (strict), not whatever the developer
// running the suite happens to have set for themselves.
function cli(repo, args, env = {}) {
  const r = spawnSync('node', [CLI, ...args], {
    cwd: repo, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo, ...env },
  });
  return { code: r.status, out: String(r.stdout || '') + String(r.stderr || '') };
}

function atLevel(repo, level) {
  fs.writeFileSync(path.join(repo, '.procoder-active'), level + '\n');
  return repo;
}

test('check exits non-zero and prints findings for a dirty file', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const result = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /SAFE/);
});

test('check exits 0 for a clean file', () => {
  const repo = repoWith({ 'a.ts': 'const x = 1;\n' });
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('baseline records findings, after which check passes', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.strictEqual(cli(repo, ['baseline', 'a.ts']).code, 0);
  assert.ok(fs.existsSync(path.join(repo, '.procoder-baseline.json')));
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('a NEW violation still fails after a baseline exists', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\n');  // procoder: literal safe/dynamic-eval, safe/xss-sink the rewritten fixture that adds one finding on top of the baseline
  const result = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /xss|innerHTML|SAFE/i);
});

test('verify passes when the baseline has not grown', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
});

test('verify fails when new violations appear on top of the baseline', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\ndebugger;\n');  // procoder: literal safe/dynamic-eval, safe/xss-sink the fixture whose two extra findings the ratchet must reject
  const result = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /not in the baseline/i);
});

// Fixing an old finding must not buy room for a new one: the totals are
// identical at step 3, so only fingerprint identity can catch it.
test('verify fails when a baselined finding is swapped for a different one', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);

  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\n');  // procoder: literal safe/dynamic-eval, safe/xss-sink step 2 of the swap fixture — the sink added beside the baselined eval
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 1);

  fs.writeFileSync(path.join(repo, 'a.ts'), 'el.innerHTML = y;\n');  // procoder: literal safe/xss-sink step 3 of the swap fixture — same total, different finding
  const swapped = cli(repo, ['verify', 'a.ts']);
  assert.strictEqual(swapped.code, 1);
  assert.match(swapped.out, /innerHTML|xss/i);
});

// Copy-paste is how legacy code grows: every identical line shares id, path and
// normalized content, so without an occurrence ordinal one baselined line
// accepts an unlimited number of clones.
test('verify fails when a baselined violation is cloned', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\n'.repeat(51));  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it

  const verified = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(verified.code, 0);
  assert.match(verified.out, /not in the baseline/i);

  const checked = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(checked.code, 0);
  assert.match(checked.out, /SAFE/);
});

test('verify passes when the baselined duplicate count is unchanged', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n'.repeat(3) });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('verify passes when a duplicate violation is deleted — shrinking is fine', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n'.repeat(3) });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\n'.repeat(2));  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
});

// A mistyped or renamed path used to exit 0, silently disabling the gate.
test('a path that does not exist is an error, not a clean run', () => {
  const repo = repoWith({ 'a.ts': 'const x = 1;\n' });
  const result = cli(repo, ['check', 'hokks']);
  assert.strictEqual(result.code, 2);
  assert.match(result.out, /hokks/);
});

test('a directory holding only excluded files is still a clean run', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["src/"]\n',
    'src/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  assert.strictEqual(cli(repo, ['check', 'src']).code, 0);
});

// A baseline from an older procoder suppresses nothing. Reporting the whole
// backlog as new is the adoption failure the ratchet exists to prevent, so the
// format change has to be said out loud.
const V1_BASELINE = JSON.stringify({ version: 1, fingerprints: ['deadbeef'] });

test('verify against a stale baseline explains the format change instead of failing on counts', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n', '.procoder-baseline.json': V1_BASELINE });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const result = cli(repo, ['verify', 'a.ts']);
  assert.strictEqual(result.code, 2);
  assert.match(result.out, /baseline.*format|re-run `procoder baseline/i);
  assert.doesNotMatch(result.out, /not in the baseline/i);
});

test('check against a stale baseline says to re-baseline', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n', '.procoder-baseline.json': V1_BASELINE });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.match(cli(repo, ['check', 'a.ts']).out, /fingerprint format changed/i);
});

test('re-baselining over a stale baseline replaces it with the current format', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n', '.procoder-baseline.json': V1_BASELINE });  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  assert.strictEqual(cli(repo, ['baseline', 'a.ts']).code, 0);
  const written = JSON.parse(fs.readFileSync(path.join(repo, '.procoder-baseline.json'), 'utf8'));
  assert.strictEqual(written.version, 2);
  assert.ok(!written.fingerprints.includes('deadbeef'), 'stale entries must not survive');
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
});

test('unknown subcommand prints usage and exits non-zero', () => {
  const repo = repoWith({});
  const result = cli(repo, ['frobnicate']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /usage/i);
});

// --unused-exclusions: a rule-scoped exclusion that stops suppressing anything
// (finding fixed, file changed) is itself an unnamed, stale suppression — a
// rung-4 violation in procoder's own config format.

test('a rule exclusion that suppresses a live finding is not reported as unused', () => {
  const repo = repoWith({
    'a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
    '.procoder.toml': '[exclude]\nrules = ["a.ts:safe/dynamic-eval"]\n',
  });
  const result = cli(repo, ['verify', '--unused-exclusions', 'a.ts']);
  assert.strictEqual(result.code, 0);
  assert.doesNotMatch(result.out, /suppressed nothing/i);
});

test('a rule exclusion that suppresses nothing is reported, and fails only with the flag', () => {
  const repo = repoWith({
    'a.ts': 'const x = 1;\n',
    '.procoder.toml': '[exclude]\nrules = ["a.ts:safe/dynamic-eval"]\n',
  });
  const plain = cli(repo, ['verify', 'a.ts']);
  assert.strictEqual(plain.code, 0, 'plain verify does not fail CI over a stale exclusion');
  assert.match(plain.out, /suppressed nothing/i);
  assert.match(plain.out, /a\.ts:safe\/dynamic-eval/);

  const flagged = cli(repo, ['verify', '--unused-exclusions', 'a.ts']);
  assert.notStrictEqual(flagged.code, 0, 'the dedicated flag opts into enforcement');
  assert.match(flagged.out, /suppressed nothing/i);
});

test('a rule exclusion naming a file outside the run scope is not reported either way', () => {
  const repo = repoWith({
    'a.ts': 'const x = 1;\n',
    'other.ts': 'const y = 1;\n',
    '.procoder.toml': '[exclude]\nrules = ["other.ts:safe/dynamic-eval"]\n',
  });
  const result = cli(repo, ['verify', '--unused-exclusions', 'a.ts']);
  assert.strictEqual(result.code, 0, 'an exclusion for a file the run never touched cannot be judged');
  assert.doesNotMatch(result.out, /suppressed nothing/i);
});

// The README promises OBVIOUS and ALONE are advisory at `pragmatic`. The CLI is
// what pre-commit hooks and CI run, so a blocking exit there contradicts it.
const ADVISORY_ONLY = '// TODO: fix this later\n';  // procoder: literal alone/orphan-todo scanner input for that rule, not an instance of it

test('pragmatic reports judgment findings but does not fail on them alone', () => {
  const repo = atLevel(repoWith({ 'a.ts': ADVISORY_ONLY }), 'pragmatic');
  const result = cli(repo, ['check', 'a.ts']);
  assert.strictEqual(result.code, 0);
  assert.match(result.out, /ALONE/);
  assert.match(result.out, /advisory|not blocking/i);
});

test('pragmatic still fails on a SAFE finding', () => {
  const repo = atLevel(repoWith({ 'a.ts': 'eval(x);\n' }), 'pragmatic');  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const result = cli(repo, ['check', 'a.ts']);
  assert.strictEqual(result.code, 1);
  assert.match(result.out, /SAFE/);
});

// CI has no user-level config, so an absent level file must mean strict.
test('no level file means strict, so judgment findings fail', () => {
  const repo = repoWith({ 'a.ts': ADVISORY_ONLY });
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 1);
});

test('strict fails on a judgment finding', () => {
  const repo = atLevel(repoWith({ 'a.ts': ADVISORY_ONLY }), 'strict');
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 1);
});

// A file over the size cap was never checked. Silence makes it identical to a
// clean pass, which is how an unchecked oversized file rides into main.
//
// The threshold comes from MAX_FILE_BYTES rather than a literal: this test
// previously hardcoded 300KB against a 256KB cap, and silently stopped testing
// anything when the cap was raised to 4MB — the file was scanned, not skipped.
//
// The file is made sparse with truncateSync instead of written out, because the
// cap is checked against statSync().size before the read: that is what genuinely
// trips it, and it costs no I/O.
test('check says a file was skipped for size instead of counting it clean', () => {
  const repo = repoWith({ 'big.ts': 'const x = 1;\n' });
  fs.truncateSync(path.join(repo, 'big.ts'), MAX_FILE_BYTES + 1);
  const result = cli(repo, ['check', 'big.ts']);
  assert.strictEqual(result.code, 0, 'a size skip reports, it does not fail the build');
  assert.match(result.out, /skipped/i);
  assert.match(result.out, /big\.ts/);
});

// An excluded path is deliberate config, not news.
test('check stays quiet about a deliberately excluded path', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["src/"]\n',
    'src/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const result = cli(repo, ['check', 'src']);
  assert.strictEqual(result.code, 0);
  assert.doesNotMatch(result.out, /skipped/i);
});

// The ratchet only holds if every subcommand agrees on which files exist. A
// file check skips must be invisible to baseline and verify too, or `verify`
// fails on findings `check` never printed.
const IGNORED_TREE = {
  'gen/.procoderignore': '*.ts\n',
  'gen/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  'src/b.ts': 'const x = 1;\n',
};

test('check, baseline and verify all honour a .procoderignore', () => {
  const repo = repoWith(IGNORED_TREE);
  assert.strictEqual(cli(repo, ['check', '.']).code, 0);
  assert.strictEqual(cli(repo, ['baseline', '.']).code, 0);
  const baseline = JSON.parse(fs.readFileSync(path.join(repo, '.procoder-baseline.json'), 'utf8'));
  assert.strictEqual(JSON.stringify(baseline).includes('gen/a.ts'), false);
  assert.strictEqual(cli(repo, ['verify', '.']).code, 0);
});

test('without the ignore file the same tree fails, so the test above proves something', () => {
  const repo = repoWith({ ...IGNORED_TREE, 'gen/.procoderignore': '# nothing\n' });
  assert.notStrictEqual(cli(repo, ['check', '.']).code, 0);
});

// Narrowing enforcement is never allowed to be silent. Reported once per
// ignore file rather than once per file: the case this feature exists for is a
// large generated subtree, and a line per file would bury the findings.
test('check reports how many files each .procoderignore skipped', () => {
  const repo = repoWith(IGNORED_TREE);
  const result = cli(repo, ['check', '.']);
  assert.match(result.out, /1 file skipped by gen\/\.procoderignore/);
});

test('check says nothing about ignore files when none matched', () => {
  const repo = repoWith({ 'src/b.ts': 'const x = 1;\n' });
  assert.doesNotMatch(cli(repo, ['check', '.']).out, /procoderignore/);
});

// `procoder statusline` — writes the statusLine block into the user's Claude
// Code settings so nobody has to hand-edit JSON and guess an install path.
//
// CLAUDE_CONFIG_DIR points at the throwaway repo in every one of these, which
// is what keeps the real ~/.claude/settings.json out of the suite: the CLI
// resolves the settings file through the same helper the rest of the project
// uses, so the redirect covers reads, writes, backups and the temp file alike.

const settingsIn = (repo) => path.join(repo, 'settings.json');
const readSettings = (repo) => JSON.parse(fs.readFileSync(settingsIn(repo), 'utf8'));

function withSettings(settings) {
  return repoWith({ 'settings.json': JSON.stringify(settings, null, 2) + '\n' });
}

test('statusline install writes a statusLine into a settings file that did not exist', () => {
  const repo = repoWith({});
  const result = cli(repo, ['statusline', 'install']);
  assert.strictEqual(result.code, 0, result.out);
  const written = readSettings(repo);
  assert.strictEqual(written.statusLine.type, 'command');
  assert.match(written.statusLine.command, /procoder-statusline\.(sh|ps1)/);
  assert.match(result.out, /settings\.json/);
});

// The whole risk of this command: it edits a file the user did not ask us to
// touch beyond one key. Anything else in there has to come out identical.
test('statusline install leaves every unrelated key untouched', () => {
  const others = {
    model: 'opus',
    env: { FOO: 'bar' },
    permissions: { allow: ['Bash(ls:*)'] },
    someKeyProcoderHasNeverHeardOf: [1, 2, 3],
  };
  const repo = withSettings(others);
  assert.strictEqual(cli(repo, ['statusline', 'install']).code, 0);
  const written = readSettings(repo);
  for (const [key, value] of Object.entries(others)) {
    assert.deepStrictEqual(written[key], value, `${key} was not preserved`);
  }
});

test('statusline install twice is a no-op the second time', () => {
  const repo = repoWith({});
  assert.strictEqual(cli(repo, ['statusline', 'install']).code, 0);
  const first = fs.readFileSync(settingsIn(repo), 'utf8');

  const again = cli(repo, ['statusline', 'install']);
  assert.strictEqual(again.code, 0, again.out);
  assert.match(again.out, /already/i);
  assert.strictEqual(fs.readFileSync(settingsIn(repo), 'utf8'), first);
  assert.strictEqual(fs.readdirSync(repo).filter((f) => f.includes('bak')).length, 0,
    'a no-op must not leave a backup behind either');
});

const FOREIGN = { type: 'command', command: 'echo my-own-statusline' };

test('statusline install refuses to clobber somebody else’s statusLine', () => {
  const repo = withSettings({ statusLine: FOREIGN });
  const result = cli(repo, ['statusline', 'install']);
  assert.strictEqual(result.code, 1);
  assert.match(result.out, /echo my-own-statusline/, 'it must say what is already there');
  assert.match(result.out, /procoder-statusline/, 'and what it would have set');
  assert.match(result.out, /--force/);
  assert.deepStrictEqual(readSettings(repo).statusLine, FOREIGN);
});

test('statusline install --force replaces a foreign statusLine', () => {
  const repo = withSettings({ statusLine: FOREIGN, model: 'opus' });
  const result = cli(repo, ['statusline', 'install', '--force']);
  assert.strictEqual(result.code, 0, result.out);
  assert.match(readSettings(repo).statusLine.command, /procoder-statusline/);
  assert.strictEqual(readSettings(repo).model, 'opus');
  const backups = fs.readdirSync(repo).filter((f) => f.startsWith('settings.json.'));
  assert.strictEqual(backups.length, 1, 'the overwritten settings must be recoverable');
  assert.deepStrictEqual(
    JSON.parse(fs.readFileSync(path.join(repo, backups[0]), 'utf8')).statusLine, FOREIGN);
  assert.match(result.out, new RegExp(backups[0].replace(/\./g, '\\.')),
    'the backup path has to be printed, or it is not a backup anyone can use');
});

// Overwriting a settings file we could not parse would destroy configuration
// the user cannot get back — a far worse outcome than no statusline.
test('statusline install reports invalid JSON and leaves the file alone', () => {
  const broken = '{ "model": "opus",\n';
  const repo = repoWith({ 'settings.json': broken });
  const result = cli(repo, ['statusline', 'install']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /JSON/i);
  assert.strictEqual(fs.readFileSync(settingsIn(repo), 'utf8'), broken);
});

test('statusline uninstall removes procoder’s entry and nothing else', () => {
  const repo = withSettings({ model: 'opus' });
  assert.strictEqual(cli(repo, ['statusline', 'install']).code, 0);

  const result = cli(repo, ['statusline', 'uninstall']);
  assert.strictEqual(result.code, 0, result.out);
  const written = readSettings(repo);
  assert.strictEqual(written.statusLine, undefined);
  assert.strictEqual(written.model, 'opus');
});

test('statusline uninstall leaves a statusLine that is not procoder’s', () => {
  const repo = withSettings({ statusLine: FOREIGN });
  const result = cli(repo, ['statusline', 'uninstall']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /echo my-own-statusline/);
  assert.deepStrictEqual(readSettings(repo).statusLine, FOREIGN);
});

test('statusline uninstall with nothing installed is a clean no-op', () => {
  const repo = repoWith({});
  const result = cli(repo, ['statusline', 'uninstall']);
  assert.strictEqual(result.code, 0, result.out);
});

test('statusline status reports what is configured', () => {
  const repo = repoWith({});
  assert.match(cli(repo, ['statusline', 'status']).out, /not installed|no statusLine/i);
  cli(repo, ['statusline', 'install']);
  assert.match(cli(repo, ['statusline', 'status']).out, /procoder-statusline/);
  assert.strictEqual(cli(repo, ['statusline', 'status']).code, 0);
});

// The failure mode this command exists to avoid is writing a plausible-looking
// command that does not actually run. So run the exact string it wrote.
test('the command statusline install writes really does print the badge', { skip: process.platform === 'win32' && 'POSIX shell only' }, () => {
  const repo = repoWith({});
  assert.strictEqual(cli(repo, ['statusline', 'install']).code, 0);
  fs.writeFileSync(path.join(repo, '.procoder-active'), 'strict\n');

  const { command } = readSettings(repo).statusLine;
  const r = spawnSync('bash', ['-c', command], {
    encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  });
  assert.strictEqual(r.status, 0, String(r.stderr));
  assert.strictEqual(String(r.stdout).trim(), '[PROCODER:STRICT]');
});

// A path holding shell metacharacters cannot be made safe by quoting alone, so
// the installer must decline and hand over the snippet instead of writing a
// command that would execute part of its own path.
test('an install path with shell metacharacters is refused, with the manual snippet', { skip: process.platform === 'win32' && 'POSIX paths only' }, () => {
  const root = path.join(__dirname, '..');
  const odd = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-odd-')), 'pro$(whoami)');
  tempDirs.push(path.dirname(odd));
  for (const dir of ['bin', 'hooks']) {
    fs.cpSync(path.join(root, dir), path.join(odd, dir), { recursive: true });
  }

  const repo = repoWith({});
  const r = spawnSync('node', [path.join(odd, 'bin', 'procoder.js'), 'statusline', 'install'], {
    cwd: repo, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  });
  assert.notStrictEqual(r.status, 0);
  const out = String(r.stdout) + String(r.stderr);
  assert.match(out, /statusLine/, 'the manual snippet is the fallback');
  assert.strictEqual(fs.existsSync(settingsIn(repo)), false, 'nothing may be written');
});
