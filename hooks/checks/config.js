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
  try {
    raw = parseToml(fs.readFileSync(path.join(repoRoot, '.procoder.toml'), 'utf8'));
  } catch (e) {
    // No config, or unreadable. Defaults are the whole point of having them.
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
    const regex = new RegExp(
      '^' + pattern
        .replace(/[.+^${}()|[\]\\]/g, '\\$&')
        .replace(/\*\*\//g, '(?:.*/)?')
        .replace(/\*/g, '[^/]*') + '$');
    return regex.test(normalized);
  });
}

module.exports = { DEFAULTS, loadConfig, isExcluded, findRepoRoot };
