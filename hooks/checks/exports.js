#!/usr/bin/env node
// procoder — the repository-level half of rung 4.
//
// Every other check in this engine answers for one file, which is why rung 4 —
// the rung procoder exists for — had no engine behind it at all: "you left a
// twin behind" is a claim about the tree. `alone/*` could only ever catch the
// single-file shapes (commented-out code, an undated deferral, a suppression),
// and "this export has no callers left" was delegated entirely to a model
// running greps.
//
// This is the index that makes it deterministic: every exported name in the
// scanned set, and every place any of those names is mentioned. It reports
// CANDIDATES, never verdicts — see the reachability limits below, which are the
// same five the rot skill has always applied by hand.

const path = require('path');
const { finding } = require('./finding');

// One pattern per language for "this file offers this name to other files".
// Deliberately the declaration forms only: an `export { a, b }` list re-exports
// names declared elsewhere, and following it would report the declaration and
// the re-export as two dead symbols.
//
// Every pattern is anchored at the start of a line, so a name inside a string
// or a comment cannot look like a declaration.
const EXPORT_PATTERNS = {
  ts: [
    /^export\s+(?:async\s+)?function\s+([A-Za-z_$][\w$]*)/,
    /^export\s+(?:const|let|var)\s+([A-Za-z_$][\w$]*)/,
    /^export\s+(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)/,
    /^export\s+(?:interface|type|enum)\s+([A-Za-z_$][\w$]*)/,
    /^exports\.([A-Za-z_$][\w$]*)\s*=/,
  ],
  py: [
    // Top-level only (column 0): a nested def is not a module's surface. A
    // leading underscore is Python's own "not part of the API".
    /^(?:async\s+)?def\s+([A-Za-z][\w]*)/,
    /^class\s+([A-Za-z][\w]*)/,
  ],
  go: [
    // Exported in Go is spelled with a capital, by the language.
    /^func\s+([A-Z]\w*)/,
    /^type\s+([A-Z]\w*)/,
    /^(?:var|const)\s+([A-Z]\w*)/,
  ],
  rust: [
    /^pub\s+(?:async\s+)?fn\s+([a-z_]\w*)/,
    /^pub\s+(?:struct|enum|trait|type)\s+([A-Za-z_]\w*)/,
  ],
  jvm: [/^\s*public\s+(?:final\s+|abstract\s+|static\s+)*(?:class|interface|enum|record)\s+(\w+)/],
  dotnet: [/^\s*public\s+(?:sealed\s+|abstract\s+|static\s+|partial\s+)*(?:class|interface|enum|record|struct)\s+(\w+)/],
};

const LANG_BY_EXT = new Map([
  ['.ts', 'ts'], ['.tsx', 'ts'], ['.js', 'ts'], ['.jsx', 'ts'], ['.mjs', 'ts'], ['.cjs', 'ts'],
  ['.py', 'py'], ['.go', 'go'], ['.rs', 'rust'],
  ['.java', 'jvm'], ['.kt', 'jvm'], ['.scala', 'jvm'],
  ['.cs', 'dotnet'],
]);

const WORD_RE = /[A-Za-z_$][\w$]*/g;

// A name that appears inside a quoted string in CODE is reachable by a route
// this index cannot follow: dynamic dispatch, a DI container, a route table,
// `getattr`, reflection. Those become "needs confirmation" rather than
// disappearing from the report — a silent omission would be this tool deciding
// on its own that a name is fine.
//
// In practice this tier fires on SELF-REGISTRATION — `register("routeIt",
// routeIt)` in the declaring file — and that is the whole of its reach. A
// string naming the symbol in some OTHER file is counted by wordCounts like any
// other word, so the symbol never reaches this test at all: it is simply not
// reported. Both directions are safe (a live symbol is never called dead), and
// the message below says which of the two a finding is.
//
// Bounded at 200 characters, and code files only. Prose is full of apostrophes,
// and an unbounded span between two of them swallows whole paragraphs: scanning
// this repository's own markdown, every single candidate came back "needs
// confirmation" because some sentence elsewhere had used the word between two
// apostrophes. A string literal that reaches a symbol by name is short.
const QUOTED = (name) => new RegExp(`["'\`][^"'\`\n]{0,200}\\b${name}\\b[^"'\`\n]{0,200}["'\`]`);  // procoder: literal safe/eslint:security/detect-non-literal-regexp name comes from an identifier capture, so it has no metacharacters

function languageOf(relPath) {
  return LANG_BY_EXT.get(path.extname(relPath).toLowerCase()) || null;
}

// The first pattern that claims this line, or null. Separate from the walk
// below because one line answers to at most one declaration form, and the
// search for it is its own question.
function declaredOn(line, patterns) {
  for (const re of patterns) {
    const m = re.exec(line);
    if (m) return m[1];
  }
  return null;
}

