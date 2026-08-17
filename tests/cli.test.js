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

// A real git repository, not the hand-made .git directory above.
// `.procoderignore` staleness is judged over TRACKED files only, and
// `git ls-files` is the only thing that knows which those are — so these
// fixtures have to be repositories git will actually answer for. Files are
// staged, and committed only when a test needs a ref for `--since`; the
// identity is passed per-command so a CI runner with no configured user can
// still build one. `untracked` is written after the staging, which is how a
// fixture gets a file the repository does not own.
function gitRepoWith(files, untracked = {}, commit = false) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-git-'));
  tempDirs.push(dir);
  const write = (rel, content) => {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  };
  for (const [rel, content] of Object.entries(files)) write(rel, content);
  const git = (...args) => spawnSync('git', args, { cwd: dir, stdio: 'ignore' });
  git('init', '-q');
  git('add', '-A');
  if (commit) {
    git('-c', 'user.email=t@example.com', '-c', 'user.name=t',
      'commit', '-q', '-m', 'fixture');
  }
  for (const [rel, content] of Object.entries(untracked)) write(rel, content);
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

// stdout on its own. `cli()` folds stderr in, which is right for reading
// messages and wrong for parsing a document: every skip notice is on stderr
// precisely so it cannot land in the middle of one.
function stdoutOf(repo, args) {
  return String(spawnSync('node', [CLI, ...args], {
    cwd: repo, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  }).stdout);
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

// Running `prettier --write .` over a freshly baselined legacy repo used to
// evaporate the baseline: every re-wrapped statement came back as a new
// finding and CI went red on code the team never touched.
const FLAT_EVAL = 'const r = eval(userInput + suffix);\n';  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
const WRAPPED_EVAL = 'const r = eval(\n  userInput + suffix,\n);\n';  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it

test('verify passes after a formatter re-wraps a baselined statement', () => {
  const repo = repoWith({ 'a.ts': FLAT_EVAL });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), WRAPPED_EVAL);
  const result = cli(repo, ['verify', 'a.ts']);
  assert.strictEqual(result.code, 0, result.out);
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

// Wrapping must not become a way to launder clones past the ratchet.
test('verify fails when a re-wrapped baselined violation is cloned fifty times', () => {
  const repo = repoWith({ 'a.ts': FLAT_EVAL });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), WRAPPED_EVAL.repeat(51));
  const result = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /not in the baseline/i);
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
  assert.strictEqual(written.version, 4);
  assert.ok(!written.entries.some((e) => e.fp === 'deadbeef'), 'stale entries must not survive');
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

// `[levels]` pins a level to the paths that earn it. Both directions are the
// point: a scripts/ directory stops failing a strict session on judgment
// findings, and an auth/ directory answers to paranoid whatever the session is.
// The pin decides whether a finding blocks, so it decides the exit code — which
// is the only part a pre-commit hook or CI ever sees.
test('a [levels] pin loosens the gate for the paths it names', () => {
  const repo = atLevel(repoWith({
    'scripts/a.ts': ADVISORY_ONLY,
    '.procoder.toml': '[levels]\npragmatic = ["scripts/"]\n',
  }), 'strict');
  const result = cli(repo, ['check', 'scripts/a.ts']);
  assert.strictEqual(result.code, 0, 'a pinned-pragmatic path must not block on a judgment finding');
  assert.match(result.out, /ALONE/, 'it is still reported');
  assert.match(result.out, /\[levels\] pin/, 'and the run says the level came from a pin');
});

test('a [levels] pin leaves paths it does not name at the session level', () => {
  const repo = atLevel(repoWith({
    'src/a.ts': ADVISORY_ONLY,
    '.procoder.toml': '[levels]\npragmatic = ["scripts/"]\n',
  }), 'strict');
  assert.strictEqual(cli(repo, ['check', 'src/a.ts']).code, 1);
});

