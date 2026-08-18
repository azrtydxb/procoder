#!/usr/bin/env node
// procoder — the deep tier: CodeQL.
//
// Everything else procoder runs is a pattern matcher. It reads a file, matches a
// shape, and reports. That catches API-level weaknesses exactly — a weak cipher,
// a hardcoded credential, `shell=True` — and it cannot catch the other half,
// where the code is idiomatic and only the data's provenance makes it wrong:
//
//     return fmt.Sprintf("[%s] Received: %s", timestamp, msg)
//
// That is a log-injection vulnerability when `msg` is attacker-controlled and
// ordinary Go otherwise. No pattern distinguishes the two. Taint analysis does,
// and CodeQL is the taint analysis available.
//
// --- the flag this file exists for ------------------------------------------
//
// CodeQL's default threat model treats command-line arguments, environment
// variables and local files as TRUSTED. That is a reasonable default for a
// long-lived service and precisely wrong for the code an agent writes, which is
// usually a function or a CLI whose caller has not been written yet. Measured on
// CWEval, whose programs read from argv and ship with working exploits:
//
//     security-extended, default threat model     1 of 15 caught   (7%)
//     security-extended, --threat-model=all       8 of 15 caught  (53%)
//
// Eight times the recall from one flag. It is not configurable here, and that is
// deliberate: a procoder gate that trusted argv would be answering a question
// nobody asked.
//
// --- why it is not in the write hook ----------------------------------------
//
// CodeQL builds a database before it can answer anything: tens of seconds for an
// interpreted language, minutes for a compiled one. The write hook has 2,000ms.
// So this tier runs from the CLI — `procoder deep`, a pre-commit hook, CI — and
// the write hook says nothing about it rather than pretending.
//
// --- what it is worth, per language -----------------------------------------
//
// Measured on the same 115 generated programs, 30 exploitable:
//
//     go     7/11    taint analysis at its best
//     c      5/10
//     js     1/2     small sample
//     py     0/2     small sample
//     cpp    1/5     still the weakest, but no longer zero
//
// C++ remains the weakest language here. Three alternatives were measured
// against it and none beat this: clang-tidy (clang-analyzer/bugprone/cert) fires
// on all five uncaught files but only ever with easily-swappable-parameters and
// unchecked-return — noise that happens to land on the right file; flawfinder
// greps for dangerous function names and scored 2 of 10 against 2.1 by chance;
// cppcheck reports correctness, not weaknesses, and scored 0 of 5.

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { finding } = require('./finding');

// Languages CodeQL can extract without being told how to build the project.
// Everything else needs `[codeql] build_command` in .procoder.toml, because
// CodeQL learns a compiled language by WATCHING a build, and procoder cannot
// guess a build.
const AUTO_LANGUAGES = new Map([
  ['.py', 'python'], ['.pyi', 'python'],
  ['.js', 'javascript'], ['.jsx', 'javascript'], ['.mjs', 'javascript'],
  ['.cjs', 'javascript'], ['.ts', 'javascript'], ['.tsx', 'javascript'],
  ['.rb', 'ruby'],
  ['.go', 'go'],
]);

const BUILT_LANGUAGES = new Map([
  ['.c', 'cpp'], ['.h', 'cpp'], ['.cpp', 'cpp'], ['.cc', 'cpp'],
  ['.cxx', 'cpp'], ['.hpp', 'cpp'],
  ['.java', 'java'], ['.kt', 'java'],
  ['.cs', 'csharp'],
]);

const CWE_TAG = /external\/cwe\/cwe-0*(\d+)/;

function hasCodeql() {
  const probe = spawnSync('codeql', ['version', '--format=terse'], {
    encoding: 'utf8', timeout: 10000, stdio: ['ignore', 'pipe', 'ignore'],
  });
  return probe.status === 0 ? String(probe.stdout || '').trim() : null;
}

// Which languages are present, split by whether CodeQL can build them alone.
function languagesIn(files) {
  const auto = new Set();
  const built = new Set();
  for (const file of files) {
    const ext = path.extname(String(file)).toLowerCase();
    if (AUTO_LANGUAGES.has(ext)) auto.add(AUTO_LANGUAGES.get(ext));
    else if (BUILT_LANGUAGES.has(ext)) built.add(BUILT_LANGUAGES.get(ext));
  }
  return { auto, built };
}

