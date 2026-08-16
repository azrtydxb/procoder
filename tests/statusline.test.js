// tests/statusline.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const SCRIPT = path.join(__dirname, '..', 'hooks', 'procoder-statusline.sh');

function run(level) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-'));
  try {
    if (level) fs.writeFileSync(path.join(dir, '.procoder-active'), level + '\n');
    return execFileSync('bash', [SCRIPT], {
      encoding: 'utf8',
      env: { ...process.env, CLAUDE_CONFIG_DIR: dir },
    }).trim();
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test('renders the active level in caps', () => {
  assert.strictEqual(run('strict'), '[PROCODER:STRICT]');
  assert.strictEqual(run('paranoid'), '[PROCODER:PARANOID]');
});

test('renders nothing when no level file exists', () => {
  assert.strictEqual(run(null), '');
});

test('ignores a corrupted level file', () => {
  assert.strictEqual(run('garbage'), '');
});