// "off" would silence a path outright, which [exclude] paths already does and
// reports as a skip. Accepted here it would be a second, quieter kill switch.
test('a [levels] pin of "off" is refused, and the path stays gated', () => {
  const repo = atLevel(repoWith({
    'scripts/a.ts': ADVISORY_ONLY,
    '.procoder.toml': '[levels]\noff = ["scripts/"]\n',
  }), 'strict');
  const result = cli(repo, ['check', 'scripts/a.ts']);
  assert.strictEqual(result.code, 1);
  assert.match(result.out, /ignoring \[levels\] "off"/);
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

// `verify` is the CI gate, and it is the one place a silently skipped file
// costs the most: the ratchet compares present findings against the baseline,
// so a file nothing looked at contributes nothing and the build goes green.
// `check` said so all along; verify and baseline did not.
// A size skip used to report and still exit 0. It does not any more: the
// ratchet is a claim about what was looked at, and `max_file_bytes` set too low
// skips every file while verify prints "ratchet holds" — a CI gate passing
// because it read nothing. 2, not 1: "cannot verify", not "you added findings".
test('verify and baseline say a file was skipped for size, and verify stops', () => {
  const repo = repoWith({ 'big.ts': 'const x = 1;\n' });
  fs.truncateSync(path.join(repo, 'big.ts'), MAX_FILE_BYTES + 1);
  const baselined = cli(repo, ['baseline', 'big.ts']);
  assert.match(baselined.out, /skipped/i, 'baseline recorded nothing and said nothing');
  const verified = cli(repo, ['verify', 'big.ts']);
  assert.strictEqual(verified.code, 2, 'the ratchet held over a file nothing looked at');
  assert.doesNotMatch(verified.out, /ratchet holds/);
  assert.match(verified.out, /skipped/i);
  assert.match(verified.out, /big\.ts/);
});

// Deliberate config, and still news: a .procoderignore has always reported its
// count, and a path exclusion narrows the gate exactly as much. What it must
// not do is report once per file — that is what would bury the findings — so
// the count is one line naming the pattern that did it.
test('verify reports an excluded path by count, not by silence', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["src/"]\n',
    'src/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const result = cli(repo, ['verify', 'src']);
  assert.strictEqual(result.code, 0);
  assert.match(result.out, /1 file skipped by \[exclude\] paths "src\/"/);
});

test('check reports an excluded path by count, and still passes', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["src/"]\n',
    'src/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const result = cli(repo, ['check', 'src']);
  assert.strictEqual(result.code, 0);
  assert.match(result.out, /1 file skipped by \[exclude\] paths "src\/"/);
  assert.doesNotMatch(result.out, /SAFE/);
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

// The escape hatch. An ignore file is the sanctioned way to narrow the gate,
// which makes "why is this file not being checked?" a question the tool has to
// be able to answer directly — and it is what lets a test assert that a file a
// .procoderignore covers still trips the rung it is there to demonstrate.
test('--no-ignore checks a file every .procoderignore covers', () => {
  const repo = repoWith(IGNORED_TREE);
  assert.strictEqual(cli(repo, ['check', '.']).code, 0);
  const result = cli(repo, ['check', '--no-ignore', '.']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /gen\/a\.ts/);
});

// --no-ignore reaches the ignore files only. A [exclude] paths entry is the
// project-wide contract and stays in force, or the flag would become a way to
// scan vendor/ and node_modules by accident.
test('--no-ignore does not re-include a [exclude] paths exclusion', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["gen/"]\n',
    'gen/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  assert.strictEqual(cli(repo, ['check', '--no-ignore', '.']).code, 0);
});

// --- [exclude] paths, said out loud ----------------------------------------
//
// A path exclusion narrowed the gate exactly as much as a .procoderignore did
// and said nothing at all about it, so a project could lose whole directories
// of coverage and see the same output as a clean run.
const EXCLUDED_TREE = {
  '.procoder.toml': '[exclude]\npaths = ["gen/"]\n',
  'gen/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  'gen/b.ts': 'eval(y);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  'src/c.ts': 'const x = 1;\n',
};

test('check reports how many files [exclude] paths skipped, and which pattern did it', () => {
  const result = cli(repoWith(EXCLUDED_TREE), ['check', '.']);
  assert.strictEqual(result.code, 0);
  assert.match(result.out, /2 files skipped by \[exclude\] paths "gen\/"/);
});

test('verify reports the same count, so the ratchet says what it did not look at', () => {
  assert.match(cli(repoWith(EXCLUDED_TREE), ['verify', '.']).out, /skipped by \[exclude\] paths/);
});

// The sharpest form: the user asks for one named file and gets silence, which
// reads exactly like a pass.
test('a file named on the command line is never silently excluded', () => {
  const result = cli(repoWith(EXCLUDED_TREE), ['check', 'gen/a.ts']);
  assert.match(result.out, /gen\/a\.ts/);
  assert.match(result.out, /\[exclude\] paths "gen\/"/);
  assert.match(result.out, /\.procoder\.toml/);
  assert.match(result.out, /not checked/);
});

test('a path exclusion pointing at a directory that no longer exists is reported', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["gone/"]\n',
    'src/c.ts': 'const x = 1;\n',
  });
  const plain = cli(repo, ['verify', '.']);
  assert.strictEqual(plain.code, 0, 'plain verify does not fail CI over a stale exclusion');
  assert.match(plain.out, /gone\//);
  assert.match(plain.out, /excludes nothing|suppressed nothing|no longer exists/i);

  const flagged = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.notStrictEqual(flagged.code, 0, 'the dedicated flag opts into enforcement');
});

test('a path exclusion that still covers a real directory is not reported', () => {
  const result = cli(repoWith(EXCLUDED_TREE), ['verify', '--unused-exclusions', '.']);
  assert.strictEqual(result.code, 0);
  assert.doesNotMatch(result.out, /excludes nothing/i);
});