function run(argv, { cwd, timeoutMs }) {
  return spawnSync('codeql', argv, {
    cwd,
    encoding: 'utf8',
    timeout: timeoutMs,
    maxBuffer: 64 * 1024 * 1024,
    shell: false,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

// One database per language, under a directory the caller owns. Rebuilt every
// run: a stale database reports yesterday's code as today's, and "the finding
// was fixed an hour ago" is how a gate loses its reader.
function buildDatabase(language, { repoRoot, dbRoot, buildCommand, timeoutMs }) {
  const db = path.join(dbRoot, `db-${language}`);
  const argv = [
    'database', 'create', db,
    `--language=${language}`,
    `--source-root=${repoRoot}`,
    '--overwrite',
  ];
  if (buildCommand) argv.push(`--command=${buildCommand}`);
  const proc = run(argv, { cwd: repoRoot, timeoutMs });
  if (proc.status !== 0) {
    return { db: null, error: firstLine(proc.stderr || proc.stdout) || 'database build failed' };
  }
  return { db, error: null };
}

function analyzeDatabase(db, language, { repoRoot, timeoutMs }) {
  const sarif = `${db}.sarif`;
  const proc = run([
    'database', 'analyze', db,
    // security-and-quality, not security-extended. Measured on CWEval, both at
    // --threat-model=all: extended caught 10 of 30 and this catches 14, at the
    // same precision (39% against 42%, on a 26% base rate). The whole difference
    // is C and C++, where extended caught 2 of 15 and this catches 6 — the
    // reliability queries it adds turn out to cover weaknesses that the security
    // suite alone reaches only through a taint path C rarely gives it.
    `${language}-security-and-quality.qls`,
    // Not negotiable — see the header. Without it CodeQL treats argv, env and
    // local files as trusted and answers a question about a different program.
    '--threat-model=all',
    '--format=sarif-latest',
    `--output=${sarif}`,
    '--rerun',
  ], { cwd: repoRoot, timeoutMs });
  if (proc.status !== 0) {
    return { sarif: null, error: firstLine(proc.stderr || proc.stdout) || 'analysis failed' };
  }
  return { sarif, error: null };
}

function firstLine(text) {
  const line = String(text || '').split('\n').map((s) => s.trim()).filter(Boolean)[0];
  return line ? line.slice(0, 200) : null;
}

// SARIF → procoder findings.
//
// The rung comes from the QUERY, not from the suite. `security-and-quality`
// contains both halves by definition, so mapping every result to rung 1 would
// file a missing-override warning next to a path traversal and make rung 1 mean
// nothing. CodeQL tags its own queries: a query carrying `security` or an
// `external/cwe/cwe-NNN` tag is a weakness, and everything else is correctness
// or maintainability, which is rung 2.
function findingsFrom(sarifPath, repoRoot) {
  const out = new Map();
  let doc;
  try {
    doc = JSON.parse(fs.readFileSync(sarifPath, 'utf8'));
  } catch (e) {
    return out;
  }
  for (const runResult of doc.runs || []) {
    const rules = new Map();
    for (const rule of (runResult.tool && runResult.tool.driver && runResult.tool.driver.rules) || []) {
      const tags = (rule.properties && rule.properties.tags) || [];
      const cwe = tags.map((t) => (CWE_TAG.exec(t) || [])[1]).find(Boolean);
      const security = !!cwe || tags.includes('security');
      rules.set(rule.id, { cwe, security, name: rule.id });
    }
    for (const result of runResult.results || []) {
      const rule = rules.get(result.ruleId) || { name: result.ruleId };
      const message = (result.message && result.message.text) || rule.name;
      for (const loc of result.locations || []) {
        const physical = loc.physicalLocation || {};
        const uri = (physical.artifactLocation && physical.artifactLocation.uri) || '';
        if (!uri) continue;
        const rel = uri.replace(/^file:\/\//, '');
        const abs = path.isAbsolute(rel) ? rel : path.join(repoRoot, rel);
        const line = (physical.region && physical.region.startLine) || 1;
        const rung = rule.security ? 'SAFE' : 'TRUE';
        if (!out.has(abs)) out.set(abs, []);
        out.get(abs).push(finding({
          rung,
          id: `${rung.toLowerCase()}/codeql:${rule.name}`,
          line,
          message: `${rule.cwe ? `CWE-${rule.cwe}: ` : ''}${message}`.slice(0, 160),
          fix: rule.security
            ? 'break the path from the untrusted source to this sink — CodeQL traced it, so it is reachable'
            : 'resolve the CodeQL finding',
        }));
      }
    }
  }
  return out;
}

// The whole tier, as one call. Returns findings per absolute path, plus the
// languages it could not analyse and why — never silence.
function deepScan(files, {
  repoRoot,
  dbRoot = path.join(os.tmpdir(), `procoder-codeql-${process.pid}`),
  buildCommand = null,
  timeoutMs = 20 * 60 * 1000,
  onProgress = () => {},
} = {}) {
  const version = hasCodeql();
  if (!version) {
    return {
      findings: new Map(),
      version: null,
      skipped: [{
        language: 'all',
        why: 'codeql is not installed',
        fix: 'install the CodeQL CLI: https://github.com/github/codeql-action/releases',
      }],
    };
  }

  const { auto, built } = languagesIn(files);
  const skipped = [];
  const languages = [...auto];
  for (const language of built) {
    if (buildCommand) languages.push(language);
    else {
      skipped.push({
        language,
        why: 'CodeQL learns a compiled language by watching a build, and none is configured',
        fix: 'set [codeql] build_command in .procoder.toml to the command that compiles this project',
      });
    }
  }
  if (!languages.length && !skipped.length) {
    return { findings: new Map(), version, skipped: [] };
  }

  fs.mkdirSync(dbRoot, { recursive: true });
  const findings = new Map();
  for (const language of languages) {
    onProgress(`codeql: building the ${language} database`);
    const build = buildDatabase(language, {
      repoRoot, dbRoot, timeoutMs, buildCommand: built.has(language) ? buildCommand : null,
    });
    if (!build.db) {
      skipped.push({ language, why: build.error, fix: 'fix the build, or exclude this language' });
      continue;
    }
    onProgress(`codeql: analysing ${language}`);
    const analysis = analyzeDatabase(build.db, language, { repoRoot, timeoutMs });
    if (!analysis.sarif) {
      skipped.push({ language, why: analysis.error, fix: 'rerun with --verbose to see the query error' });
      continue;
    }
    for (const [file, list] of findingsFrom(analysis.sarif, repoRoot)) {
      if (!findings.has(file)) findings.set(file, []);
      findings.get(file).push(...list);
    }
  }
  return { findings, version, skipped };
}

module.exports = {
  deepScan, hasCodeql, languagesIn, findingsFrom, AUTO_LANGUAGES, BUILT_LANGUAGES,
};
