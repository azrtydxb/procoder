// tests/commands.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const dir = path.join(__dirname, '..', 'commands');

test('every command file declares a description and a prompt', () => {
  const files = fs.readdirSync(dir).filter((f) => f.endsWith('.toml'));
  assert.ok(files.length >= 2);
  for (const file of files) {
    const raw = fs.readFileSync(path.join(dir, file), 'utf8');
    assert.match(raw, /^description\s*=\s*"/m, `${file} missing description`);
    assert.match(raw, /^prompt\s*=\s*"""/m, `${file} missing prompt`);
  }
});

test('command names match their filenames', () => {
  for (const file of fs.readdirSync(dir).filter((f) => f.endsWith('.toml'))) {
    const raw = fs.readFileSync(path.join(dir, file), 'utf8');
    const expected = path.basename(file, '.toml');
    assert.ok(raw.includes(expected), `${file} does not reference ${expected}`);
  }
});