// The two ways a path exclusion rots without its path going anywhere. Both were
// silent until now, which is the shape this project exists to argue against: an
// exclusion nobody judges narrows the gate on the day it stops being needed and
// says nothing on the day it starts mattering again.

test('a path exclusion whose target has gone clean is reported', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["gen/"]\n',
    'gen/a.ts': 'const x = 1;\n',
    'src/c.ts': 'const y = 1;\n',
  });
  const plain = cli(repo, ['verify', '.']);
  assert.strictEqual(plain.code, 0, 'plain verify does not fail CI over a stale exclusion');
  assert.match(plain.out, /excludes nothing/i);
  assert.match(plain.out, /gen\/ — nothing it excludes has a finding/);

  const flagged = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.notStrictEqual(flagged.code, 0, 'the dedicated flag opts into enforcement');
});

test('a glob path exclusion matching nothing is reported', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["**/*.gen.ts"]\n',
    'src/c.ts': 'const x = 1;\n',
  });
  const plain = cli(repo, ['verify', '.']);
  assert.strictEqual(plain.code, 0);
  assert.match(plain.out, /\*\*\/\*\.gen\.ts — it matches no file in the tree/);

  const flagged = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.notStrictEqual(flagged.code, 0, 'the dedicated flag opts into enforcement');
});

test('a glob path exclusion still suppressing a finding is not reported', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["**/*.gen.ts"]\n',
    'src/c.gen.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const result = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.strictEqual(result.code, 0);
  assert.doesNotMatch(result.out, /excludes nothing/i);
});

// The same restraint an out-of-run rule exclusion gets. "This glob matches
// nothing" and "everything under it is clean" are claims about the tree; a run
// over one file has not seen the tree and must not make either — and, since the
// scan is the expensive half, must not pay for them.
test('the tree-wide path exclusion rules stay quiet on a partial run', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["gen/", "**/*.gen.ts"]\n',
    'gen/a.ts': 'const x = 1;\n',
    'src/c.ts': 'const y = 1;\n',
  });
  const result = cli(repo, ['verify', '--unused-exclusions', 'src/c.ts']);
  assert.strictEqual(result.code, 0, 'a run that never saw the tree cannot judge a path exclusion');
  assert.doesNotMatch(result.out, /excludes nothing/i);
});

// --- .procoderignore, judged for staleness ---------------------------------
//
// The third instrument that narrows enforcement, and the last one nothing
// judged. Same reporting and the same exit contract as the other two: said out
// loud under plain `verify`, failing only under --unused-exclusions.
const CLEAN_IGNORED_TREE = {
  'gen/.procoderignore': '# generated\n*.ts\n',
  'gen/a.ts': 'const x = 1;\n',
  'src/c.ts': 'const y = 1;\n',
};

test('a .procoderignore covering a tree that has gone clean is reported', () => {
  const repo = gitRepoWith(CLEAN_IGNORED_TREE);
  const plain = cli(repo, ['verify', '.']);
  assert.strictEqual(plain.code, 0, 'plain verify does not fail CI over a stale ignore pattern');
  assert.match(plain.out, /ignores nothing/i);
  assert.match(plain.out, /gen\/\.procoderignore:2 "\*\.ts" — nothing it ignores has a finding/,
    'an ignore file can sit anywhere, so the report names which file and which pattern');

  const flagged = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.notStrictEqual(flagged.code, 0, 'the dedicated flag opts into enforcement');
  assert.match(flagged.out, /ignores nothing/i);
});

