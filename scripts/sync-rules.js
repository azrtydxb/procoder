#!/usr/bin/env node
// procoder — renders the doctrine into every platform's rule file.
//
// skills/procoder/SKILL.md is the single source. Ponytail hand-maintains ten
// copies of its doctrine; procoder generates them, because a doctrine that
// forbids stale twins cannot ship with nine of them.
//
// Usage: node scripts/sync-rules.js [--check]

const fs = require('fs');
const path = require('path');
const { getProcoderInstructions } = require('../hooks/procoder-instructions');

const ROOT = path.join(__dirname, '..');
const SOURCE = 'skills/procoder/SKILL.md';

const WARNING = [
  '<!-- DO NOT EDIT. Generated from ' + SOURCE + ' by scripts/sync-rules.js.',
  '     Hand edits are overwritten and fail CI. Edit the source instead. -->',
].join('\n');

const TARGETS = [
  { path: 'AGENTS.md', level: 'strict' },
  { path: '.cursor/rules/procoder.mdc', level: 'strict', frontmatter: '---\nalwaysApply: true\ndescription: procoder — four-rung ship gate\n---\n' },
  { path: '.windsurf/rules/procoder.md', level: 'strict' },
  { path: '.clinerules/procoder.md', level: 'strict' },
  { path: '.kiro/steering/procoder.md', level: 'strict' },
  { path: '.qoder/rules/procoder.md', level: 'strict' },
  { path: '.agents/rules/procoder.md', level: 'strict' },
  { path: '.opencode/command/procoder.md', level: 'strict' },
  { path: '.openclaw/skills/procoder/SKILL.md', level: 'strict' },
];

function render() {
  const out = new Map();
  for (const target of TARGETS) {
    const body = getProcoderInstructions(target.level);
    out.set(target.path, (target.frontmatter || '') + WARNING + '\n\n' + body + '\n');
  }
  return out;
}

function main() {
  const check = process.argv.includes('--check');
  const drifted = [];

  for (const [rel, content] of render()) {
    const abs = path.join(ROOT, rel);
    // An absent target counts as drift; an unreadable one is a real failure and
    // should stop the sync rather than be quietly rewritten.
    const current = fs.existsSync(abs) ? fs.readFileSync(abs, 'utf8') : null;

    if (current === content) continue;
    if (check) { drifted.push(rel); continue; }

    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, content);
    process.stdout.write(`wrote ${rel}\n`);
  }

  if (check && drifted.length) {
    process.stderr.write(
      'procoder: generated rule files are out of sync with ' + SOURCE + ':\n' +
      drifted.map((f) => '  ' + f).join('\n') +
      '\nRun: npm run sync\n');
    process.exit(1);
  }
}

if (require.main === module) main();

module.exports = { render, TARGETS, WARNING };
