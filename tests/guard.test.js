// tests/guard.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.join(__dirname, '..');
const preCommit = fs.readFileSync(path.join(root, 'scripts/templates/pre-commit.sh'), 'utf8');
const ci = fs.readFileSync(path.join(root, 'scripts/templates/procoder-ci.yml'), 'utf8');
const skill = fs.readFileSync(path.join(root, 'skills/guard/SKILL.md'), 'utf8');

test('pre-commit template checks only staged files', () => {
  assert.match(preCommit, /git diff --cached --name-only/);
  assert.match(preCommit, /procoder(?:\.js)?\s+check/);
  assert.match(preCommit, /^set -euo pipefail$/m);
});

test('pre-commit template exits non-zero on findings', () => {
  assert.match(preCommit, /exit 1|exit \$/);
});

test('pre-commit template is valid bash', () => {
  const file = path.join(os.tmpdir(), `procoder-pc-${Date.now()}.sh`);
  fs.writeFileSync(file, preCommit);
  assert.doesNotThrow(() => execFileSync('bash', ['-n', file]));
  fs.unlinkSync(file);
});

test('CI template runs check and enforces the ratchet via the CLI', () => {
  assert.match(ci, /procoder(?:\.js)?\s+check/);
  assert.match(ci, /procoder(?:\.js)?\s+verify/);
  assert.match(ci, /runs-on:/);
  // The ratchet must use the CLI, not a shell approximation of the count.
  assert.ok(!/grep -c/.test(ci), 'CI template counts baseline entries by hand');
});

test('the skill names both files before writing them', () => {
  assert.match(skill, /pre-commit/);
  assert.match(skill, /\.github\/workflows/);
  assert.match(skill, /ask|confirm|before writing/i);
});

test('the skill states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