test('a .procoderignore still suppressing a finding is not reported', () => {
  const repo = gitRepoWith({
    ...CLEAN_IGNORED_TREE,
    'gen/a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const result = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.strictEqual(result.code, 0, 'an ignore file doing real work must never be called stale');
  assert.doesNotMatch(result.out, /ignores nothing/i);
});

test('an ignore pattern matching nothing at all is reported', () => {
  const repo = gitRepoWith({ '.procoderignore': '*.gen.ts\n', 'src/c.ts': 'const y = 1;\n' });
  const plain = cli(repo, ['verify', '.']);
  assert.strictEqual(plain.code, 0);
  assert.match(plain.out, /\.procoderignore:1 "\*\.gen\.ts" — it matches no file in the tree/);
  assert.notStrictEqual(cli(repo, ['verify', '--unused-exclusions', '.']).code, 0);
});

// procoder's own root ignore file covers `.claude/` and `.superpowers/`:
// untracked agent scratch, present on a developer's machine and absent in CI.
// A rule that judged untracked content would call both stale forever, on two
// lines nobody may delete — a permanent false report, which is worse than a
// missed one because it teaches people to delete the fences.
test('an ignore pattern covering only untracked files is never reported', () => {
  const repo = gitRepoWith(
    { '.procoderignore': 'scratch/\n', 'src/c.ts': 'const y = 1;\n' },
    { 'scratch/note.ts': 'const z = 1;\n' });
  const result = cli(repo, ['verify', '--unused-exclusions', '.']);
  assert.strictEqual(result.code, 0);
  assert.doesNotMatch(result.out, /ignores nothing/i);
});

// The same restraint the tree-wide path-exclusion rules get: a run over one
// file has not seen the tree, cannot judge what an ignore file elsewhere is
// holding back, and must not pay for the scan that would answer it.
test('a partial-scope run judges no ignore file at all', () => {
  const repo = gitRepoWith(CLEAN_IGNORED_TREE);
  const result = cli(repo, ['verify', '--unused-exclusions', 'src/c.ts']);
  assert.strictEqual(result.code, 0, 'a run that never saw the tree cannot judge an ignore file');
  assert.doesNotMatch(result.out, /ignores nothing/i);
});

// `--since` is the same claim in a different shape: the run covered the files
// that changed, not the tree, so it has seen nothing that could condemn an
// ignore file somewhere else.
test('a --since run judges no ignore file either', () => {
  const repo = gitRepoWith(CLEAN_IGNORED_TREE, {}, true);
  fs.writeFileSync(path.join(repo, 'src/c.ts'), 'const y = 2;\n');
  const result = cli(repo, ['verify', '--unused-exclusions', '--since', 'HEAD']);
  assert.strictEqual(result.code, 0, 'a changed-files run has not seen the tree');
  assert.doesNotMatch(result.out, /ignores nothing/i);
});

// A CI gate that passes because it looked at nothing is the worst version of
// this defect: `max_file_bytes` set too low skipped every file in the repo and
// verify still printed "ratchet holds" and exited 0.
test('verify does not claim the ratchet holds over files it never read', () => {
  const repo = repoWith({
    '.procoder.toml': '[limits]\nmax_file_bytes = 4\n',
    'a.ts': 'eval(x);\n',  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  });
  const result = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(result.code, 0, 'a verify that read nothing must not pass');
  assert.doesNotMatch(result.out, /ratchet holds/);
  assert.match(result.out, /not checked|could not be checked/i);
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

// A plugin install lives under a version-named directory that `/procoder:update`
// replaces wholesale. A command naming that directory stops resolving the day
// the version changes, and because the statusline script is simply gone it
// prints nothing at all — the badge disappears with no error anywhere. These
// tests stand a fake versioned cache up, move the version, and insist the
// command written before the move still prints the badge after it.

const POSIX_ONLY = { skip: process.platform === 'win32' && 'POSIX shell only' };

// bin/ requires ../hooks/, so those two directories are the whole plugin as far
// as `statusline install` is concerned.
function versionedInstall(version) {
  const cache = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cache-'));
  tempDirs.push(cache);
  const home = path.join(cache, 'plugins', 'cache', 'procoder', 'procoder', version);
  for (const dir of ['bin', 'hooks']) {
    fs.cpSync(path.join(__dirname, '..', dir), path.join(home, dir), { recursive: true });
  }
  return home;
}

function runStatusLine(command, repo, input = '{}') {
  return spawnSync('bash', ['-c', command], {
    input, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  });
}

function installFrom(home, repo, args = []) {
  return spawnSync('node', [path.join(home, 'bin', 'procoder.js'), 'statusline', 'install', ...args], {
    cwd: repo, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  });
}

test('the command written for a plugin install survives a version bump', POSIX_ONLY, () => {
  const home = versionedInstall('0.2.0');
  const repo = repoWith({});
  atLevel(repo, 'strict');

  const installed = installFrom(home, repo);
  assert.strictEqual(installed.status, 0, String(installed.stdout) + String(installed.stderr));
  const { command } = readSettings(repo).statusLine;
  assert.doesNotMatch(command, /0\.2\.0/, 'the written command must not pin a version');

  fs.renameSync(home, path.join(path.dirname(home), '0.3.0'));
  const r = runStatusLine(command, repo);
  assert.strictEqual(r.status, 0, String(r.stderr));
  assert.strictEqual(String(r.stdout).trim(), '[PROCODER:STRICT]');
});

// The other half of not pinning: when the plugin is gone entirely, the command
// has to fall silent rather than spray a resolution error into the status bar.
test('the plugin command prints nothing, quietly, once the plugin is removed', POSIX_ONLY, () => {
  const home = versionedInstall('0.2.0');
  const repo = repoWith({});
  atLevel(repo, 'strict');
  assert.strictEqual(installFrom(home, repo).status, 0);

  const { command } = readSettings(repo).statusLine;
  fs.rmSync(path.dirname(home), { recursive: true, force: true });
  const r = runStatusLine(command, repo);
  assert.strictEqual(r.status, 0, String(r.stderr));
  assert.strictEqual(String(r.stdout), '');
  assert.strictEqual(String(r.stderr), '');
});

// `--append`: the user's own statusline keeps working and the badge joins it,
// instead of --force being the only offered path.

// Reads all of stdin and echoes it back, which is what makes the shared-stdin
// assertion below real: a composition that pipes the session JSON into only one
// of the two commands leaves this one printing MINE: and nothing after it.
function repoWithMine() {
  const repo = repoWith({ 'mine.sh': 'printf "MINE:%s" "$(cat)"\n' });
  const command = `bash "${path.join(repo, 'mine.sh')}"`;
  fs.writeFileSync(settingsIn(repo),
    JSON.stringify({ model: 'opus', statusLine: { type: 'command', command } }, null, 2) + '\n');
  return { repo, mine: { type: 'command', command } };
}

test('statusline install --append runs both commands, ours last', POSIX_ONLY, () => {
  const { repo, mine } = repoWithMine();
  const result = cli(repo, ['statusline', 'install', '--append']);
  assert.strictEqual(result.code, 0, result.out);
  atLevel(repo, 'strict');

  const entry = readSettings(repo).statusLine;
  assert.deepStrictEqual(entry.procoderOriginal, mine, 'the original has to be recorded verbatim');
  const r = runStatusLine(entry.command, repo, '{"cwd":"/tmp/x"}');
  assert.strictEqual(r.status, 0, String(r.stderr));
  assert.strictEqual(String(r.stdout).trim(), 'MINE:{"cwd":"/tmp/x"} [PROCODER:STRICT]');
});

// The subtle one. Claude Code hands the statusline JSON on stdin, and stdin can
// only be read once, so a naive `a | b` or `a; b` gives one command the session
// context and the other an empty pipe. Both parts must see the same bytes.
test('both composed commands receive the same stdin', POSIX_ONLY, () => {
  const { repo } = repoWithMine();
  assert.strictEqual(cli(repo, ['statusline', 'install', '--append']).code, 0);
  atLevel(repo, 'paranoid');

  const session = '{"cwd":"/some/where","session_id":"abc"}';
  const r = runStatusLine(readSettings(repo).statusLine.command, repo, session);
  assert.strictEqual(r.status, 0, String(r.stderr));
  assert.ok(String(r.stdout).includes(`MINE:${session}`),
    `the existing command must see the whole session JSON, got: ${r.stdout}`);
  assert.ok(String(r.stdout).includes('[PROCODER:PARANOID]'), 'and ours must still run');
});

test('uninstall after --append restores the original command byte for byte', POSIX_ONLY, () => {
  const { repo, mine } = repoWithMine();
  const before = fs.readFileSync(settingsIn(repo), 'utf8');
  assert.strictEqual(cli(repo, ['statusline', 'install', '--append']).code, 0);

  const result = cli(repo, ['statusline', 'uninstall']);
  assert.strictEqual(result.code, 0, result.out);
  assert.deepStrictEqual(readSettings(repo).statusLine, mine);
  assert.strictEqual(readSettings(repo).model, 'opus');
  assert.strictEqual(fs.readFileSync(settingsIn(repo), 'utf8'), before,
    'the settings file has to come back exactly as it was');
});

test('statusline status reports composed as its own state', POSIX_ONLY, () => {
  const { repo } = repoWithMine();
  assert.match(cli(repo, ['statusline', 'status']).out, /not procoder/i);
  assert.strictEqual(cli(repo, ['statusline', 'install', '--append']).code, 0);

  const status = cli(repo, ['statusline', 'status']);
  assert.strictEqual(status.code, 0);
  assert.match(status.out, /composed/i);
  assert.match(status.out, /mine\.sh/, 'it must name the statusline it is composed with');
});

test('--append with no statusline configured is a plain install', POSIX_ONLY, () => {
  const repo = repoWith({});
  assert.strictEqual(cli(repo, ['statusline', 'install', '--append']).code, 0);
  const entry = readSettings(repo).statusLine;
  assert.strictEqual(entry.procoderOriginal, undefined);
  assert.match(entry.command, /procoder-statusline/);
});

test('statusline install --append twice is a no-op the second time', POSIX_ONLY, () => {
  const { repo } = repoWithMine();
  assert.strictEqual(cli(repo, ['statusline', 'install', '--append']).code, 0);
  const first = fs.readFileSync(settingsIn(repo), 'utf8');

  const again = cli(repo, ['statusline', 'install', '--append']);
  assert.strictEqual(again.code, 0, again.out);
  assert.match(again.out, /already/i);
  assert.strictEqual(fs.readFileSync(settingsIn(repo), 'utf8'), first);
});

// --- machine-readable output ------------------------------------------------
//
// A finding that only exists as a text line cannot reach the two places a gate
// has to appear: a pull request's diff, and a security dashboard. Both formats
// are built from the same per-file results the text path prints, so these tests
// assert the shape AND that the counts agree with the default rendering.

const DIRTY = 'eval(x);\n';  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it

test('--format json reports the same findings the text run does, with fingerprints', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  const result = cli(repo, ['check', '--format', 'json', 'a.ts']);
  assert.strictEqual(result.code, 1, 'a SAFE finding still fails the run');
  const doc = JSON.parse(result.out);
  assert.strictEqual(doc.findings.length, 1);
  assert.strictEqual(doc.findings[0].id, 'safe/dynamic-eval');  // procoder: literal safe/dynamic-eval the id as data, not a sink
  assert.strictEqual(doc.findings[0].rungNumber, 1);
  assert.strictEqual(doc.findings[0].blocking, true);
  assert.match(doc.findings[0].fingerprint, /^[0-9a-f]{8,}$/, 'no ratchet fingerprint on the finding');
  assert.strictEqual(doc.summary.blocking, 1);
});

test('--format sarif emits a 2.1.0 document with a rule per id', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  const doc = JSON.parse(cli(repo, ['check', '--format', 'sarif', 'a.ts']).out);
  assert.strictEqual(doc.version, '2.1.0');
  const run = doc.runs[0];
  assert.strictEqual(run.tool.driver.name, 'procoder');
  assert.strictEqual(run.tool.driver.rules.length, 1);
  assert.strictEqual(run.results[0].level, 'error');
  assert.strictEqual(run.results[0].locations[0].physicalLocation.artifactLocation.uri, 'a.ts');
  assert.ok(run.results[0].partialFingerprints.procoderFingerprint,
    'without a fingerprint a dashboard reports every moved line as new');
});

// An advisory finding must not arrive as a SARIF error: a dashboard that fails
// a build on a judgment rung is a dashboard somebody turns off.
test('sarif grades a non-blocking finding as a warning', () => {
  const repo = atLevel(repoWith({ 'a.ts': ADVISORY_ONLY }), 'pragmatic');
  const doc = JSON.parse(cli(repo, ['check', '--format', 'sarif', 'a.ts']).out);
  assert.strictEqual(doc.runs[0].results[0].level, 'warning');
});

// A file nothing read is not a file that came back clean. SARIF is what feeds
// GitHub code scanning and most dashboards, and it used to render the two
// identically — absent from `results` either way — which is the widest-reaching
// version of "enforcement narrower than the user believes" this project has had.
test('sarif names the files that went unread, and why', () => {
  const repo = repoWith({
    'a.ts': DIRTY,
    'big.ts': `// ${'x'.repeat(200)}\n`,
    'skipped/b.ts': DIRTY,
    '.procoder.toml': '[exclude]\npaths = ["skipped/"]\n[limits]\nmax_file_bytes = 64\n',
  });
  // stdout only: the skip notices are on stderr, which is the whole reason the
  // document stays parseable while the coverage it lost is still said out loud.
  const run = JSON.parse(stdoutOf(repo,
    ['check', '--format', 'sarif', 'a.ts', 'big.ts', 'skipped/b.ts'])).runs[0];
  assert.deepStrictEqual(
    run.results.map((r) => r.locations[0].physicalLocation.artifactLocation.uri), ['a.ts'],
    'only the file that was actually read may contribute results');

  const notes = run.invocations[0].toolExecutionNotifications;
  const byFile = new Map(notes.map(
    (n) => [n.locations[0].physicalLocation.artifactLocation.uri, n]));
  assert.strictEqual(notes.length, 2, 'a skipped file that reaches no consumer is an invisible hole');
  assert.strictEqual(byFile.get('big.ts').descriptor.id, 'procoder/skipped/too-large');
  assert.strictEqual(byFile.get('big.ts').level, 'error',
    'a file that was in scope and could not be read is a hole in the scope');
  assert.strictEqual(byFile.get('skipped/b.ts').descriptor.id, 'procoder/skipped/excluded');
  assert.strictEqual(byFile.get('skipped/b.ts').level, 'warning',
    'a deliberate exclusion is coverage lost, not a broken run');
  assert.strictEqual(run.invocations[0].executionSuccessful, false,
    'a run that could not read a file in scope did not fully execute');
  // The descriptor a notification names has to resolve to something.
  assert.deepStrictEqual(
    run.tool.driver.notifications.map((n) => n.id).sort(),
    ['procoder/skipped/excluded', 'procoder/skipped/too-large']);
});

// The other half of the contract: a consumer that ignores notifications must
// never be worse off than it was before they existed.
test('a sarif run with nothing skipped is unchanged in shape', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  const run = JSON.parse(cli(repo, ['check', '--format', 'sarif', 'a.ts']).out).runs[0];
  assert.deepStrictEqual(Object.keys(run), ['tool', 'results']);
  assert.deepStrictEqual(Object.keys(run.tool.driver),
    ['name', 'version', 'informationUri', 'rules']);
});

// A .procoderignore is the same class of skip as an [exclude] path and has to
// arrive the same way — the asymmetry between them is what let whole
// directories of coverage disappear silently once before.
test('an ignored file is a sarif notification too, and does not fail the run', () => {
  const repo = repoWith({ 'gen/.procoderignore': '*.ts\n', 'gen/a.ts': DIRTY });
  const run = JSON.parse(stdoutOf(repo, ['check', '--format', 'sarif', 'gen/a.ts'])).runs[0];
  const note = run.invocations[0].toolExecutionNotifications[0];
  assert.strictEqual(note.descriptor.id, 'procoder/skipped/ignored');
  assert.strictEqual(note.level, 'warning');
  assert.strictEqual(run.invocations[0].executionSuccessful, true,
    'a scope decision the project made is not a failed run');
});

// The mapping the dashboard routes on: rung name and number on the rule, the
// ratchet's own fingerprint on the result, and the level from whether the
// finding blocked at the level THIS file was judged at.
test('sarif carries rung, rung number and fingerprint on an advisory finding', () => {
  const repo = atLevel(repoWith({ 'a.ts': ADVISORY_ONLY }), 'pragmatic');
  const run = JSON.parse(cli(repo, ['check', '--format', 'sarif', 'a.ts']).out).runs[0];
  assert.strictEqual(run.results[0].level, 'warning');
  assert.match(run.results[0].partialFingerprints.procoderFingerprint, /^[0-9a-f]{8,}$/);
  assert.strictEqual(run.tool.driver.rules[0].properties.rung, 'ALONE');
  assert.strictEqual(run.tool.driver.rules[0].properties.rungNumber, 4);
});

// A baselined finding is suppressed everywhere or nowhere. Arriving as a
// `note` would put accepted debt back on the dashboard as a new row.
test('a baselined finding is absent from sarif entirely', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  cli(repo, ['baseline', 'a.ts']);
  const run = JSON.parse(cli(repo, ['check', '--format', 'sarif', 'a.ts']).out).runs[0];
  assert.deepStrictEqual(run.results, []);
  assert.deepStrictEqual(run.tool.driver.rules, []);
});

