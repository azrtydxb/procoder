#!/usr/bin/env node
// procoder — the mandated toolchain.
//
// procoder used to carry ~9,500 lines of hand-written rules: regexes for
// injection sinks, a local taint tracker, shape heuristics per language. They
// were measured against CWEval, whose 115 programs come with working exploits,
// so a miss is a real exploit and not a scanner's opinion. They named the right
// weakness in 1 of 30. That is not a rule set worth maintaining, and every
// language added would have doubled it.
//
// So the rules are gone and real analyzers do the work. This file is the whole
// policy, and there are only four rules to it:
//
//   TIERED      Which analyzers run where is decided by what the tool IS, once,
//               in its definition — never at runtime by a stopwatch. Fast
//               analyzers (ruff, eslint, golangci-lint with gosec, clippy) run
//               on every write and run TO COMPLETION. semgrep needs seconds to
//               load its rules, so it runs at commit. CodeQL builds a database,
//               so it runs from `procoder deep`. procoder does not truncate an
//               analyzer to fit a budget: a tool killed halfway returns an empty
//               result, and an empty result is byte for byte what a clean file
//               returns. A gate whose scheduler can manufacture false clean
//               results is not a gate.
//   MANDATORY   A language in the project must have its analyzers installed.
//               A missing analyzer is reported as a gap in the gate, never as a
//               clean file — "we could not look" and "we looked and it is fine"
//               are different answers and only one is safe to ship on.
//   LATEST      Analyzers move faster than their rule sets are documented; a
//               pinned old version is a rule set nobody is reading. procoder
//               reports one that has fallen behind and says how to update it.
//   COMPLETE    Security rule sets are enabled explicitly. A tool run on its
//               defaults is a tool with its security rules off — ruff's default
//               set (E4, E7, E9, F) contains not one.
//   NO SILENCE  A finding is answered by fixing the code. The only other
//               permitted answer is a rule switched off in the project's own
//               analyzer config, WITH the research that showed it wrong written
//               beside it — reproducible evidence, not an opinion. procoder
//               passes no disable flag of its own and writes no ignore comment,
//               so every silence is one a reviewer can find in one file and
//               argue with. A single wrong finding is answered narrowly instead,
//               with a `procoder:` marker on the line or an entry in
//               .procoder.toml.
//
// What this honestly buys, measured on the same corpus with every tool below at
// its latest and CodeQL 2.26.3 `security-extended` alongside them: 5 of 30
// exploits named correctly. Analyzers find vulnerabilities by tracing a known
// framework entry point to a sink, and code being written has usually not been
// wired to one yet. They are kept because what they DO prove — a weak cipher, a
// hardcoded credential, a shell built from a variable, an unchecked error — they
// prove exactly, in every language, without procoder maintaining a line of it.
// The rung-1 imperative in the prompt is what carries the rest, and the README
// says so rather than implying this file is a vulnerability scanner.

const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Same reason as resolve.js binPath: a project's analyzers live in
// node_modules/.bin far more often than on PATH, and reporting one as missing to
// a project that installed it is the fastest way to have the gate switched off.
function searchPath(repoRoot) {
  const local = repoRoot ? path.join(repoRoot, 'node_modules', '.bin') : null;
  const current = process.env.PATH || '';
  return local && fs.existsSync(local) ? local + path.delimiter + current : current;
}

