const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const CLI = path.join(__dirname, '..', 'bin', 'procoder.js');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cli-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

function cli(repo, args) {
  try {
    return { code: 0, out: execFileSync('node', [CLI, ...args], { cwd: repo, encoding: 'utf8' }) };
  } catch (e) {
    return { code: e.status, out: String(e.stdout || '') + String(e.stderr || '') };
  }
}

test('check exits non-zero and prints findings for a dirty file', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  const result = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /SAFE/);
});

test('check exits 0 for a clean file', () => {
  const repo = repoWith({ 'a.ts': 'const x = 1;\n' });
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('baseline records findings, after which check passes', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  assert.strictEqual(cli(repo, ['baseline', 'a.ts']).code, 0);
  assert.ok(fs.existsSync(path.join(repo, '.procoder-baseline.json')));
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('a NEW violation still fails after a baseline exists', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\n');
  const result = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /xss|innerHTML|SAFE/i);
});

test('verify passes when the baseline has not grown', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
});

test('verify fails when new violations appear on top of the baseline', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\ndebugger;\n');
  const result = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /not in the baseline/i);
});

// Fixing an old finding must not buy room for a new one: the totals are
// identical at step 3, so only fingerprint identity can catch it.
test('verify fails when a baselined finding is swapped for a different one', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);

  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\n');
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 1);

  fs.writeFileSync(path.join(repo, 'a.ts'), 'el.innerHTML = y;\n');
  const swapped = cli(repo, ['verify', 'a.ts']);
  assert.strictEqual(swapped.code, 1);
  assert.match(swapped.out, /innerHTML|xss/i);
});

// Copy-paste is how legacy code grows: every identical line shares id, path and
// normalized content, so without an occurrence ordinal one baselined line
// accepts an unlimited number of clones.
test('verify fails when a baselined violation is cloned', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\n'.repeat(51));

  const verified = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(verified.code, 0);
  assert.match(verified.out, /not in the baseline/i);

  const checked = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(checked.code, 0);
  assert.match(checked.out, /SAFE/);
});

test('verify passes when the baselined duplicate count is unchanged', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n'.repeat(3) });
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('verify passes when a duplicate violation is deleted — shrinking is fine', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n'.repeat(3) });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\n'.repeat(2));
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
    'src/a.ts': 'eval(x);\n',
  });
  assert.strictEqual(cli(repo, ['check', 'src']).code, 0);
});

// A baseline from an older procoder suppresses nothing. Reporting the whole
// backlog as new is the adoption failure the ratchet exists to prevent, so the
// format change has to be said out loud.
const V1_BASELINE = JSON.stringify({ version: 1, fingerprints: ['deadbeef'] });

test('verify against a stale baseline explains the format change instead of failing on counts', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n', '.procoder-baseline.json': V1_BASELINE });
  const result = cli(repo, ['verify', 'a.ts']);
  assert.strictEqual(result.code, 2);
  assert.match(result.out, /baseline.*format|re-run `procoder baseline/i);
  assert.doesNotMatch(result.out, /not in the baseline/i);
});

test('check against a stale baseline says to re-baseline', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n', '.procoder-baseline.json': V1_BASELINE });
  assert.match(cli(repo, ['check', 'a.ts']).out, /fingerprint format changed/i);
});

test('re-baselining over a stale baseline replaces it with the current format', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n', '.procoder-baseline.json': V1_BASELINE });
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
    'a.ts': 'eval(x);\n',
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
