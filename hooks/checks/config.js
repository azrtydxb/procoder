#!/usr/bin/env node
// procoder — .procoder.toml loading, defaults, and path exclusion.

const fs = require('fs');
const path = require('path');
const { parseToml } = require('./toml');

const DEFAULTS = {
  exclude: { paths: ['node_modules/', 'vendor/', 'dist/', 'build/', '.git/'], rules: [] },
  thresholds: { function_lines: 40, nesting_depth: 3, params: 4, complexity: 10 },
  // Rungs 1-2 are facts (it is injectable, or it is not); rungs 3-4 are
  // judgment. Only `pragmatic` acts on the difference — see procoder-check.js.
  rungs: { safe: 'error', true: 'error', obvious: 'warn', alone: 'warn' },
  baseline: { file: '.procoder-baseline.json' },
};

function findRepoRoot(startDir) {
  let dir = path.resolve(startDir);
  for (;;) {
    if (fs.existsSync(path.join(dir, '.git'))) return dir;
    const parent = path.dirname(dir);
    if (parent === dir) return path.resolve(startDir);
    dir = parent;
  }
}

function mergeSection(defaults, override) {
  if (!override || typeof override !== 'object') return { ...defaults };
  const out = { ...defaults };
  for (const [key, value] of Object.entries(override)) {
    if (value !== undefined && value !== null) out[key] = value;
  }
  return out;
}

function loadConfig(repoRoot) {
  let raw = {};
  const file = path.join(repoRoot, '.procoder.toml');
  try {
    raw = parseToml(fs.readFileSync(file, 'utf8'));
  } catch (e) {
    // Having no config is the normal case, and defaults are the whole point of
    // having them. Having one that cannot be read is not: silently falling back
    // would run every check under settings the author never chose, so say so.
    if (e.code !== 'ENOENT') {
      process.stderr.write(`procoder: cannot read ${file} (${e.message}) — using defaults\n`);
    }
  }

  return {
    root: repoRoot,
    exclude: {
      paths: Array.isArray(raw.exclude && raw.exclude.paths)
        ? DEFAULTS.exclude.paths.concat(raw.exclude.paths)
        : DEFAULTS.exclude.paths.slice(),
      rules: parseRuleExclusions(raw.exclude && raw.exclude.rules),
    },
    thresholds: mergeSection(DEFAULTS.thresholds, raw.thresholds),
    rungs: mergeSection(DEFAULTS.rungs, raw.rungs),
    baseline: mergeSection(DEFAULTS.baseline, raw.baseline),
  };
}