// A skip notice on stdout would break the document for every consumer of it.
test('a skip notice never lands in the middle of a json document', () => {
  const repo = repoWith({ 'a.ts': DIRTY, '.procoder.toml': '[exclude]\npaths = ["skipped/"]\n' });
  fs.mkdirSync(path.join(repo, 'skipped'));
  fs.writeFileSync(path.join(repo, 'skipped', 'b.ts'), DIRTY);
  const r = spawnSync('node', [CLI, 'check', '--format', 'json', 'a.ts', 'skipped/b.ts'],
    { cwd: repo, encoding: 'utf8', env: { ...process.env, CLAUDE_CONFIG_DIR: repo } });
  assert.doesNotThrow(() => JSON.parse(String(r.stdout)), 'stdout was not a parseable document');
  assert.match(String(r.stderr), /skipped/, 'the skip went unreported');
});

test('an unknown --format is refused, not guessed at', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  const result = cli(repo, ['check', '--format', 'xml', 'a.ts']);
  assert.strictEqual(result.code, 2);
  assert.match(result.out, /--format xml is not available/);
});

// --- --since ----------------------------------------------------------------

function repoWithCommit(files, committed) {
  const repo = repoWith(committed);
  const git = (...args) => spawnSync('git', args, { cwd: repo, encoding: 'utf8' });
  fs.rmSync(path.join(repo, '.git'), { recursive: true, force: true });
  git('init', '-q');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'add', '-A');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-qm', 'base');
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(repo, rel)), { recursive: true });
    fs.writeFileSync(path.join(repo, rel), content);
  }
  return repo;
}

