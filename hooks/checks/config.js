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

function isExcluded(config, relPath) {
  const normalized = String(relPath).replace(/\\/g, '/');
  return config.exclude.paths.some((pattern) => matchesPattern(pattern, normalized));
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

module.exports = { DEFAULTS, loadConfig, isExcluded, isRuleExcluded, findRepoRoot };
