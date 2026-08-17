// tests/docs-drift.test.js
//
// The documentation, checked against the code it describes. Six times in this
// project's short history a page has gone stale — five listing defects that
// were already fixed, once staying silent about two rungs, a worker pool, SARIF
// output and --since while ten commits landed. Every one of them was caught by
// a human reading, or by an audit nobody schedules. This turns "did someone
// remember to update the docs" into a failing test.
//
// Everything guarded here is derived from the source at run time. There is no
// list of rule ids, flags or config keys in this file, because a list is the
// thing that drifts: the day rule 45 lands it is in this test's input without
// anybody touching this test.
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const { BUILTIN_RULE_IDS } = require('../hooks/checks/patterns/markers');
const { RUNGS } = require('../hooks/checks/finding');
const { DEFAULTS } = require('../hooks/checks/config');
const { LEVEL_RANK } = require('../hooks/procoder-config');

const root = path.join(__dirname, '..');
const CONFIG_DOC = 'docs/configuration.html';
const DOCTRINE_DOC = 'docs/doctrine.html';
const COMMANDS_DOC = 'docs/commands.html';
const RULES_DOC = 'docs/limitations.html or another published page';

const read = (rel) => fs.readFileSync(path.join(root, rel), 'utf8');

// The published documentation: README.md and every tracked page under docs/,
// minus docs/superpowers/, which holds planning records for work already done
// and is ignored by procoder's own scan for the same reason — it describes what
// was intended on a day, not what the tool does now.
//
// Membership in this set, rather than one named file per item, is the invariant
// on purpose. Pinning "rule ids live in page X" would fail the day the docs are
// reorganised, and a guard that fails on a legitimate edit is a guard that gets
// deleted. What a reader needs is that the thing is findable in the published
// docs; the failure message still names where it belongs.
function docFiles() {
  return execFileSync('git', ['ls-files'], { cwd: root, encoding: 'utf8' })
    .split('\n')
    .filter((f) => f === 'README.md' || (f.startsWith('docs/') && /\.(md|html)$/.test(f)))
    .filter((f) => !f.startsWith('docs/superpowers/'));
}

// A suppression marker naming a rule is not documentation of that rule — it is
// an author telling the scanner to look away, and it appears in prose about the
// marker mechanism itself. Stripped everywhere, in every guard.
const MARKER = /procoder:\s*literal[^\n]*/gi;

// Code blocks, stripped for the rule-id guard only. `rules =
// ["x.js:obvious/complexity"]` is a config example: it teaches the shape of an
// exclusion, not what the rule detects, and a reader hunting the id learns
// nothing from it. Kept for the CLI and config guards, where the usage block
// and the TOML sample are how you learn a flag or a key.
const docText = (rel) => read(rel).replace(MARKER, ' ');
const prose = (rel) => docText(rel)
  .replace(/```[\s\S]*?```/g, ' ')
  .replace(/<pre[\s\S]*?<\/pre>/g, ' ');

// Matching is on the identifier, never on prose that contains it. Ids are
// collected into a Set and looked up whole, so `safe/dynamic-evall` — a real
// typo in a page here — does not count as documenting `safe/dynamic-eval`.
// Failure mode of that choice: an id split across a line break, or hyphenated
// differently, reads as absent. That direction is the safe one — it asks for a
// mention that is already there rather than accepting one that is not.
const RULE_ID = /\b(?:safe|true|obvious|alone)\/[a-z0-9-]+/g;

function documentedRuleIds() {
  const ids = new Set();
  for (const rel of docFiles()) {
    for (const id of prose(rel).match(RULE_ID) || []) ids.add(id);
  }
  return ids;
}

// Tags dropped and whitespace collapsed, so `<code>procoder check</code>` and a
// line-wrapped `procoder\n  check` both read as the same phrase.
function haystack() {
  return docFiles().map(docText).join('\n')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ');
}

const CLI = read('bin/procoder.js');