// Security rule sets, named explicitly. Every flag here turns something ON.
// If you are about to add one that turns something off, read NO SILENCE above.
const ANALYZERS = {
  semgrep: {
    // The only analyzer that covers every language procoder supports, so it is
    // required whenever any source file is present rather than per-language.
    name: 'semgrep',
    languages: '*',
    argv: (files) => [
      '--config=p/security-audit', '--config=p/cwe-top-25', '--config=p/secrets',
      '--json', '--quiet', '--metrics=off', ...files,
    ],
    install: 'pipx install semgrep   (or: brew install semgrep)',
    upgrade: 'pipx upgrade semgrep',
    version: (out) => (out.match(/(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['--version'],
  },

  ruff: {
    name: 'ruff',
    languages: ['py'],
    // S is flake8-bandit: the security rule set. Without it ruff runs E4/E7/E9/F
    // and reports no security finding of any kind.
    argv: (files) => ['check', '--select', 'S,B,E,F', '--output-format', 'json', ...files],
    install: 'brew install ruff   (or: pipx install ruff)',
    upgrade: 'brew upgrade ruff',
    version: (out) => (out.match(/(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['--version'],
  },

  eslint: {
    name: 'eslint',
    languages: ['js', 'jsx', 'mjs', 'cjs', 'ts', 'tsx', 'mts', 'cts'],
    argv: (files) => ['--format', 'json', ...files],
    // eslint carries no security rules of its own; the plugin is the reason it
    // is in this list at all, so its absence is a gap like a missing binary.
    requiresPlugin: 'eslint-plugin-security',
    install: 'npm i -D eslint eslint-plugin-security',
    upgrade: 'npm i -D eslint@latest eslint-plugin-security@latest',
    version: (out) => (out.match(/v?(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['--version'],
  },

  gosec: {
    name: 'gosec',
    languages: ['go'],
    // gosec needs a package that type-checks; a file that does not build is
    // reported as unanalysed rather than clean. See NEEDS_BUILD.
    argv: (files) => ['-fmt=json', '-no-fail', ...files],
    needsBuild: true,
    install: 'go install github.com/securego/gosec/v2/cmd/gosec@latest',
    upgrade: 'go install github.com/securego/gosec/v2/cmd/gosec@latest',
    version: (out) => (out.match(/(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['--version'],
  },

  'golangci-lint': {
    name: 'golangci-lint',
    languages: ['go'],
    argv: (files) => ['run', '--output.json.path', 'stdout', ...files],
    needsBuild: true,
    install: 'brew install golangci-lint',
    upgrade: 'brew upgrade golangci-lint',
    version: (out) => (out.match(/(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['--version'],
  },

  // The deep tier. Declared here so `procoder doctor` reports it like any other
  // mandated analyzer, but it is never spawned per file — see codeql.js for why
  // (a database build, and --threat-model=all, which is worth 8x the recall of
  // the default and is the reason this entry exists at all).
  codeql: {
    name: 'codeql',
    languages: '*',
    tier: 'deep',
    install: 'download the CodeQL CLI: https://github.com/github/codeql-action/releases',
    upgrade: 'replace the CodeQL CLI with the current release',
    version: (out) => (out.match(/(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['version', '--format=terse'],
  },

  cargo: {
    name: 'cargo',
    languages: ['rs'],
    // clippy lints a crate, never a file; -p bounds it to the member that owns
    // the file being written.
    argv: () => ['clippy', '--message-format', 'short', '--quiet'],
    stream: 'stderr',
    install: 'rustup component add clippy',
    upgrade: 'rustup update',
    version: (out) => (out.match(/(\d+\.\d+\.\d+)/) || [])[1],
    versionArgv: ['--version'],
  },
};

// Languages procoder recognises but cannot gate, and the honest reason. Naming
// them here is the point: a silent gap reads as a pass.
// C and C++ were on this list, on the evidence that flawfinder and cppcheck
// prove nothing about them — flawfinder greps for dangerous function names and
// named the right weakness in 2 of 10 against 2.1 by chance, cppcheck reports
// correctness rather than weaknesses and named 0 of 5. That evidence still
// holds for those two tools, and it was the wrong conclusion: semgrep carries
// real C and C++ rules and catches, for instance, `strcat`/`strcpy` into a fixed
// buffer. They are gated, by semgrep, at commit time.
const UNGATED = {
  java: 'no analyzer configured yet',
  kt: 'no analyzer configured yet',
  cs: 'no analyzer configured yet',
};

const EXT_ALIAS = { h: 'c', hpp: 'cpp', cc: 'cpp', cxx: 'cpp', pyi: 'py' };

// The extensions procoder considers source at all. An analyzer scoped to '*'
// means "every language in this list", never "every file on disk" — without it
// a lockfile, a .gitignore and a linter's own cache each earn a rung-1 finding
// saying they were not checked for weaknesses, which is both false and the
// fastest way to teach someone to ignore the gate.
const SOURCE_EXT = new Set([
  'py', 'js', 'jsx', 'mjs', 'cjs', 'ts', 'tsx', 'mts', 'cts',
  'go', 'rs', 'java', 'kt', 'kts', 'cs', 'c', 'h', 'cpp', 'cc', 'cxx', 'hpp',
  'rb', 'php', 'swift', 'scala', 'sh', 'bash',
]);

function isSource(file) {
  return SOURCE_EXT.has(extOf(file));
}

function extOf(file) {
  const raw = String(file || '').split('.').pop().toLowerCase();
  return EXT_ALIAS[raw] || raw;
}

// Which analyzers must be present for this file. `*` languages apply to every
// source file procoder knows how to read at all.
// Analyzers required for a per-write check. The deep tier is excluded: it runs
// from the CLI over a whole tree and cannot answer about one file in 2s, so
// demanding it on every write would report a gap procoder has no intention of
// filling there. `procoder doctor` reports it separately, because a project
// without CodeQL genuinely is missing the taint tier.
function requiredFor(file) {
  if (!isSource(file)) return [];
  const ext = extOf(file);
  const out = [];
  for (const [key, a] of Object.entries(ANALYZERS)) {
    if (a.tier === 'deep') continue;
    if (a.languages === '*' || a.languages.includes(ext)) out.push(key);
  }
  return out;
}

function isUngated(file) {
  return UNGATED[extOf(file)] || null;
}

// --- presence and version ---------------------------------------------------

// Keyed by PATH as well as by analyzer: the answer "ruff is installed" is only
// true for the PATH it was discovered on, and procoder's own tests move PATH
// between scenarios. A cache that ignores that reports an analyzer present after
// it has been taken away — which is the one direction this must never fail in.
const probeCache = new Map();

function probe(key, repoRoot) {
  const search = searchPath(repoRoot);
  const cacheKey = `${key}\u0000${search}`;
  if (probeCache.has(cacheKey)) return probeCache.get(cacheKey);
  const a = ANALYZERS[key];
  let result;
  try {
    const out = execFileSync(a.name, a.versionArgv, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      timeout: 4000,
      env: { ...process.env, PATH: search },
    });
    result = { present: true, version: a.version(out) || null };
  } catch (e) {
    result = { present: false, version: null };
  }
  probeCache.set(cacheKey, result);
  return result;
}

// A gap is anything that stops an analyzer from answering: not installed, or
// installed without the plugin that carries its security rules.
function gapsFor(file, repoRoot) {
  const gaps = [];
  for (const key of requiredFor(file)) {
    const a = ANALYZERS[key];
    const { present, version } = probe(key, repoRoot);
    if (!present) {
      gaps.push({ analyzer: a.name, why: 'not installed', install: a.install });
      continue;
    }
    if (a.requiresPlugin && !hasPlugin(a.requiresPlugin, repoRoot)) {
      gaps.push({
        analyzer: a.name,
        why: `installed, but ${a.requiresPlugin} is missing — its security rules are what procoder relies on`,
        install: a.install,
      });
      continue;
    }
    void version; // currency is a `procoder doctor` question, not a per-write one
  }
  return gaps;
}

function hasPlugin(pkg, repoRoot) {
  try {
    require.resolve(pkg, { paths: [repoRoot || process.cwd(), process.cwd()] });
    return true;
  } catch (e) {
    return false;
  }
}

module.exports = {
  ANALYZERS, UNGATED, SOURCE_EXT, extOf, isSource, requiredFor, isUngated,
  probe, gapsFor, hasPlugin,
};