test('--since checks what changed and leaves the rest alone', () => {
  const repo = repoWithCommit({ 'new.ts': DIRTY }, { 'old.ts': DIRTY });
  const result = cli(repo, ['check', '--since', 'HEAD']);
  assert.strictEqual(result.code, 1);
  assert.match(result.out, /new\.ts/);
  assert.doesNotMatch(result.out, /old\.ts/, 'an unchanged file was checked anyway');
});

// The shipped CI template used to do this in shell with `|| true`: a git
// failure produced an empty file list and the build passed green having checked
// nothing. Exit 2 — "cannot check" — is the whole point of moving it in here.
test('--since exits 2 when git cannot resolve the ref', () => {
  const repo = repoWithCommit({}, { 'old.ts': DIRTY });
  const result = cli(repo, ['check', '--since', 'no-such-ref']);
  assert.strictEqual(result.code, 2);
  assert.match(result.out, /git diff .* failed/);
});

// `--diff-filter=ACM` dropped `R`, and git reports a moved file as a rename,
// not as a delete plus an add. So the one commit shape that moves live code —
// a rename — selected no files, printed "nothing to check" and exited 0 over a
// finding that was still there, which is exactly the claim `--since` was
// written to make impossible.
test('--since sees a renamed file, at its new path', () => {
  const repo = repoWithCommit({}, { 'old.ts': DIRTY });
  const git = (...args) => spawnSync('git', ['-c', 'user.email=t@t', '-c', 'user.name=t', ...args],
    { cwd: repo, encoding: 'utf8' });
  git('mv', 'old.ts', 'new.ts');
  git('commit', '-qm', 'rename');
  const result = cli(repo, ['check', '--since', 'HEAD~1']);
  assert.strictEqual(result.code, 1, 'a rename must not pass green over a live finding');
  assert.match(result.out, /new\.ts/, 'the finding belongs at the path the file lives at now');
  assert.doesNotMatch(result.out, /no files changed/);
});