// Patterns are directory prefixes ("vendor/") or simple globs ("**/*.gen.ts").
// Deliberately not a full glob engine — the config only needs these two shapes.
function matchesPattern(pattern, normalized) {
  if (pattern.endsWith('/')) {
    return normalized === pattern.slice(0, -1) || normalized.startsWith(pattern);
  }
  // `?` is not a documented glob wildcard here (only `*` and `**` are), so it
  // must be escaped like the other regex metacharacters, else patterns such as
  // "?abc.ts" build an invalid regex ("nothing to repeat").
  try {
    const regex = new RegExp(
      '^' + pattern
        .replace(/[.+^${}()|[\]\\?]/g, '\\$&')
        .replace(/\*\*\//g, '(?:.*/)?')
        .replace(/\*/g, '[^/]*') + '$');
    return regex.test(normalized);
  } catch (e) {
    // Belt-and-braces: any pattern shape we didn't anticipate must never throw
    // into the hook. An exclude that fails to match is far better than a
    // crashed session, so fall back to a plain literal comparison.
    return normalized === pattern;
  }
}

// --- .procoderignore ------------------------------------------------------
//
// A per-directory ignore file, applying to its own directory and everything
// beneath it, so a large generated subtree is excluded by one file next to it
// rather than by a central list that grows stale far from what it describes.
//
// The supported syntax is a documented subset of .gitignore, and nothing more:
// one pattern per line; `#` comments; blank lines; `!` negation; a trailing `/`
// for directory-only; a leading `/` anchoring to the file's own directory; `*`
// within a path segment and `**` across them. Character classes, `?` and
// backslash escapes are NOT supported — they match literally, as they already
// do in [exclude] paths. Anything unparseable is dropped, never guessed at.
const IGNORE_FILE = '.procoderignore';

// Per-config, so tests and long-lived callers each get their own, and a config
// object stays plain data (it is spread and compared elsewhere).
const IGNORE_CACHES = new WeakMap();

// One glob-free stretch: `*` matches within a path segment, everything else is
// literal — `?`, character classes and escapes are not in the supported subset.
function literalBody(stretch) {
  return stretch.replace(/[.+^${}()|[\]\\?]/g, '\\$&').replace(/\*/g, '[^/]*');
}

// `**/` spans zero or more directories, so "b/**/*.ts" covers "b/x.ts" too; a
// bare `**` spans anything.
function globBody(pattern) {
  return pattern.split('**/')
    .map((part) => part.split('**').map(literalBody).join('.*'))
    .join('(?:.*/)?');
}

// One line → a matcher, or null for a blank, a comment, or anything unusable.
function compileIgnore(line, base) {
  let pattern = line.trim();
  if (!pattern || pattern.startsWith('#')) return null;
  const negate = pattern.startsWith('!');
  if (negate) pattern = pattern.slice(1);
  const dirOnly = pattern.endsWith('/');
  if (dirOnly) pattern = pattern.replace(/\/+$/, '');
  // A slash anywhere anchors the pattern to the directory holding the ignore
  // file, as git does; a bare name matches at any depth below it.
  const anchored = pattern.includes('/');
  if (pattern.startsWith('/')) pattern = pattern.slice(1);
  // No pattern may name a parent: matching is against a path relative to the
  // ignore file's own directory, which never contains "..", but dropping these
  // says so explicitly rather than relying on the match failing.
  if (!pattern || pattern.split('/').includes('..')) return null;
  try {
    return {
      base,
      negate,
      re: new RegExp(`^${anchored ? '' : '(?:.*/)?'}${globBody(pattern)}${dirOnly ? '/.+' : '(?:/.+)?'}$`),
    };
  } catch (e) {
    // A pattern shape we did not anticipate must never throw into the hook.
    // Ignoring nothing is the safe direction: the file stays gated.
    return null;
  }
}

function readIgnore(root, relDir) {
  let text = '';
  try {
    text = fs.readFileSync(path.join(root, relDir, IGNORE_FILE), 'utf8');
  } catch (e) {
    // Having no ignore file is the normal case, and one that cannot be read
    // degrades to the same thing rather than crashing a hook.
    return [];
  }
  return text.split(/\r?\n/).map((line) => compileIgnore(line, relDir)).filter(Boolean);
}

// Patterns in force for `relDir`, root-most first: a directory inherits its
// parent's and appends its own. Cached per directory, so a run over a deep
// tree reads each ignore file once and walks each chain once — the hook has
// 2s per file and must not re-walk from the repo root every time.
function ignoreRulesFor(config, relDir) {
  let cache = IGNORE_CACHES.get(config);
  if (!cache) { cache = new Map(); IGNORE_CACHES.set(config, cache); }
  const hit = cache.get(relDir);
  if (hit) return hit;
  const cut = relDir.lastIndexOf('/');
  const rules = (relDir === '' ? [] : ignoreRulesFor(config, cut === -1 ? '' : relDir.slice(0, cut)))
    .concat(readIgnore(config.root, relDir));
  cache.set(relDir, rules);
  return rules;
}

// The ignore file that excludes `relPath`, or null. Last match wins, and a
// deeper file's patterns come later — so the deepest .procoderignore decides,
// as in git. Only ancestors of the path are ever consulted, which is why a
// subdirectory's file cannot affect anything above it.
// `config.noIgnore` is the escape hatch behind `procoder check --no-ignore`:
// ignore files stop applying, and nothing else changes — [exclude] paths is the
// project-wide contract and stays in force, so the flag cannot be a back door
// into node_modules.
function ignoredBy(config, relPath) {
  if (!config.root || config.noIgnore || relPath.split('/').includes('..')) return null;
  const cut = relPath.lastIndexOf('/');
  let winner = null;
  for (const rule of ignoreRulesFor(config, cut === -1 ? '' : relPath.slice(0, cut))) {
    if (rule.re.test(rule.base ? relPath.slice(rule.base.length + 1) : relPath)) winner = rule;
  }
  return winner && !winner.negate ? `${winner.base ? `${winner.base}/` : ''}${IGNORE_FILE}` : null;
}

// Why a path is not checked, or null. `.procoder.toml` is consulted first and
// its verdict is final: the root config is the project-wide contract, and a
// `!` in some subdirectory must not be able to re-include what it excluded. A
// .procoderignore may only narrow further.
function excludeReason(config, relPath) {
  const normalized = String(relPath).replace(/\\/g, '/');
  if (config.exclude.paths.some((pattern) => matchesPattern(pattern, normalized))) return 'excluded';
  const file = ignoredBy(config, normalized);
  return file ? `ignored:${file}` : null;
}

function isExcluded(config, relPath) {
  return excludeReason(config, relPath) !== null;
}

// `rules = ["path/pattern:check/id"]` — the narrow form of exclusion. A path
// exclusion silences every rung at once and forever; this one silences a single
// named check in a single place, which is the only shape the doctrine allows.
// Entries without both halves are dropped rather than widened to a whole file,
// and so are directory ("hooks/") or glob ("**/*.js") paths: a mechanism whose
// justification is that it is narrow must not be quietly widenable to a tree.
function parseRuleExclusions(raw) {
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((entry) => typeof entry === 'string')
    .map((entry) => {
      // First colon, not last: check ids carry the external tool's own rule id
      // ("true/eslint:no-eval"), so an id may contain colons. A path with one
      // is not a real case.
      const split = entry.indexOf(':');
      if (split === -1) return { entry, path: '', id: '' };
      return { entry, path: entry.slice(0, split), id: entry.slice(split + 1) };
    })
    .filter((rule) => {
      const ok = rule.path && rule.id
        && !rule.path.endsWith('/') && !rule.path.includes('*');
      // Dropping silently is how a correct-looking exclusion ends up doing
      // nothing for a week. Say so; never throw — config must not crash a hook.
      if (!ok) process.stderr.write(`procoder: ignoring rule exclusion "${rule.entry}" — expected "path:rule-id" with an exact file path\n`);
      return ok;
    })
    .map((rule) => ({ path: rule.path, id: rule.id }));
}

function isRuleExcluded(config, relPath, id) {
  const normalized = String(relPath).replace(/\\/g, '/');
  return config.exclude.rules.some(
    (rule) => rule.id === id && rule.path === normalized);
}

module.exports = {
  DEFAULTS, loadConfig, isExcluded, excludeReason, isRuleExcluded, findRepoRoot, IGNORE_FILE,
};