// The CLI surface comes from the source, not from `--help`. --help is a string
// that can itself be the thing that drifted — it is documentation, and this
// test exists because documentation drifts. The source is what the program
// actually accepts, and a flag the parser honours but the usage text omits is a
// real failure that only this direction catches. `tests/cli.test.js` is where
// --help is held to the source.
function quoted(section) {
  return [...section.matchAll(/'([a-z-]+)'/g)].map((m) => m[1]);
}

function cliFlags() {
  const flags = new Set();
  for (const m of CLI.matchAll(/const (?:VALUE_)?FLAGS = \[([^\]]*)\]/g)) {
    for (const f of quoted(m[1])) flags.add(f);
  }
  for (const m of CLI.matchAll(/(?:argv|args)\.includes\('(--[a-z-]+)'\)/g)) flags.add(m[1]);
  return [...flags].filter((f) => f.startsWith('--'));
}

function mapKeys(name) {
  const body = CLI.match(new RegExp(`const ${name} = new Map\\(\\[([\\s\\S]*?)\\]\\);`));
  return body ? quoted(body[1]) : [];
}

// Every command as the user types it. `install` alone is a word that appears on
// every page here; `procoder statusline install` is the identifier.
function cliCommands() {
  const commands = mapKeys('COMMANDS')
    .concat([...CLI.matchAll(/command === '([a-z-]+)'/g)].map((m) => m[1]));
  return commands.map((c) => `procoder ${c}`)
    .concat(mapKeys('STATUSLINE').map((c) => `procoder statusline ${c}`));
}

// Config keys are matched as TOML writes them — `key =` in a sample, or
// `[section] key` in prose. Not as bare words: `paths`, `params`, `file` and
// `true` occur in ordinary English on every page, and a guard that accepts
// those can never fail, which is worse than not having it.
function keyDocumented(text, section, key) {
  return new RegExp(`(^|\\s)${key}\\s*=`, 'm').test(text)
    || text.includes(`[${section}] ${key}`);
}

function configEntries() {
  return Object.entries(DEFAULTS).flatMap(([section, value]) => {
    const keys = value && !Array.isArray(value) && typeof value === 'object'
      ? Object.keys(value) : [];
    return [{ section, item: `[${section}]` }]
      .concat(keys.map((key) => ({ section, key, item: `[${section}] ${key}` })));
  });
}

// --- the five guards -------------------------------------------------------

function missingRuleIds() {
  const documented = documentedRuleIds();
  assert.ok(BUILTIN_RULE_IDS.length > 0, 'no rule ids derived — the guard has gone blind');
  return BUILTIN_RULE_IDS.filter((id) => !documented.has(id));
}

// Rung names, from the array the engine validates every finding against. Held
// to two pages rather than to the doc set: the ladder is the product, and a
// rung the doctrine page does not list does not exist as far as a reader is
// concerned. FAST and MEANT shipped emitting findings under exactly that gap.
function missingRungs() {
  assert.strictEqual(RUNGS.length > 4, true, 'no rungs derived — the guard has gone blind');
  const pages = [DOCTRINE_DOC, 'README.md'].map((rel) => docText(rel));
  return RUNGS.filter((rung) => !pages.every((text) => new RegExp(`\\b${rung}\\b`).test(text)))
    .map((rung) => `${rung} (must appear on both ${DOCTRINE_DOC} and README.md)`);
}

function missingCli() {
  const text = haystack();
  const flags = cliFlags();
  const commands = cliCommands();
  assert.ok(flags.length > 0 && commands.length > 0,
    'no flags or commands parsed out of bin/procoder.js — the guard has gone blind');
  return commands.filter((c) => !text.includes(c))
    .concat(flags.filter((f) => !new RegExp(`${f}(?![\\w-])`).test(text)));
}

function missingConfigKeys() {
  const text = docText(CONFIG_DOC);
  const entries = configEntries();
  assert.ok(entries.length > 5, 'no config keys derived — the guard has gone blind');
  return entries.filter((e) => (e.key
    ? !keyDocumented(text, e.section, e.key)
    : !text.includes(e.item))).map((e) => e.item);
}

