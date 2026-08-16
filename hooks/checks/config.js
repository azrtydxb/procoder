#!/usr/bin/env node
// procoder — .procoder.toml loading, defaults, and path exclusion.

const fs = require('fs');
const path = require('path');
const { parseToml } = require('./toml');

const DEFAULTS = {
  level: 'strict',
  exclude: { paths: ['node_modules/', 'vendor/', 'dist/', 'build/', '.git/'] },
  thresholds: { function_lines: 40, nesting_depth: 3, params: 4, complexity: 10 },
  // true_ avoids the TOML boolean literal; it is rung 2, TRUE.
  rungs: { safe: 'error', true_: 'error', obvious: 'warn', alone: 'warn' },
  baseline: { file: '.procoder-baseline.json', enforce_no_growth: true },
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
    level: typeof raw.level === 'string' ? raw.level : DEFAULTS.level,
    exclude: {
      paths: Array.isArray(raw.exclude && raw.exclude.paths)
        ? DEFAULTS.exclude.paths.concat(raw.exclude.paths)
        : DEFAULTS.exclude.paths.slice(),
    },
    thresholds: mergeSection(DEFAULTS.thresholds, raw.thresholds),
    rungs: mergeSection(DEFAULTS.rungs, raw.rungs),
    baseline: mergeSection(DEFAULTS.baseline, raw.baseline),
  };
}

// Patterns are directory prefixes ("vendor/") or simple globs ("**/*.gen.ts").
// Deliberately not a full glob engine — the config only needs these two shapes.
function isExcluded(config, relPath) {
  const normalized = String(relPath).replace(/\\/g, '/');
  return config.exclude.paths.some((pattern) => {
    if (pattern.endsWith('/')) {
      return normalized === pattern.slice(0, -1) || normalized.startsWith(pattern);
    }
    // `?` is not a documented glob wildcard here (only `*` and `**` are), so
    // it must be escaped like the other regex metacharacters, else patterns
    // such as "?abc.ts" build an invalid regex ("nothing to repeat").
    try {
      const regex = new RegExp(
        '^' + pattern
          .replace(/[.+^${}()|[\]\\?]/g, '\\$&')
          .replace(/\*\*\//g, '(?:.*/)?')
          .replace(/\*/g, '[^/]*') + '$');
      return regex.test(normalized);
    } catch (e) {
      // Belt-and-braces: any pattern shape we didn't anticipate must never
      // throw into the hook. An exclude that fails to match is far better
      // than a crashed session, so fall back to a plain literal comparison.
      return normalized === pattern;
    }
  });
}

module.exports = { DEFAULTS, loadConfig, isExcluded, findRepoRoot };