// CommonJS puts its whole surface in one object at the bottom of the file:
// `module.exports = { a, b, c }`, often spread over several lines. That is the
// dominant shape in Node tooling — this repository is written that way — and a
// scanner that only understood ESM `export` found nothing at all in it.
//
// The keys are the exported names: `{ checkFile, MAX_FINDINGS }` exports two,
// and `{ run: runCheck }` exports `run`. Bounded at 40 lines because an export
// block is a list, and a `{` that never closes must not swallow the file.
const CJS_OPEN = /^\s*module\.exports\s*=\s*\{/;
const CJS_KEY = /^\s*([A-Za-z_$][\w$]*)\s*[,:]?\s*$|^\s*([A-Za-z_$][\w$]*)\s*:/;
const CJS_MAX_LINES = 40;

function commonjsExports(relPath, lines, start) {
  const found = [];
  for (let i = start + 1; i < lines.length && i - start <= CJS_MAX_LINES; i += 1) {
    if (/^\s*\}/.test(lines[i])) break;
    // One line may list several names: `checkFile, MAX_FINDINGS,`.
    for (const part of lines[i].split(',')) {
      const m = CJS_KEY.exec(part);
      if (m) found.push({ name: m[1] || m[2], line: start + 1, file: relPath });
    }
  }
  return found;
}

// Every exported name in one file, with the line it is declared on.
function exportsIn(relPath, source) {
  const lang = languageOf(relPath);
  if (!lang) return [];
  const found = [];
  const lines = source.split(/\r?\n/);
  lines.forEach((line, index) => {
    if (lang === 'ts' && CJS_OPEN.test(line)) {
      found.push(...commonjsExports(relPath, lines, index));
      return;
    }
    const name = declaredOn(line, EXPORT_PATTERNS[lang]);
    if (name) found.push({ name, line: index + 1, file: relPath });
  });
  return found;
}

// Word counts for one file. Counting rather than collecting: a name's own
// declaration mentions it once, and the question is whether anything ELSE does.
function wordCounts(source) {
  const counts = new Map();
  const words = String(source).match(WORD_RE) || [];
  for (const word of words) counts.set(word, (counts.get(word) || 0) + 1);
  return counts;
}

// The whole index, built in one pass over the files the caller hands it.
//
// `read` is injected so this module never touches the filesystem: the CLI
// already has the source in hand for every file it walks, and re-reading the
// tree here would double the I/O of a scan whose whole point is that it is one
// pass.
function buildIndex(files, read) {
  const index = { exports: [], counts: new Map(), quoted: new Map() };
  for (const relPath of files) {
    const source = read(relPath);
    if (source === null) continue;
    index.exports.push(...exportsIn(relPath, source));
    index.counts.set(relPath, wordCounts(source));
    // Only code carries a name-as-string reference. A mention in a document is
    // a mention, and is already counted above.
    if (languageOf(relPath)) index.quoted.set(relPath, source);
  }
  return index;
}

function mentionsOutside(index, name, ownFile) {
  let total = 0;
  for (const [relPath, counts] of index.counts) {
    if (relPath === ownFile) continue;
    total += counts.get(name) || 0;
  }
  return total;
}

function quotedAnywhere(index, name) {
  const re = QUOTED(name);
  for (const source of index.quoted.values()) if (re.test(source)) return true;
  return false;
}

// Entry points a repository declares outside its source: a published package's
// `main`/`bin`/`exports`, and the conventional index file. A name exported from
// one of these has callers this index will never see, and reporting it would be
// the one failure that makes a rot report untrustworthy.
const ENTRY_BASENAMES = new Set(['index.ts', 'index.js', 'index.mjs', 'mod.rs', 'lib.rs', '__init__.py']);

function isEntryPoint(relPath, entryFiles) {
  return entryFiles.has(relPath) || ENTRY_BASENAMES.has(path.basename(relPath));
}

// Candidates, one finding each, at rung 4.
//
// Two tiers, and the difference is the whole honesty of this command: a name
// nothing mentions is a deletion; a name mentioned only inside a string is
// `needs confirmation`, because the mention may be a route table or it may be a
// coincidence, and this index cannot tell.
function deadExports(index, { entryFiles = new Set() } = {}) {
  const findings = [];
  for (const symbol of index.exports) {
    if (isEntryPoint(symbol.file, entryFiles)) continue;
    if (mentionsOutside(index, symbol.name, symbol.file) > 0) continue;
    const quoted = quotedAnywhere(index, symbol.name);
    findings.push({
      ...finding({
        rung: 'ALONE',
        id: 'alone/dead-export',
        line: symbol.line,
        message: quoted
          ? `${symbol.name} is exported and mentioned nowhere else, but its own file names it in a string`
          : `${symbol.name} is exported and mentioned nowhere else in the scan`,
        fix: quoted
          ? 'confirm it is not reached by name (routing, DI, reflection) — then delete'
          : 'delete it, or say why it is public',
      }),
      file: symbol.file,
      needsConfirmation: quoted,
    });
  }
  return findings;
}

module.exports = { exportsIn, buildIndex, deadExports };
