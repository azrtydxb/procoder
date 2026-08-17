#!/usr/bin/env node
// procoder — .procoder.toml loading, defaults, and path exclusion.

const fs = require('fs');
const path = require('path');
const { parseToml } = require('./toml');
const { LEVEL_RANK, normalizeLevel } = require('../procoder-config');

// The largest file the engine will open. A measured ceiling, not a preference:
// past it the engine either overflows the stack building the finding list or
// spends more than the 2s hook budget inside the one stage of a check that
// cannot be abandoned part-way, the language pack. run.js carries the
// derivation, the per-pack numbers behind it and — the part that matters — the
// slowdown factor it is stated to survive. It lives here because
// `[limits] max_file_bytes` lets a project clamp it, and a clamp belongs where
// the config is read.
const MAX_FILE_BYTES = 1024 * 1024;

const DEFAULTS = {
  exclude: { paths: ['node_modules/', 'vendor/', 'dist/', 'build/', '.git/'], rules: [] },
  limits: { max_file_bytes: MAX_FILE_BYTES },
  thresholds: { function_lines: 40, nesting_depth: 3, params: 4, complexity: 10 },
  // Rungs 1-2 are facts (it is injectable, or it is not); rungs 3-4 are
  // judgment. Only `pragmatic` acts on the difference — see procoder-check.js.
  rungs: { safe: 'error', true: 'error', obvious: 'warn', alone: 'warn' },
  levels: [],
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

// The 1-based line a key sits on, for a warning that has to name one. The TOML
// parser keeps no line numbers, and threading them through every value it
// returns would be a large change for one key. Falls back to line 1 rather than
// suppressing the warning: a warning with a slightly wrong line still reaches
// the author, a swallowed one does not.
function lineOf(text, re) {
  const at = String(text).split(/\r?\n/).findIndex((line) => re.test(line));
  return at === -1 ? 1 : at + 1;
}

const MAX_FILE_BYTES_LINE = /^\s*(?:limits\s*\.\s*)?max_file_bytes\s*=/;

// `[limits] max_file_bytes` clamps the size cap DOWNWARD ONLY.
//
// The built-in ceiling is a measurement, not a taste: above it the engine
// misses the hook budget or crashes, and either way the file is not checked. It
// is stated to hold on a host up to three times slower than the one that
// measured it, and this clamp is the knob for a project that needs more
// headroom than that — hardware nobody here has measured, a container with a
// CPU quota, a laptop on battery. So a smaller value is honoured, and a larger
// one is refused with a warning naming file and line.
// Config may always narrow what procoder trusts itself to do; it may never
// widen it past what measurement supports.
//
// Anything that is not a positive whole number of bytes is refused the same
// way. Zero and negatives especially: obeyed literally they would skip every
// file in the repo and report nothing wrong, turning one config line into a
// silent kill switch for the whole gate.
function fileBytesLimit(raw, text, file) {
  const value = raw.limits && raw.limits.max_file_bytes;
  if (value === undefined || value === null) return MAX_FILE_BYTES;
  const at = `${file}:${lineOf(text, MAX_FILE_BYTES_LINE)}`;
  if (typeof value !== 'number' || !Number.isFinite(value) || Math.floor(value) < 1) {
    process.stderr.write(`procoder: ${at}: max_file_bytes must be a positive number of bytes, ignored: ${JSON.stringify(value)}\n`);
    return MAX_FILE_BYTES;
  }
  if (value > MAX_FILE_BYTES) {
    process.stderr.write(`procoder: ${at}: max_file_bytes ${value} is above the measured ceiling ${MAX_FILE_BYTES} — using ${MAX_FILE_BYTES}. Files above it cannot be checked inside the hook budget.\n`);
    return MAX_FILE_BYTES;
  }
  return Math.floor(value);
}

function loadConfig(repoRoot) {
  let raw = {};
  let text = '';
  const file = path.join(repoRoot, '.procoder.toml');
  try {
    text = fs.readFileSync(file, 'utf8');
    raw = parseToml(text);
  } catch (e) {
    // Having no config is the normal case, and defaults are the whole point of
    // having them. Having one that cannot be read is not: silently falling back
    // would run every check under settings the author never chose, so say so.
    if (e.code !== 'ENOENT') {
      process.stderr.write(`procoder: cannot read ${file} (${e.message}) — using defaults\n`);
    }
  }

  // `configuredPaths` is the project's own list, kept apart from the built-in
  // defaults it is concatenated onto. Only the author's entries can go stale
  // (see unusedPathExclusions), and only theirs are worth naming back to them:
  // nobody needs telling that node_modules/ is excluded.
  const configured = Array.isArray(raw.exclude && raw.exclude.paths)
    ? raw.exclude.paths.filter((p) => typeof p === 'string')
    : [];

  return {
    root: repoRoot,
    exclude: {
      paths: DEFAULTS.exclude.paths.concat(configured),
      configuredPaths: configured,
      rules: parseRuleExclusions(raw.exclude && raw.exclude.rules),
    },
    limits: { max_file_bytes: fileBytesLimit(raw, text, '.procoder.toml') },
    levels: parseLevelPins(raw.levels),
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

// Which `[exclude] paths` pattern kept this file out, or null. `excludeReason`
// answers "was it excluded"; a caller telling the user why a file they named
// themselves produced no output has to be able to name the line to change.
// Separate rather than folded into the reason string, because that string is
// compared by value throughout the engine and by the baseline.
function excludingPattern(config, relPath) {
  const normalized = String(relPath).replace(/\\/g, '/');
  return config.exclude.paths.find((pattern) => matchesPattern(pattern, normalized)) || null;
}

// Path exclusions that can no longer be excluding anything. Three ways that
// happens, and the third is the expensive one:
//
//   gone     the file or directory the pattern names is not there any more —
//            `vendor/` after vendor/ was deleted, `src/generated/` after the
//            generator moved. Decidable from the path alone, with no scan.
//   empty    the pattern is there but matches no file in the tree — the shape
//            a glob rots into, since `**/*.gen.ts` survives the deletion of
//            every generated file without changing a character.
//   clean    it matches files, and not one of them has a finding. The
//            exclusion is holding nothing back today; the day it starts to,
//            nobody will be told.
//
// Left in force, each of the three excludes whatever lands at that path next,
// by a decision nobody made. Rule exclusions have been judged to this standard
// since they existed (`unusedRuleExclusions` in bin/procoder.js) and path
// exclusions were judged only on `gone`, which is the half that costs nothing.
//
// `empty` and `clean` cannot be answered from config alone: both are claims
// about the tree, and a run over one file cannot make them. So they are decided
// only when the caller passes an `audit` — `{ files, findings }`, the tree's
// repo-relative paths and a scan of one of them — and the CLI passes one only
// for a `verify` whose targets covered the whole repository. That is the same
// restraint rule exclusions get: an exclusion the run could not see is left
// alone rather than guessed at. Called with no audit (the hook, any direct API
// caller) this does exactly what it did before, at exactly the old cost.
//
// Only the project's own entries, never the built-in defaults.
// Why one pattern is doing nothing, or null if it is still earning its place.
function staleReason(config, pattern, audit) {
  if (!pattern.includes('*')
    && !fs.existsSync(path.join(config.root, pattern.replace(/\/+$/, '')))) {
    return 'the path no longer exists';
  }
  if (!audit) return null;
  const matched = audit.files.filter((rel) => matchesPattern(pattern, rel));
  if (matched.length === 0) return 'it matches no file in the tree';
  // `findings` returns null for a file it could not judge — one still excluded
  // by another pattern, ignored, too large, unreadable. Null is not 0, so one
  // unjudgeable file is enough to leave the exclusion alone: a scan that saw
  // less than the whole set cannot report the set clean.
  return matched.every((rel) => audit.findings(rel, pattern) === 0)
    ? `nothing it excludes has a finding (${matched.length} file${matched.length === 1 ? '' : 's'} scanned)`
    : null;
}

function unusedPathExclusions(config, audit) {
  // A hand-built config (a direct API caller, a test) may carry no
  // `configuredPaths` at all, and nothing here may throw into a hook.
  if (!config.root || !Array.isArray(config.exclude && config.exclude.configuredPaths)) return [];
  return config.exclude.configuredPaths
    .filter((pattern) => typeof pattern === 'string' && pattern !== '')
    .map((pattern) => ({ pattern, reason: staleReason(config, pattern, audit) }))
    .filter((entry) => entry.reason !== null);
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

// --- [levels] --------------------------------------------------------------
//
// `[levels] paranoid = ["src/auth/", "**/payments/*.ts"]` pins a level to the
// paths that earn it, so the gate follows the blast radius rather than whatever
// the session happens to be set to. Auth, payments and crypto want paranoid
// whoever is typing; a scripts/ directory is worth pragmatic even in a strict
// session — and nobody remembers to type either at the moment it matters.
//
// Patterns are the `[exclude] paths` shapes and nothing new: a directory prefix
// or a simple glob. Two pins covering one file resolve to the stricter of them.
// A path named twice is a config its author should fix, and until they do, the
// safer of their two answers is the one to obey.
//
// `off` is refused. It would silence a path outright, which `[exclude] paths`
// already does — and reports as a skip, which this would not. A second, quieter
// way to turn the gate off is exactly the twin rung 4 is about.
function pinsFor(name, patterns) {
  const level = normalizeLevel(name);
  if (!level || !LEVEL_RANK[level]) {
    process.stderr.write(`procoder: ignoring [levels] "${name}" — expected pragmatic, strict or `
      + `paranoid${level === 'off' ? '. "off" silences a path; [exclude] paths does that and says so' : ''}\n`);
    return [];
  }
  if (!Array.isArray(patterns)) {
    process.stderr.write(`procoder: ignoring [levels] ${level} — expected an array of path patterns\n`);
    return [];
  }
  return patterns
    .filter((pattern) => typeof pattern === 'string' && pattern !== '')
    .map((pattern) => ({ level, pattern }));
}

function parseLevelPins(raw) {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return [];
  return Object.entries(raw).flatMap(([name, patterns]) => pinsFor(name, patterns));
}

// The level this file is gated at: a matching pin, or the session's own level.
// A session that is `off` stays off — a pin narrows or widens a running gate,
// and must never restart one the user turned off.
function levelFor(config, relPath, sessionLevel) {
  const pins = config.levels;
  if (sessionLevel === 'off' || !Array.isArray(pins) || pins.length === 0) return sessionLevel;
  const normalized = String(relPath).replace(/\\/g, '/');
  let winner = null;
  for (const pin of pins) {
    if (!matchesPattern(pin.pattern, normalized)) continue;
    if (!winner || LEVEL_RANK[pin.level] > LEVEL_RANK[winner]) winner = pin.level;
  }
  return winner || sessionLevel;
}

function isRuleExcluded(config, relPath, id) {
  const normalized = String(relPath).replace(/\\/g, '/');
  return config.exclude.rules.some(
    (rule) => rule.id === id && rule.path === normalized);
}

module.exports = {
  DEFAULTS, MAX_FILE_BYTES, loadConfig, isExcluded, excludeReason, excludingPattern,
  unusedPathExclusions, isRuleExcluded, findRepoRoot, IGNORE_FILE, levelFor,
};