// The reverse direction: a key the page still documents that the engine no
// longer reads. Only real TOML samples are parsed — a block with a `[section]`
// header — so the .procoderignore and marker examples on the same page are not
// mistaken for config.
function readsKey(section, key) {
  if (section === 'levels') return Object.prototype.hasOwnProperty.call(LEVEL_RANK, key);
  const value = DEFAULTS[section];
  return Boolean(value) && Object.prototype.hasOwnProperty.call(value, key);
}

function blockEntries(block) {
  const entries = [];
  let section = null;
  for (const line of block.split('\n')) {
    const head = line.match(/^\[([a-z_]+)\]/);
    if (head) { section = head[1]; entries.push({ section }); continue; }
    const kv = line.match(/^([a-z_]+)\s*=/);
    if (kv && section) entries.push({ section, key: kv[1] });
  }
  return entries;
}

function documentedButUnread() {
  const blocks = (docText(CONFIG_DOC).match(/<pre><code>[\s\S]*?<\/code><\/pre>/g) || [])
    .map((block) => block.replace(/<\/?(?:pre|code)>/g, ''))
    .filter((block) => /^\[[a-z_]+\]/m.test(block));
  return blocks.flatMap(blockEntries)
    .filter((e) => (e.key ? !readsKey(e.section, e.key) : !DEFAULTS[e.section]))
    .map((e) => (e.key ? `[${e.section}] ${e.key}` : `[${e.section}]`));
}

// The escape hatch. Per item, with a reason, in this file — never a blanket
// skip and never a regex that quietly swallows a category, which is rung 4's
// own rule on suppressions applied to the test that enforces it. An entry here
// is checked below: the day its item becomes documented, the entry fails and
// must be deleted, so the hatch cannot outlive the thing it excused.
const UNDOCUMENTED = [
];

const excused = new Set(UNDOCUMENTED.map((u) => u.item));

const GUARDS = [
  {
    title: 'every rule the engine can report is documented',
    what: 'rule ids the documentation never mentions',
    fix: `Describe each in ${RULES_DOC} — in prose or a table, not only inside a`
      + ' code block or a suppression marker, which is where these already appear',
    missing: missingRuleIds,
  },
  {
    title: 'every rung the engine can emit is documented',
    what: 'rungs the ladder pages do not list',
    fix: `Add each to ${DOCTRINE_DOC} and README.md`,
    missing: missingRungs,
  },
  {
    title: 'every CLI command and flag is documented',
    what: 'CLI commands and flags the documentation does not mention',
    fix: `Document each in ${COMMANDS_DOC} (and in README.md if it belongs there)`,
    missing: missingCli,
  },
  {
    title: 'every config key the engine reads is documented',
    what: 'config keys the reference does not mention',
    fix: `Add each to ${CONFIG_DOC} — in the sample as \`key = value\`, or in prose`
      + ' as `[section] key`',
    missing: missingConfigKeys,
  },
  {
    title: 'every documented config key is still read by the engine',
    what: 'config keys the reference documents that the engine never reads',
    fix: `Delete each from the sample in ${CONFIG_DOC}, or make the engine read it`,
    missing: documentedButUnread,
  },
];

for (const guard of GUARDS) {
  test(guard.title, () => {
    const missing = guard.missing().filter((item) => !excused.has(item));
    assert.ok(missing.length === 0,
      `${guard.what} (${missing.length}):\n  ${missing.join('\n  ')}\n\n${guard.fix}.\n`);
  });
}

test('every documentation exemption still needs to exist', () => {
  const missing = new Set(GUARDS.flatMap((guard) => guard.missing()));
  for (const entry of UNDOCUMENTED) {
    assert.ok(entry.why && entry.why.length > 10,
      `exemption for ${entry.item} has no reason — state why it is undocumented, or delete it`);
    assert.ok(missing.has(entry.item),
      `${entry.item} is documented now — delete its exemption so it is guarded again`);
  }
});
