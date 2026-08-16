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

function extractField(raw, field) {
  const match = raw.match(new RegExp(`^${field}\\s*=\\s*"([^"]*)"`, 'm'));
  return match ? match[1] : '';
}

function extractPrompt(raw) {
  const match = raw.match(/^prompt\s*=\s*"""([\s\S]*?)"""/m);
  return match ? match[1] : '';
}

// Each command's own name (minus the "procoder" / "procoder-" prefix) must
// show up in the description, and the prompt must actually invoke the skill
// or behavior the filename promises — not just contain the filename string
// anywhere, which would pass even for a command wired to the wrong skill.
const EXPECTATIONS = {
  'procoder.toml': { descriptionContains: 'intensity level', promptContains: '/procoder' },
  'procoder-help.toml': { descriptionContains: 'rungs', promptContains: 'procoder-help skill' },
  'procoder-deps.toml': { descriptionContains: 'dependencies', promptContains: 'procoder-deps skill' },
};

test('each command is wired to the behavior its filename promises', () => {
  const files = fs.readdirSync(dir).filter((f) => f.endsWith('.toml'));
  assert.deepStrictEqual(files.sort(), Object.keys(EXPECTATIONS).sort(),
    'EXPECTATIONS must be updated when commands are added or removed');
  for (const file of files) {
    const raw = fs.readFileSync(path.join(dir, file), 'utf8');
    const { descriptionContains, promptContains } = EXPECTATIONS[file];
    const description = extractField(raw, 'description');
    const prompt = extractPrompt(raw);
    assert.ok(description.length > 0, `${file} has an empty description`);
    assert.ok(description.includes(descriptionContains),
      `${file} description does not mention "${descriptionContains}"`);
    assert.ok(prompt.includes(promptContains),
      `${file} prompt does not reference "${promptContains}"`);
  }
});