// `--unused-exclusions` and `--aging` were accepted under `--since` and
// silently dropped. A flag that does nothing is how somebody concludes the
// check ran.
test('--since honours --unused-exclusions over the files it read', () => {
  const repo = repoWithCommit(
    { 'a.ts': 'const x = 2;\n' },
    { 'a.ts': 'const x = 1;\n', '.procoder.toml': '[exclude]\nrules = ["a.ts:safe/dynamic-eval"]\n' });
  const result = cli(repo, ['verify', '--unused-exclusions', '--since', 'HEAD']);
  assert.strictEqual(result.code, 1, 'the flag was accepted and then ignored');
  assert.match(result.out, /suppressed nothing/);
});

// The baseline is what `--aging` judges, and no file selection changes it —
// which is why honouring the flag here is the only reading that is true.
test('--since honours --aging', () => {
  const repo = repoWithCommit({ 'b.ts': 'const y = 1;\n' }, { 'a.ts': DIRTY });
  cli(repo, ['baseline', 'a.ts']);
  const result = cli(repo, ['verify', '--aging', '0', '--since', 'HEAD']);
  assert.strictEqual(result.code, 1, 'the flag was accepted and then ignored');
  assert.match(result.out, /older than 0 days/);
});

test('--since says when nothing changed instead of exiting 0 in silence', () => {
  const repo = repoWithCommit({}, { 'clean.ts': 'const x = 1;\n' });
  const result = cli(repo, ['check', '--since', 'HEAD']);
  assert.strictEqual(result.code, 0);
  assert.match(result.out, /no files changed since HEAD/);
});

// --- init -------------------------------------------------------------------

test('init writes a starter config and never touches an existing one', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  assert.strictEqual(cli(repo, ['init']).code, 0);
  const written = fs.readFileSync(path.join(repo, '.procoder.toml'), 'utf8');
  assert.match(written, /\[exclude\]/);

  fs.writeFileSync(path.join(repo, '.procoder.toml'), '# mine\n');
  const second = cli(repo, ['init']);
  assert.match(second.out, /already exists/);
  assert.strictEqual(fs.readFileSync(path.join(repo, '.procoder.toml'), 'utf8'), '# mine\n',
    'init overwrote a config somebody wrote');
});

// The point of --baseline is that an existing repository is green on the first
// run: adopting procoder must not start with a cleanup.
test('init --baseline accepts today findings, so verify passes and new code still fails', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  assert.strictEqual(cli(repo, ['init', '--baseline']).code, 0);
  assert.strictEqual(cli(repo, ['verify', '.']).code, 0);

  fs.writeFileSync(path.join(repo, 'b.ts'), DIRTY);
  assert.strictEqual(cli(repo, ['verify', '.']).code, 1, 'new code rode in on the baseline');
});

// --- verify --aging ---------------------------------------------------------

test('--aging names accepted debt older than the window, and fails on it', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  cli(repo, ['baseline', 'a.ts']);
  const file = path.join(repo, '.procoder-baseline.json');
  const doc = JSON.parse(fs.readFileSync(file, 'utf8'));
  doc.entries[0].added = '2020-01-01';
  fs.writeFileSync(file, JSON.stringify(doc));

  const aged = cli(repo, ['verify', '--aging', '30', 'a.ts']);
  assert.strictEqual(aged.code, 1);
  assert.match(aged.out, /accepted finding.*older than 30 days/);
  assert.match(aged.out, /2020-01-01/);

  assert.strictEqual(cli(repo, ['verify', '--aging', '99999', 'a.ts']).code, 0,
    'a window nothing is older than must pass');
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0,
    'without the flag, age does not fail the run');
});

test('--aging is refused on a command that cannot use it', () => {
  const repo = repoWith({ 'a.ts': DIRTY });
  const result = cli(repo, ['check', '--aging', '30', 'a.ts']);
  assert.strictEqual(result.code, 2);
  assert.match(result.out, /--aging takes a number of days/);
});
