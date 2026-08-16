# procoder Plan 2 — Check Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deterministic checks that run on every file Claude writes — reusing the project's own linters when present, falling back to built-in packs when not — with a ratchet baseline so legacy repos stay usable.

**Architecture:** `procoder-check.js` (PostToolUse) loads `.procoder.toml`, resolves whether the project has real tooling for the file's language, runs that tool or the built-in pack, always runs the universal pack on top, suppresses anything already in `.procoder-baseline.json`, and emits at most five findings as `additionalContext`.

**Tech Stack:** Node.js ≥18, CommonJS, zero runtime dependencies. Tests via `node --test` with per-language fixture pairs.

**Spec:** [docs/superpowers/specs/2026-08-16-procoder-design.md](../specs/2026-08-16-procoder-design.md)
**Depends on:** Plan 1 (`hooks/procoder-config.js`, `hooks/procoder-runtime.js`, `hooks/claude-hooks.json` already declares the PostToolUse entry).

## Global Constraints

- Node.js ≥18, CommonJS, **zero runtime dependencies** — including the TOML parser, which is written here as a documented subset.
- Total hook budget **2 seconds**. Any external tool invocation gets a 1500 ms timeout and is abandoned on overrun; the hook emits whatever it has.
- Findings are capped at **5 per file**, ordered by rung (SAFE, TRUE, OBVIOUS, ALONE) then by line.
- The hook **never blocks**. It emits `additionalContext` only. There is no deny path.
- Rung severities default to: SAFE `error`, TRUE `error`, OBVIOUS `warn`, ALONE `warn`.
- Every check has a stable `id` of the form `<rung>/<slug>` (e.g. `safe/hardcoded-secret`). Ids are permanent — baselines reference them.
- Baseline fingerprints are `sha1(id + '\0' + relPath + '\0' + normalizedLine)`. **Never** line numbers — a reformat must not resurrect suppressed findings.
- `PROCODER_NO_HOOK=1` disables the hook entirely.
- No check may read a file outside the repo root, and no check may execute a command string built from file contents.

---

## File Structure

| File | Responsibility |
|---|---|
| `hooks/checks/toml.js` | Minimal TOML subset parser (tables, strings, ints, bools, string arrays) |
| `hooks/checks/config.js` | Loads and defaults `.procoder.toml`; path exclusion matching |
| `hooks/checks/finding.js` | Finding shape, ordering, capping, and the one-line output format |
| `hooks/checks/universal.js` | Language-independent pack: secrets, PII in logs, orphan TODOs, commented-out code, deprecation without removal trigger |
| `hooks/checks/shape.js` | Brace/indent shape metrics shared by the language packs |
| `hooks/checks/lang/ts.js` | TypeScript/JavaScript pack |
| `hooks/checks/lang/py.js` | Python pack |
| `hooks/checks/lang/go.js` | Go pack |
| `hooks/checks/lang/rust.js` | Rust pack |
| `hooks/checks/lang/jvm.js` | Java/Kotlin pack |
| `hooks/checks/lang/dotnet.js` | C# pack |
| `hooks/checks/registry.js` | Extension → pack, and pack → preferred external tool |
| `hooks/checks/resolve.js` | Detects configured project tooling; runs it; parses its output |
| `hooks/checks/baseline.js` | Ratchet: fingerprint, record, suppress, growth check |
| `hooks/checks/run.js` | Orchestrates: config → packs → tool → baseline → cap |
| `hooks/procoder-check.js` | PostToolUse entry point |
| `bin/procoder.js` | CLI for baseline/CI use (`procoder check|baseline`) |
| `tests/fixtures/<lang>/{dirty,clean}.<ext>` | Fixture pairs |

---

## Task 1: Minimal TOML parser

**Files:**
- Create: `hooks/checks/toml.js`
- Test: `tests/toml.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces: `parseToml(text) → object`. Supports `[table]`, `[a.b]`, `key = "string"`, integers, floats, booleans, and arrays of strings. Comments with `#`. Anything else is ignored rather than throwing.

- [ ] **Step 1: Write the failing test**

```js
// tests/toml.test.js
const test = require('node:test');
const assert = require('node:assert');
const { parseToml } = require('../hooks/checks/toml');

test('parses scalars at the root', () => {
  assert.deepStrictEqual(
    parseToml('level = "strict"\nenabled = true\nmax = 10\n'),
    { level: 'strict', enabled: true, max: 10 });
});

test('parses tables and dotted tables', () => {
  const out = parseToml('[thresholds]\nfunction_lines = 40\n\n[a.b]\nx = "y"\n');
  assert.strictEqual(out.thresholds.function_lines, 40);
  assert.strictEqual(out.a.b.x, 'y');
});

test('parses single-line string arrays', () => {
  const out = parseToml('[exclude]\npaths = ["vendor/", "dist/"]\n');
  assert.deepStrictEqual(out.exclude.paths, ['vendor/', 'dist/']);
});

test('ignores comments and blank lines', () => {
  const out = parseToml('# a comment\n\nlevel = "strict" # trailing\n');
  assert.strictEqual(out.level, 'strict');
});

test('malformed input yields an object, never a throw', () => {
  assert.doesNotThrow(() => parseToml('[[[garbage\nnot a pair\n= 5\n'));
  assert.deepStrictEqual(parseToml('total nonsense'), {});
});

test('a # inside a quoted string is not a comment', () => {
  assert.strictEqual(parseToml('token = "abc#def"\n').token, 'abc#def');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/toml.test.js`
Expected: FAIL — `Cannot find module '../hooks/checks/toml'`.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — TOML subset parser.
//
// Supports exactly what .procoder.toml needs: [tables], [dotted.tables],
// key = "string" | int | float | true/false, and single-line arrays of strings.
// Everything else is skipped rather than raising, because a malformed config
// must degrade to defaults, never break a session.
//
// procoder: subset parser, swap for a real TOML library if the config grows
// multi-line arrays, dates, or inline tables.

function parseValue(raw) {
  const text = raw.trim();
  if (text.startsWith('"') && text.endsWith('"') && text.length >= 2) {
    return text.slice(1, -1);
  }
  if (text.startsWith("'") && text.endsWith("'") && text.length >= 2) {
    return text.slice(1, -1);
  }
  if (text === 'true') return true;
  if (text === 'false') return false;
  if (text.startsWith('[') && text.endsWith(']')) {
    const inner = text.slice(1, -1).trim();
    if (!inner) return [];
    return inner.split(',')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => parseValue(item));
  }
  if (/^-?\d+$/.test(text)) return parseInt(text, 10);
  if (/^-?\d*\.\d+$/.test(text)) return parseFloat(text);
  return text;
}

// Strips a trailing comment, respecting quotes so "abc#def" survives.
function stripComment(line) {
  let inSingle = false;
  let inDouble = false;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === '"' && !inSingle) inDouble = !inDouble;
    else if (ch === "'" && !inDouble) inSingle = !inSingle;
    else if (ch === '#' && !inSingle && !inDouble) return line.slice(0, i);
  }
  return line;
}

function parseToml(text) {
  const result = {};
  let table = result;

  for (const rawLine of String(text || '').split(/\r?\n/)) {
    const line = stripComment(rawLine).trim();
    if (!line) continue;

    const tableMatch = /^\[([A-Za-z0-9_.\-]+)\]$/.exec(line);
    if (tableMatch) {
      table = result;
      for (const part of tableMatch[1].split('.')) {
        if (typeof table[part] !== 'object' || table[part] === null) table[part] = {};
        table = table[part];
      }
      continue;
    }

    const pairMatch = /^([A-Za-z0-9_\-]+)\s*=\s*(.+)$/.exec(line);
    if (pairMatch) {
      table[pairMatch[1]] = parseValue(pairMatch[2]);
    }
    // Anything else is silently skipped: defaults beat a crash.
  }

  return result;
}

module.exports = { parseToml };
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/toml.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/toml.js tests/toml.test.js
git commit -m "feat: TOML subset parser for .procoder.toml"
```

---

## Task 2: Config loader

**Files:**
- Create: `hooks/checks/config.js`, `.procoder.toml`
- Test: `tests/checks-config.test.js`

**Interfaces:**
- Consumes: `toml.parseToml`.
- Produces:
  - `loadConfig(repoRoot) → Config`
  - `Config = { level, exclude: {paths: string[]}, thresholds: {function_lines, nesting_depth, params, complexity}, rungs: {safe,true_,obvious,alone}, baseline: {file, enforce_no_growth} }`
  - `isExcluded(config, relPath) → boolean`
  - `findRepoRoot(startDir) → string`
  - `DEFAULTS` object

- [ ] **Step 1: Write the failing test**

```js
// tests/checks-config.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadConfig, isExcluded, DEFAULTS, findRepoRoot } = require('../hooks/checks/config');

function tempRepo(files = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cfg-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('absent config yields the documented defaults', () => {
  const cfg = loadConfig(tempRepo());
  assert.strictEqual(cfg.level, DEFAULTS.level);
  assert.strictEqual(cfg.thresholds.function_lines, 40);
  assert.strictEqual(cfg.thresholds.nesting_depth, 3);
  assert.strictEqual(cfg.thresholds.params, 4);
  assert.strictEqual(cfg.thresholds.complexity, 10);
  assert.strictEqual(cfg.rungs.safe, 'error');
  assert.strictEqual(cfg.rungs.obvious, 'warn');
});

test('config values override defaults, unset keys keep them', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[thresholds]\nfunction_lines = 80\n\n[rungs]\nobvious = "error"\n',
  }));
  assert.strictEqual(cfg.thresholds.function_lines, 80);
  assert.strictEqual(cfg.thresholds.nesting_depth, 3);
  assert.strictEqual(cfg.rungs.obvious, 'error');
});

test('malformed config falls back to defaults without throwing', () => {
  const cfg = loadConfig(tempRepo({ '.procoder.toml': '[[[not toml' }));
  assert.strictEqual(cfg.thresholds.function_lines, 40);
});

test('isExcluded matches directory prefixes and glob suffixes', () => {
  const cfg = loadConfig(tempRepo({
    '.procoder.toml': '[exclude]\npaths = ["vendor/", "**/*.generated.ts", "migrations/"]\n',
  }));
  assert.ok(isExcluded(cfg, 'vendor/lib/x.js'));
  assert.ok(isExcluded(cfg, 'src/api.generated.ts'));
  assert.ok(isExcluded(cfg, 'migrations/001_init.sql'));
  assert.ok(!isExcluded(cfg, 'src/api.ts'));
  assert.ok(!isExcluded(cfg, 'src/vendorish.ts'));
});

test('findRepoRoot walks up to the .git directory', () => {
  const dir = tempRepo({ 'a/b/c.txt': 'x' });
  assert.strictEqual(findRepoRoot(path.join(dir, 'a', 'b')), dir);
});

test('findRepoRoot returns the start dir when no .git exists', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-nogit-'));
  assert.strictEqual(findRepoRoot(dir), dir);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/checks-config.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
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
```

Also write the repo's own `.procoder.toml` — procoder must be governed by procoder:

```toml
level = "strict"

[exclude]
paths = ["tests/fixtures/", "AGENTS.md", ".cursor/", ".windsurf/", ".clinerules/", ".kiro/", ".qoder/", ".agents/", ".opencode/", ".openclaw/"]

[baseline]
enforce_no_growth = true
```

Fixtures are excluded because they contain deliberate violations; the generated
rule directories are excluded because they are generated.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/checks-config.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/config.js .procoder.toml tests/checks-config.test.js
git commit -m "feat: .procoder.toml config loading and path exclusion"
```

---

## Task 3: Finding model and output format

**Files:**
- Create: `hooks/checks/finding.js`
- Test: `tests/finding.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `finding({rung, id, line, message, fix}) → Finding`
  - `RUNGS = ['SAFE','TRUE','OBVIOUS','ALONE']`
  - `rungIndex(rung) → number`
  - `sortFindings(findings) → Finding[]` (rung order, then line)
  - `capFindings(findings, max) → Finding[]`
  - `formatFindings(findings, relPath) → string`

  A `Finding` is `{ rung, id, line, message, fix }`. Every pack in this plan returns arrays of these.

- [ ] **Step 1: Write the failing test**

```js
// tests/finding.test.js
const test = require('node:test');
const assert = require('node:assert');
const { finding, sortFindings, capFindings, formatFindings, RUNGS, rungIndex } =
  require('../hooks/checks/finding');

const f = (rung, line, id = 'x/y') =>
  finding({ rung, id, line, message: 'msg', fix: 'do the thing' });

test('sorts by rung order first, then by line', () => {
  const sorted = sortFindings([f('ALONE', 1), f('SAFE', 9), f('OBVIOUS', 2), f('SAFE', 3)]);
  assert.deepStrictEqual(
    sorted.map((x) => [x.rung, x.line]),
    [['SAFE', 3], ['SAFE', 9], ['OBVIOUS', 2], ['ALONE', 1]]);
});

test('rungIndex follows the doctrine order', () => {
  assert.deepStrictEqual(RUNGS, ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE']);
  assert.ok(rungIndex('SAFE') < rungIndex('TRUE'));
  assert.ok(rungIndex('OBVIOUS') < rungIndex('ALONE'));
});

test('caps to the limit after sorting, keeping the most severe', () => {
  const capped = capFindings(sortFindings([
    f('ALONE', 1), f('SAFE', 2), f('OBVIOUS', 3), f('TRUE', 4), f('ALONE', 5), f('SAFE', 6),
  ]), 3);
  assert.strictEqual(capped.length, 3);
  assert.deepStrictEqual(capped.map((x) => x.rung), ['SAFE', 'SAFE', 'TRUE']);
});

test('formats one line per finding in the doctrine shape', () => {
  const out = formatFindings([
    finding({ rung: 'SAFE', id: 'safe/raw-input', line: 42, message: 'raw req.body.role into authz check', fix: 'validate + server-side role lookup' }),
  ], 'api/users.ts');
  assert.strictEqual(
    out.trim(),
    '[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup');
});

test('formatting an empty list yields an empty string', () => {
  assert.strictEqual(formatFindings([], 'x.ts'), '');
});

test('finding rejects an unknown rung', () => {
  assert.throws(() => finding({ rung: 'FAST', id: 'a/b', line: 1, message: 'm', fix: 'f' }));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/finding.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — the finding shape and its one-line output format.

const RUNGS = ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE'];

function rungIndex(rung) {
  return RUNGS.indexOf(rung);
}

function finding({ rung, id, line, message, fix }) {
  if (!RUNGS.includes(rung)) throw new Error(`unknown rung: ${rung}`);
  return { rung, id, line: Number(line) || 0, message: String(message), fix: String(fix) };
}

function sortFindings(findings) {
  return findings.slice().sort((a, b) =>
    rungIndex(a.rung) - rungIndex(b.rung) || a.line - b.line);
}

function capFindings(findings, max) {
  return findings.slice(0, max);
}

// [1 SAFE]    api/users.ts:42   what is wrong → what to do
// Rationale belongs in the fix clause, never on its own line.
function formatFindings(findings, relPath) {
  return findings.map((f) => {
    const label = `[${rungIndex(f.rung) + 1} ${f.rung}]`.padEnd(11);
    const location = `${relPath}:${f.line}`.padEnd(17);
    return `${label} ${location} ${f.message} → ${f.fix}`;
  }).join('\n');
}

module.exports = { RUNGS, rungIndex, finding, sortFindings, capFindings, formatFindings };
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/finding.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/finding.js tests/finding.test.js
git commit -m "feat: finding model and output format"
```

---

## Task 4: Universal pattern pack

**Files:**
- Create: `hooks/checks/universal.js`
- Test: `tests/universal.test.js`, `tests/fixtures/universal/{dirty,clean}.txt`

**Interfaces:**
- Consumes: `finding`.
- Produces: `checkUniversal(source, {relPath, config}) → Finding[]`. Runs on every file regardless of language — these are precisely the checks linters do not perform.

Checks implemented, with permanent ids:

| id | Rung | Detects |
|---|---|---|
| `safe/hardcoded-secret` | SAFE | AWS keys, GitHub/Slack/Stripe tokens, private key headers, `password=`/`api_key=` with a literal |
| `safe/secret-in-log` | SAFE | token/password/authorization/cookie interpolated into a log call |
| `safe/pii-in-log` | SAFE | email/ssn/phone/address fields interpolated into a log call |
| `alone/commented-code` | ALONE | ≥3 consecutive comment lines that parse as code |
| `alone/orphan-todo` | ALONE | TODO/FIXME/HACK with no owner or ticket reference |
| `alone/deprecated-no-trigger` | ALONE | a deprecation marker with no removal date, version, or condition |

- [ ] **Step 1: Write the failing test**

```js
// tests/universal.test.js
const test = require('node:test');
const assert = require('node:assert');
const { checkUniversal } = require('../hooks/checks/universal');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const run = (src) => checkUniversal(src, { relPath: 'x.js', config });
const ids = (src) => run(src).map((f) => f.id);

test('flags hardcoded secrets of several shapes', () => {
  assert.ok(ids('const k = "AKIAIOSFODNN7EXAMPLE";').includes('safe/hardcoded-secret'));
  assert.ok(ids('token = "ghp_aBcD1234567890aBcD1234567890aBcD12"').includes('safe/hardcoded-secret'));
  assert.ok(ids('-----BEGIN RSA PRIVATE KEY-----').includes('safe/hardcoded-secret'));
  assert.ok(ids('const password = "hunter2correcthorse";').includes('safe/hardcoded-secret'));
});

test('does not flag secrets read from the environment or a manager', () => {
  assert.deepStrictEqual(ids('const key = process.env.API_KEY;'), []);
  assert.deepStrictEqual(ids('password = os.environ["DB_PASSWORD"]'), []);
  assert.deepStrictEqual(ids('const token = await secrets.get("stripe");'), []);
  assert.deepStrictEqual(ids('password = "" # set at startup'), []);
});

test('flags secrets and PII reaching log calls', () => {
  assert.ok(ids('logger.info(`auth ${token}`)').includes('safe/secret-in-log'));
  assert.ok(ids('console.log("authorization: " + req.headers.authorization)').includes('safe/secret-in-log'));
  assert.ok(ids('log.debug(f"user email {user.email}")').includes('safe/pii-in-log'));
  assert.deepStrictEqual(ids('logger.info(`user ${user.id} logged in`)'), []);
});

test('flags a block of commented-out code but not prose comments', () => {
  const commented = [
    '// const x = compute(a, b);',
    '// if (x > 3) {',
    '//   send(x);',
    '// }',
  ].join('\n');
  assert.ok(ids(commented).includes('alone/commented-code'));

  const prose = [
    '// This exists because the upstream API returns 200 on failure.',
    '// See INFRA-4821 for the vendor ticket.',
    '// Remove once they ship the fix.',
  ].join('\n');
  assert.ok(!ids(prose).includes('alone/commented-code'));
});

test('flags TODOs without an owner or ticket', () => {
  assert.ok(ids('// TODO: fix this later').includes('alone/orphan-todo'));
  assert.ok(!ids('// TODO(pascal): drop the shim').includes('alone/orphan-todo'));
  assert.ok(!ids('// TODO INFRA-4821: drop the shim').includes('alone/orphan-todo'));
});

test('flags deprecations with no removal trigger', () => {
  assert.ok(ids('// @deprecated use createUser instead').includes('alone/deprecated-no-trigger'));
  assert.ok(!ids('// @deprecated remove after v3.0').includes('alone/deprecated-no-trigger'));
  assert.ok(!ids('// procoder: remove after the 2026-10 migration').includes('alone/deprecated-no-trigger'));
});

test('the clean fixture produces no findings and the dirty one produces several', () => {
  const fs = require('fs');
  const path = require('path');
  const dir = path.join(__dirname, 'fixtures', 'universal');
  assert.strictEqual(
    checkUniversal(fs.readFileSync(path.join(dir, 'clean.txt'), 'utf8'),
      { relPath: 'clean.txt', config }).length, 0);
  assert.ok(
    checkUniversal(fs.readFileSync(path.join(dir, 'dirty.txt'), 'utf8'),
      { relPath: 'dirty.txt', config }).length >= 5);
});

test('every finding carries a line number and a fix', () => {
  for (const f of run('const k = "AKIAIOSFODNN7EXAMPLE";\n// TODO: later\n')) {
    assert.ok(f.line > 0, `${f.id} has no line`);
    assert.ok(f.fix.length > 0, `${f.id} has no fix`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/universal.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation and fixtures**

```js
#!/usr/bin/env node
// procoder — the language-independent pack.
//
// These are the checks linters do not run: credentials in source, secrets and
// PII reaching logs, code left commented out, and deprecations with no removal
// trigger. They apply to every file type, including config and docs.

const { finding } = require('./finding');

const SECRET_PATTERNS = [
  { re: /\bAKIA[0-9A-Z]{16}\b/, what: 'AWS access key id' },
  { re: /\bgh[pousr]_[A-Za-z0-9]{30,}\b/, what: 'GitHub token' },
  { re: /\bxox[baprs]-[A-Za-z0-9-]{10,}\b/, what: 'Slack token' },
  { re: /\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{10,}\b/, what: 'Stripe key' },
  { re: /-----BEGIN (?:RSA |EC |OPENSSH |PGP )?PRIVATE KEY-----/, what: 'private key' },
  { re: /\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\./, what: 'JWT' },
];

// A literal assigned to a credential-shaped name. Values that are empty, obvious
// placeholders, or reads from env/secret managers are not credentials.
const CREDENTIAL_ASSIGN =
  /\b(?:password|passwd|secret|api[_-]?key|apikey|access[_-]?token|auth[_-]?token|client[_-]?secret|private[_-]?key)\b\s*[:=]\s*["'`]([^"'`]{8,})["'`]/i;

const PLACEHOLDER = /^(?:x{3,}|\.{3,}|<[^>]+>|\{\{.*\}\}|\$\{.*\}|changeme|placeholder|example|test|dummy|redacted|your[_-]?\w+)$/i;

const LOG_CALL = /\b(?:console\.(?:log|info|warn|error|debug)|logger?\.(?:log|info|warn|error|debug|trace)|log\.(?:info|warn|error|debug|trace)|print|println|printf|fmt\.Print\w*|System\.out\.print\w*)\s*\(/i;

const SECRET_WORD = /\b(?:token|password|passwd|secret|api[_-]?key|authorization|auth[_-]?header|cookie|session[_-]?id|credential|private[_-]?key)\b/i;
const PII_WORD = /\b(?:email|e-mail|ssn|social[_-]?security|phone[_-]?number|date[_-]?of[_-]?birth|dob|home[_-]?address|street[_-]?address|passport|credit[_-]?card|card[_-]?number|iban)\b/i;

// Interpolation or concatenation of a variable into the logged string — a bare
// literal mentioning the word is fine ("password reset requested").
const INTERPOLATED = /\$\{[^}]*\}|%[sdv]|\{\}|\{[a-z_][\w.]*\}|["'`]\s*[+,]\s*\w|\bf["']/i;

const COMMENT_LINE = /^\s*(?:\/\/|#|--|\*(?!\/))\s?(.*)$/;
// A commented line is CODE, not prose, when it ends in a code terminator or
// contains an assignment/call/brace — prose sentences do not.
const LOOKS_LIKE_CODE =
  /[;{}]\s*$|^\s*(?:if|for|while|return|const|let|var|def|func|fn|class|import|from|public|private)\b|=\s*[^=]|\w+\([^)]*\)\s*[;{]?\s*$/;

const TODO = /\b(TODO|FIXME|HACK|XXX)\b(?!\s*[(:]?\s*(?:[A-Z]{2,}-\d+|\([^)]+\)))/;
const TODO_OWNED = /\b(?:TODO|FIXME|HACK|XXX)\b\s*(?:\([^)]+\)|[:\s]*[A-Z]{2,}-\d+)/;

const DEPRECATED = /@?\bdeprecated\b|\bDeprecated\s*\(|#\[deprecated/i;
const REMOVAL_TRIGGER =
  /\b(?:remove|delete|drop|sunset)\b[^.\n]{0,40}\b(?:after|by|in|once|when)\b|\bv?\d+\.\d+\b|\b20\d\d-\d\d(?:-\d\d)?\b/i;

function checkUniversal(source, { relPath, config } = {}) {
  const findings = [];
  const lines = String(source || '').split(/\r?\n/);

  let commentRun = 0;
  let commentRunStart = 0;
  let codeCommentsInRun = 0;

  lines.forEach((line, index) => {
    const lineNo = index + 1;

    for (const { re, what } of SECRET_PATTERNS) {
      if (re.test(line)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/hardcoded-secret', line: lineNo,
          message: `${what} literal in source`,
          fix: 'read from env or a secret manager, and rotate this value — it is in git history',
        }));
        break;
      }
    }

    const credential = CREDENTIAL_ASSIGN.exec(line);
    if (credential && !PLACEHOLDER.test(credential[1]) && !/process\.env|os\.environ|getenv|secrets?\./i.test(line)) {
      findings.push(finding({
        rung: 'SAFE', id: 'safe/hardcoded-secret', line: lineNo,
        message: 'credential assigned a literal value',
        fix: 'read from env or a secret manager; fail loudly at startup if absent',
      }));
    }

    if (LOG_CALL.test(line) && INTERPOLATED.test(line)) {
      if (SECRET_WORD.test(line)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/secret-in-log', line: lineNo,
          message: 'credential interpolated into a log call',
          fix: 'log a correlation id instead; never the credential',
        }));
      } else if (PII_WORD.test(line)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/pii-in-log', line: lineNo,
          message: 'PII interpolated into a log call',
          fix: 'redact or hash the field, or log the record id only',
        }));
      }
    }

    const comment = COMMENT_LINE.exec(line);
    if (comment) {
      if (commentRun === 0) commentRunStart = lineNo;
      commentRun += 1;
      if (LOOKS_LIKE_CODE.test(comment[1])) codeCommentsInRun += 1;
    } else {
      if (commentRun >= 3 && codeCommentsInRun >= 2) {
        findings.push(finding({
          rung: 'ALONE', id: 'alone/commented-code', line: commentRunStart,
          message: `${commentRun} lines of commented-out code`,
          fix: 'delete it — version control remembers',
        }));
      }
      commentRun = 0;
      codeCommentsInRun = 0;
    }

    if (TODO.test(line) && !TODO_OWNED.test(line)) {
      findings.push(finding({
        rung: 'ALONE', id: 'alone/orphan-todo', line: lineNo,
        message: 'TODO with no owner or ticket',
        fix: 'add TODO(owner) or a ticket id, or do it now',
      }));
    }

    if (DEPRECATED.test(line) && !REMOVAL_TRIGGER.test(line)) {
      findings.push(finding({
        rung: 'ALONE', id: 'alone/deprecated-no-trigger', line: lineNo,
        message: 'deprecation with no removal trigger',
        fix: 'add "remove after <version|date|condition>", or delete the old path now',
      }));
    }
  });

  // A comment block running to end-of-file still counts.
  if (commentRun >= 3 && codeCommentsInRun >= 2) {
    findings.push(finding({
      rung: 'ALONE', id: 'alone/commented-code', line: commentRunStart,
      message: `${commentRun} lines of commented-out code`,
      fix: 'delete it — version control remembers',
    }));
  }

  return findings;
}

module.exports = { checkUniversal };
```

Write `tests/fixtures/universal/dirty.txt` containing at least one instance of
each of the six ids, and `clean.txt` containing the near-miss variants that must
NOT fire: `process.env` reads, an owned `TODO(pascal)`, a deprecation with
`remove after v3.0`, a prose comment block of four lines, a log line
interpolating `user.id`, and the literal word `password` inside a message string.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/universal.test.js`
Expected: PASS (8 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/universal.js tests/universal.test.js tests/fixtures/universal
git commit -m "feat: universal pattern pack — secrets, PII in logs, rot"
```

---

## Task 5: Shared shape metrics

**Files:**
- Create: `hooks/checks/shape.js`
- Test: `tests/shape.test.js`

**Interfaces:**
- Consumes: `finding`.
- Produces:
  - `analyzeBraces(source) → {maxDepth, blocks: [{startLine, endLine, length}]}` for brace languages
  - `analyzeIndent(source, {tabWidth}) → {maxDepth, blocks}` for Python
  - `countParams(signatureText) → number`
  - `estimateComplexity(bodyText) → number` (1 + branch/loop/catch/logical-operator count)
  - `shapeFindings({blocks, maxDepth, thresholds, kind}) → Finding[]`

  Every language pack calls these rather than re-deriving thresholds, so a single
  `.procoder.toml` threshold change applies to all six.

- [ ] **Step 1: Write the failing test**

```js
// tests/shape.test.js
const test = require('node:test');
const assert = require('node:assert');
const {
  analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
} = require('../hooks/checks/shape');
const { DEFAULTS } = require('../hooks/checks/config');

test('analyzeBraces reports nesting depth and block spans', () => {
  const src = [
    'function a() {',      // 1
    '  if (x) {',          // 2
    '    while (y) {',     // 3
    '      go();',
    '    }',
    '  }',
    '}',
  ].join('\n');
  const out = analyzeBraces(src);
  assert.strictEqual(out.maxDepth, 3);
  assert.ok(out.blocks.some((b) => b.startLine === 1 && b.endLine === 7));
});

test('analyzeBraces ignores braces inside strings and comments', () => {
  const src = 'const s = "{{{";\n// }}}\nfunction a() {\n  go();\n}\n';
  assert.strictEqual(analyzeBraces(src).maxDepth, 1);
});

test('analyzeIndent reports depth for indentation languages', () => {
  const src = [
    'def a():',
    '    if x:',
    '        while y:',
    '            go()',
  ].join('\n');
  assert.strictEqual(analyzeIndent(src, { tabWidth: 4 }).maxDepth, 3);
});

test('countParams handles defaults, generics, and destructuring', () => {
  assert.strictEqual(countParams('(a, b, c)'), 3);
  assert.strictEqual(countParams('()'), 0);
  assert.strictEqual(countParams('(a: Map<string, number>, b)'), 2);
  assert.strictEqual(countParams('({ a, b }, c = [1, 2])'), 2);
});

test('estimateComplexity counts branches, loops and logical operators', () => {
  assert.strictEqual(estimateComplexity('return 1;'), 1);
  assert.strictEqual(estimateComplexity('if (a) {} else if (b) {}'), 3);
  assert.strictEqual(estimateComplexity('if (a && b || c) {}'), 4);
  assert.ok(estimateComplexity('for (;;) { if (a) { while (b) {} } }') >= 4);
});

test('shapeFindings fires only above the configured thresholds', () => {
  const thresholds = DEFAULTS.thresholds;
  const none = shapeFindings({
    blocks: [{ startLine: 1, endLine: 10, length: 10, params: 2, complexity: 3 }],
    maxDepth: 2, thresholds, kind: 'function',
  });
  assert.deepStrictEqual(none, []);

  const ids = shapeFindings({
    blocks: [{ startLine: 1, endLine: 95, length: 95, params: 7, complexity: 22 }],
    maxDepth: 6, thresholds, kind: 'function',
  }).map((f) => f.id);
  assert.ok(ids.includes('obvious/function-too-long'));
  assert.ok(ids.includes('obvious/too-many-params'));
  assert.ok(ids.includes('obvious/complexity'));
  assert.ok(ids.includes('obvious/nesting-depth'));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/shape.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — shape metrics shared by every language pack.
//
// Not a parser. A brace/indent counter that is right often enough to flag a
// 90-line function, and cheap enough to run inside a 2s hook budget.
//
// procoder: heuristic scanner, replace with a real parser per language if
// false positives on shape checks become the top complaint.

const { finding } = require('./finding');

// Removes string and comment content so their braces do not count. Replaces
// rather than deletes, so line numbers survive.
function stripNoise(source) {
  return String(source || '')
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p) => p + m.slice(p.length).replace(/./g, ' '))
    .replace(/#[^\n]*/g, (m) => m.replace(/./g, ' '))
    .replace(/"(?:[^"\\\n]|\\.)*"/g, (m) => '"' + ' '.repeat(Math.max(0, m.length - 2)) + '"')
    .replace(/'(?:[^'\\\n]|\\.)*'/g, (m) => "'" + ' '.repeat(Math.max(0, m.length - 2)) + "'")
    .replace(/`(?:[^`\\]|\\.)*`/g, (m) => m.replace(/[^\n]/g, ' '));
}

function analyzeBraces(source) {
  const lines = stripNoise(source).split(/\r?\n/);
  const stack = [];
  const blocks = [];
  let depth = 0;
  let maxDepth = 0;

  lines.forEach((line, index) => {
    for (const ch of line) {
      if (ch === '{') {
        depth += 1;
        maxDepth = Math.max(maxDepth, depth);
        stack.push(index + 1);
      } else if (ch === '}') {
        const startLine = stack.pop();
        depth = Math.max(0, depth - 1);
        if (startLine !== undefined) {
          blocks.push({ startLine, endLine: index + 1, length: index + 1 - startLine + 1 });
        }
      }
    }
  });

  return { maxDepth, blocks };
}

function analyzeIndent(source, { tabWidth = 4 } = {}) {
  const lines = stripNoise(source).split(/\r?\n/);
  const blocks = [];
  let maxDepth = 0;
  let openBlock = null;

  lines.forEach((line, index) => {
    if (!line.trim()) return;
    const leading = /^[ \t]*/.exec(line)[0];
    const width = leading.replace(/\t/g, ' '.repeat(tabWidth)).length;
    const depth = Math.floor(width / tabWidth);
    maxDepth = Math.max(maxDepth, depth);

    if (/^\s*(?:def|class|async def)\s/.test(line)) {
      if (openBlock) {
        blocks.push({ ...openBlock, endLine: index, length: index - openBlock.startLine + 1 });
      }
      openBlock = { startLine: index + 1, baseDepth: depth };
    } else if (openBlock && depth <= openBlock.baseDepth) {
      blocks.push({ ...openBlock, endLine: index, length: index - openBlock.startLine + 1 });
      openBlock = null;
    }
  });

  if (openBlock) {
    blocks.push({ ...openBlock, endLine: lines.length, length: lines.length - openBlock.startLine + 1 });
  }

  return { maxDepth, blocks };
}

function countParams(signatureText) {
  const inner = String(signatureText || '').replace(/^\s*\(|\)\s*$/g, '');
  if (!inner.trim()) return 0;

  let depth = 0;
  let count = 1;
  for (const ch of inner) {
    if ('([{<'.includes(ch)) depth += 1;
    else if (')]}>'.includes(ch)) depth -= 1;
    else if (ch === ',' && depth === 0) count += 1;
  }
  return count;
}

const BRANCH = /\b(?:if|else\s+if|elif|for|foreach|while|case|catch|except|when|rescue)\b|\?\s*[^:]+:|&&|\|\||\band\b|\bor\b/g;

function estimateComplexity(bodyText) {
  const matches = stripNoise(bodyText).match(BRANCH);
  return 1 + (matches ? matches.length : 0);
}

function shapeFindings({ blocks = [], maxDepth = 0, thresholds, kind = 'function' }) {
  const findings = [];

  for (const block of blocks) {
    if (block.length > thresholds.function_lines) {
      findings.push(finding({
        rung: 'OBVIOUS', id: 'obvious/function-too-long', line: block.startLine,
        message: `${kind} is ${block.length} lines (limit ${thresholds.function_lines})`,
        fix: 'extract the distinct steps into named functions',
      }));
    }
    if (block.params > thresholds.params) {
      findings.push(finding({
        rung: 'OBVIOUS', id: 'obvious/too-many-params', line: block.startLine,
        message: `${block.params} parameters (limit ${thresholds.params})`,
        fix: 'group them into an options object or struct',
      }));
    }
    if (block.complexity > thresholds.complexity) {
      findings.push(finding({
        rung: 'OBVIOUS', id: 'obvious/complexity', line: block.startLine,
        message: `cyclomatic complexity ~${block.complexity} (limit ${thresholds.complexity})`,
        fix: 'split the branches, or replace the chain with a lookup',
      }));
    }
  }

  if (maxDepth > thresholds.nesting_depth) {
    findings.push(finding({
      rung: 'OBVIOUS', id: 'obvious/nesting-depth', line: 1,
      message: `nesting depth ${maxDepth} (limit ${thresholds.nesting_depth})`,
      fix: 'invert the conditions into guard clauses and return early',
    }));
  }

  return findings;
}

module.exports = {
  stripNoise, analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/shape.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/shape.js tests/shape.test.js
git commit -m "feat: shared shape metrics for the language packs"
```

---

## Task 6: TypeScript / JavaScript pack

**Files:**
- Create: `hooks/checks/lang/ts.js`
- Test: `tests/lang-ts.test.js`, `tests/fixtures/ts/{dirty,clean}.ts`

**Interfaces:**
- Consumes: `finding`, `shape` (`analyzeBraces`, `countParams`, `estimateComplexity`, `shapeFindings`).
- Produces: `check(source, {relPath, config}) → Finding[]`, plus `EXTENSIONS = ['.ts','.tsx','.js','.jsx','.mjs','.cjs']`. Every language pack in Tasks 6–11 exports this same `{check, EXTENSIONS}` shape so `registry.js` can treat them uniformly.

- [ ] **Step 1: Write the failing test**

```js
// tests/lang-ts.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/ts');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.ts', config }).map((f) => f.id);

test('declares the extensions it owns', () => {
  assert.ok(EXTENSIONS.includes('.ts') && EXTENSIONS.includes('.jsx'));
});

test('flags SQL built by string concatenation or template', () => {
  assert.ok(ids('db.query(`SELECT * FROM users WHERE id = ${id}`)').includes('safe/sql-injection'));
  assert.ok(ids('db.query("SELECT * FROM t WHERE a = " + a)').includes('safe/sql-injection'));
  assert.ok(!ids('db.query("SELECT * FROM users WHERE id = ?", [id])').includes('safe/sql-injection'));
});

test('flags XSS sinks and dynamic evaluation', () => {
  assert.ok(ids('el.innerHTML = userInput;').includes('safe/xss-sink'));
  assert.ok(ids('<div dangerouslySetInnerHTML={{ __html: body }} />').includes('safe/xss-sink'));
  assert.ok(ids('eval(payload)').includes('safe/dynamic-eval'));
  assert.ok(!ids('el.textContent = userInput;').includes('safe/xss-sink'));
});

test('flags disabled TLS verification and weak randomness for tokens', () => {
  assert.ok(ids('rejectUnauthorized: false').includes('safe/tls-disabled'));
  assert.ok(ids('const token = Math.random().toString(36);').includes('safe/weak-random'));
  assert.ok(!ids('const jitter = Math.random() * 100;').includes('safe/weak-random'));
});

test('flags swallowed errors and floating promises', () => {
  assert.ok(ids('try { go(); } catch (e) {}').includes('true/swallowed-error'));
  assert.ok(ids('try { go(); } catch (e) { /* ignore */ }').includes('true/swallowed-error'));
  assert.ok(!ids('try { go(); } catch (e) { logger.error(e); }').includes('true/swallowed-error'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('console.log("here")').includes('alone/debug-leftover'));
  assert.ok(ids('debugger;').includes('alone/debug-leftover'));
  assert.ok(!ids('logger.info("started")').includes('alone/debug-leftover'));
});

test('flags shape violations via the shared metrics', () => {
  const long = 'function big(a, b, c, d, e) {\n' + '  work();\n'.repeat(60) + '}\n';
  const found = ids(long);
  assert.ok(found.includes('obvious/function-too-long'));
  assert.ok(found.includes('obvious/too-many-params'));
});

test('flags nested ternaries', () => {
  assert.ok(ids('const x = a ? b ? 1 : 2 : 3;').includes('obvious/nested-ternary'));
  assert.ok(!ids('const x = a ? 1 : 2;').includes('obvious/nested-ternary'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'ts');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.ts'), 'utf8'),
    { relPath: 'clean.ts', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.ts'), 'utf8'),
    { relPath: 'dirty.ts', config }).length >= 6);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/lang-ts.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:query|execute|raw|exec)\s*\(\s*(?:`[^`]*\$\{|["'][^"']*["']\s*\+)/i,
    message: 'SQL built by interpolation or concatenation',
    fix: 'use a parameterized query with bound values',
  },
  {
    id: 'safe/xss-sink', rung: 'SAFE',
    re: /\.innerHTML\s*=|\.outerHTML\s*=|dangerouslySetInnerHTML|document\.write\s*\(/,
    message: 'raw HTML sink',
    fix: 'use textContent, or sanitize before assigning',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE',
    re: /\beval\s*\(|new\s+Function\s*\(|setTimeout\s*\(\s*["'`]/,
    message: 'dynamic code evaluation',
    fix: 'replace with a lookup table or a direct call',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /rejectUnauthorized\s*:\s*false|NODE_TLS_REJECT_UNAUTHORIZED\s*=\s*["']?0/,
    message: 'TLS certificate verification disabled',
    fix: 'trust the proper CA instead of disabling verification',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|otp|session[_-]?id|password)\b[^\n]{0,40}Math\.random\s*\(/i,
    message: 'Math.random() used for a security value',
    fix: 'use crypto.randomUUID() or crypto.randomBytes()',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bdebugger\s*;|\bconsole\.(?:log|debug|dir|trace)\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
  {
    id: 'obvious/nested-ternary', rung: 'OBVIOUS',
    re: /\?[^?:\n]*\?[^:\n]*:[^:\n]*:/,
    message: 'nested ternary',
    fix: 'rewrite as if/else or a lookup',
  },
];

// try { ... } catch (e) { }  — with only whitespace or a comment in the block.
const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;

const FUNCTION_SIGNATURE =
  /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?\w+\s*)\(([^)]*)\)\s*(?::[^{=]+)?(?:=>)?\s*\{/g;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);

  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      if (rule.re.test(line)) {
        findings.push(finding({
          rung: rule.rung, id: rule.id, line: index + 1,
          message: rule.message, fix: rule.fix,
        }));
      }
    }
  });

  for (const match of text.matchAll(SWALLOWED)) {
    findings.push(finding({
      rung: 'TRUE', id: 'true/swallowed-error',
      line: text.slice(0, match.index).split('\n').length,
      message: 'error swallowed by an empty catch',
      fix: 'log with context and rethrow, or handle it explicitly',
    }));
  }

  // Attach params and complexity to the brace blocks that start on a signature line.
  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(FUNCTION_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks.map((block) => {
    const signature = signatures.get(block.startLine);
    const body = lines.slice(block.startLine - 1, block.endLine).join('\n');
    return {
      ...block,
      params: signature === undefined ? 0 : countParams('(' + signature + ')'),
      complexity: signature === undefined ? 0 : estimateComplexity(body),
    };
  }).filter((block) => signatures.has(block.startLine));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'function',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
```

Write `tests/fixtures/ts/dirty.ts` with one instance of each id above, and
`clean.ts` with the passing counterparts: a parameterized query, `textContent`,
`crypto.randomUUID()`, a catch that logs and rethrows, `logger.info`, a
four-parameter function under 40 lines, and a flat ternary.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/lang-ts.test.js`
Expected: PASS (9 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/lang/ts.js tests/lang-ts.test.js tests/fixtures/ts
git commit -m "feat: TypeScript/JavaScript check pack"
```

---

## Task 7: Python pack

**Files:**
- Create: `hooks/checks/lang/py.js`
- Test: `tests/lang-py.test.js`, `tests/fixtures/py/{dirty,clean}.py`

**Interfaces:**
- Consumes: `finding`, `shape` (`analyzeIndent`, `countParams`, `estimateComplexity`, `shapeFindings`).
- Produces: `check(source, {relPath, config}) → Finding[]`, `EXTENSIONS = ['.py']`.

- [ ] **Step 1: Write the failing test**

```js
// tests/lang-py.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/py');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.py', config }).map((f) => f.id);

test('owns the .py extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.py']);
});

test('flags SQL built with f-strings, % or concatenation', () => {
  assert.ok(ids('cursor.execute(f"SELECT * FROM t WHERE id = {uid}")').includes('safe/sql-injection'));
  assert.ok(ids('cursor.execute("SELECT * FROM t WHERE id = %s" % uid)').includes('safe/sql-injection'));
  assert.ok(!ids('cursor.execute("SELECT * FROM t WHERE id = %s", (uid,))').includes('safe/sql-injection'));
});

test('flags shell and dynamic execution risks', () => {
  assert.ok(ids('subprocess.run(cmd, shell=True)').includes('safe/shell-injection'));
  assert.ok(ids('os.system("rm " + target)').includes('safe/shell-injection'));
  assert.ok(ids('eval(user_input)').includes('safe/dynamic-eval'));
  assert.ok(!ids('subprocess.run(["ls", target])').includes('safe/shell-injection'));
});

test('flags unsafe deserialization and weak hashing', () => {
  assert.ok(ids('data = pickle.loads(payload)').includes('safe/unsafe-deserialize'));
  assert.ok(ids('yaml.load(text)').includes('safe/unsafe-deserialize'));
  assert.ok(ids('hashlib.md5(password.encode())').includes('safe/weak-hash'));
  assert.ok(!ids('yaml.safe_load(text)').includes('safe/unsafe-deserialize'));
});

test('flags bare and silent exception handling', () => {
  assert.ok(ids('try:\n    go()\nexcept:\n    pass\n').includes('true/bare-except'));
  assert.ok(ids('try:\n    go()\nexcept Exception:\n    pass\n').includes('true/swallowed-error'));
  assert.ok(!ids('try:\n    go()\nexcept ValueError as e:\n    logger.exception(e)\n    raise\n').includes('true/bare-except'));
});

test('flags mutable default arguments', () => {
  assert.ok(ids('def add(item, into=[]):').includes('true/mutable-default'));
  assert.ok(ids('def add(item, opts={}):').includes('true/mutable-default'));
  assert.ok(!ids('def add(item, into=None):').includes('true/mutable-default'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('print("here")').includes('alone/debug-leftover'));
  assert.ok(ids('breakpoint()').includes('alone/debug-leftover'));
  assert.ok(!ids('logger.info("started")').includes('alone/debug-leftover'));
});

test('flags shape violations using indentation depth', () => {
  const deep = 'def a():\n    if x:\n        for y in z:\n            while w:\n                go()\n';
  assert.ok(ids(deep).includes('obvious/nesting-depth'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'py');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.py'), 'utf8'),
    { relPath: 'clean.py', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.py'), 'utf8'),
    { relPath: 'dirty.py', config }).length >= 6);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/lang-py.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — Python pack.

const { finding } = require('../finding');
const { analyzeIndent, countParams, estimateComplexity, shapeFindings } = require('../shape');

const EXTENSIONS = ['.py'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:execute|executemany|raw|text)\s*\(\s*(?:f["']|["'][^"']*["']\s*%|["'][^"']*["']\s*\+|["'][^"']*["']\s*\.format\s*\()/i,
    message: 'SQL built by f-string, % or concatenation',
    fix: 'pass parameters as the second argument instead',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /shell\s*=\s*True|\bos\.system\s*\(|\bos\.popen\s*\(/,
    message: 'shell execution with an interpolated command',
    fix: 'pass an argument list and leave shell=False',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE',
    re: /\beval\s*\(|\bexec\s*\(|\b__import__\s*\(/,
    message: 'dynamic code evaluation',
    fix: 'replace with a dict lookup or a direct call',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /\bpickle\.loads?\s*\(|\bmarshal\.loads?\s*\(|\byaml\.load\s*\((?![^)]*Safe)/,
    message: 'unsafe deserialization of untrusted bytes',
    fix: 'use json, or yaml.safe_load',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\bhashlib\.(?:md5|sha1)\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use hashlib.sha256, or argon2/bcrypt for passwords',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /verify\s*=\s*False|ssl\._create_unverified_context/,
    message: 'TLS certificate verification disabled',
    fix: 'point at the proper CA bundle instead',
  },
  {
    id: 'true/mutable-default', rung: 'TRUE',
    re: /\bdef\s+\w+\s*\([^)]*=\s*(?:\[\s*\]|\{\s*\}|set\s*\(\s*\))/,
    message: 'mutable default argument — shared across calls',
    fix: 'default to None and build the value inside the function',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /^\s*print\s*\(|\bbreakpoint\s*\(\s*\)|\bpdb\.set_trace\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

const BARE_EXCEPT = /^\s*except\s*:\s*$/;
const BROAD_EXCEPT = /^\s*except\s+(?:Exception|BaseException)\b[^:]*:\s*$/;
const SILENT_BODY = /^\s*(?:pass|\.\.\.)\s*$/;
const DEF_LINE = /^\s*(?:async\s+)?def\s+\w+\s*\(([^)]*)\)/;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const lines = String(source || '').split(/\r?\n/);

  lines.forEach((line, index) => {
    const lineNo = index + 1;

    for (const rule of LINE_RULES) {
      if (rule.re.test(line)) {
        findings.push(finding({
          rung: rule.rung, id: rule.id, line: lineNo,
          message: rule.message, fix: rule.fix,
        }));
      }
    }

    if (BARE_EXCEPT.test(line)) {
      findings.push(finding({
        rung: 'TRUE', id: 'true/bare-except', line: lineNo,
        message: 'bare except catches SystemExit and KeyboardInterrupt too',
        fix: 'catch the specific exception you can actually handle',
      }));
    } else if (BROAD_EXCEPT.test(line) && SILENT_BODY.test(lines[index + 1] || '')) {
      findings.push(finding({
        rung: 'TRUE', id: 'true/swallowed-error', line: lineNo,
        message: 'exception silently discarded',
        fix: 'log with context and re-raise, or handle it explicitly',
      }));
    }
  });

  const { maxDepth, blocks } = analyzeIndent(source, { tabWidth: 4 });
  const measured = blocks.map((block) => {
    const signature = DEF_LINE.exec(lines[block.startLine - 1] || '');
    const body = lines.slice(block.startLine - 1, block.endLine).join('\n');
    return {
      ...block,
      params: signature ? countParams('(' + signature[1] + ')') : 0,
      complexity: estimateComplexity(body),
    };
  });

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'function',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
```

Write `tests/fixtures/py/dirty.py` with one instance of each id, and `clean.py`
with the passing counterparts: parameterized `execute`, `subprocess.run` with an
argument list, `yaml.safe_load`, `hashlib.sha256`, `except ValueError as e:` that
logs and re-raises, `into=None`, `logger.info`, and a shallow function.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/lang-py.test.js`
Expected: PASS (9 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/lang/py.js tests/lang-py.test.js tests/fixtures/py
git commit -m "feat: Python check pack"
```

---

## Task 8: Go pack

**Files:**
- Create: `hooks/checks/lang/go.js`
- Test: `tests/lang-go.test.js`, `tests/fixtures/go/{dirty,clean}.go`

**Interfaces:**
- Consumes: `finding`, `shape` (`analyzeBraces`, `countParams`, `estimateComplexity`, `shapeFindings`).
- Produces: `check(source, {relPath, config}) → Finding[]`, `EXTENSIONS = ['.go']`.

- [ ] **Step 1: Write the failing test**

```js
// tests/lang-go.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/go');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.go', config }).map((f) => f.id);

test('owns the .go extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.go']);
});

test('flags discarded errors', () => {
  assert.ok(ids('result, _ := doWork()').includes('true/ignored-error'));
  assert.ok(!ids('result, err := doWork()\nif err != nil {\n\treturn err\n}').includes('true/ignored-error'));
});

test('flags SQL interpolation', () => {
  assert.ok(ids('db.Query(fmt.Sprintf("SELECT * FROM t WHERE id = %s", id))').includes('safe/sql-injection'));
  assert.ok(!ids('db.Query("SELECT * FROM t WHERE id = $1", id)').includes('safe/sql-injection'));
});

test('flags command injection and disabled TLS', () => {
  assert.ok(ids('exec.Command("sh", "-c", userInput)').includes('safe/shell-injection'));
  assert.ok(ids('InsecureSkipVerify: true').includes('safe/tls-disabled'));
  assert.ok(!ids('exec.Command("ls", dir)').includes('safe/shell-injection'));
});

test('flags weak hashing and math/rand for secrets', () => {
  assert.ok(ids('h := md5.New()').includes('safe/weak-hash'));
  assert.ok(ids('token := rand.Int63()').includes('safe/weak-random'));
  assert.ok(!ids('h := sha256.New()').includes('safe/weak-hash'));
});

test('flags panic in library code and missing Close', () => {
  assert.ok(ids('panic("unreachable")').includes('true/panic-in-library'));
  assert.ok(ids('resp, err := http.Get(url)').includes('true/unclosed-resource'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('fmt.Println("here")').includes('alone/debug-leftover'));
  assert.ok(!ids('log.Info("started")').includes('alone/debug-leftover'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'go');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.go'), 'utf8'),
    { relPath: 'clean.go', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.go'), 'utf8'),
    { relPath: 'dirty.go', config }).length >= 5);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/lang-go.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — Go pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.go'];

const LINE_RULES = [
  {
    id: 'true/ignored-error', rung: 'TRUE',
    re: /,\s*_\s*(?::?=)\s*\w|\b_\s*=\s*\w+\.(?:Close|Write|Exec)\s*\(/,
    message: 'error discarded into _',
    fix: 'handle it, wrap it with context, or return it',
  },
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:Query|QueryRow|Exec)\w*\s*\(\s*(?:fmt\.Sprintf|["'`][^"'`]*["'`]\s*\+)/,
    message: 'SQL built by Sprintf or concatenation',
    fix: 'use placeholders ($1, ?) and pass the values as arguments',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /exec\.Command\s*\(\s*["'`](?:sh|bash|cmd|powershell)["'`]\s*,\s*["'`]-c/,
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with an argument slice',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /InsecureSkipVerify\s*:\s*true/,
    message: 'TLS certificate verification disabled',
    fix: 'configure RootCAs with the proper certificate',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\b(?:md5|sha1)\.(?:New|Sum)\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use sha256, or argon2/bcrypt for passwords',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*:?=\s*rand\.(?:Int|Intn|Int63|Float64)\b/i,
    message: 'math/rand used for a security value',
    fix: 'use crypto/rand',
  },
  {
    id: 'true/panic-in-library', rung: 'TRUE',
    re: /^\s*panic\s*\(/,
    message: 'panic in library code crashes the caller',
    fix: 'return an error and let the caller decide',
  },
  {
    id: 'true/unclosed-resource', rung: 'TRUE',
    re: /\b(?:resp|res|f|file|conn|rows)\s*,\s*(?:err|_)\s*:?=\s*(?:http\.(?:Get|Post|Do)|os\.(?:Open|Create)|net\.Dial|db\.Query)\b/,
    message: 'resource opened without a visible Close',
    fix: 'add defer <resource>.Close() on the next line',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bfmt\.Print(?:ln|f)?\s*\(|\bspew\.Dump\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

const FUNC_SIGNATURE = /func\s+(?:\([^)]*\)\s*)?\w*\s*\(([^)]*)\)[^{\n]*\{/g;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);

  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      if (!rule.re.test(line)) continue;
      // defer Close() on the following line satisfies the resource rule.
      if (rule.id === 'true/unclosed-resource' &&
          /defer\s+\w+\.Close\s*\(/.test(lines.slice(index + 1, index + 4).join('\n'))) {
        continue;
      }
      findings.push(finding({
        rung: rule.rung, id: rule.id, line: index + 1,
        message: rule.message, fix: rule.fix,
      }));
    }
  });

  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(FUNC_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'func',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
```

Write `tests/fixtures/go/dirty.go` with one instance of each id, and `clean.go`
with: `if err != nil { return err }`, placeholder SQL, `exec.Command("ls", dir)`,
`sha256.New()`, `crypto/rand`, a returned error rather than a panic, an
`http.Get` followed by `defer resp.Body.Close()`, and a project logger call.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/lang-go.test.js`
Expected: PASS (8 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/lang/go.js tests/lang-go.test.js tests/fixtures/go
git commit -m "feat: Go check pack"
```

---

## Task 9: Rust pack

**Files:**
- Create: `hooks/checks/lang/rust.js`
- Test: `tests/lang-rust.test.js`, `tests/fixtures/rust/{dirty,clean}.rs`

**Interfaces:**
- Consumes: `finding`, `shape` (`analyzeBraces`, `countParams`, `estimateComplexity`, `shapeFindings`).
- Produces: `check(source, {relPath, config}) → Finding[]`, `EXTENSIONS = ['.rs']`.

- [ ] **Step 1: Write the failing test**

```js
// tests/lang-rust.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/rust');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'x.rs', config }).map((f) => f.id);

test('owns the .rs extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.rs']);
});

test('flags unwrap and expect on fallible calls', () => {
  assert.ok(ids('let v = parse(input).unwrap();').includes('true/unwrap-in-library'));
  assert.ok(ids('let v = parse(input).expect("should parse");').includes('true/unwrap-in-library'));
  assert.ok(!ids('let v = parse(input)?;').includes('true/unwrap-in-library'));
});

test('does not flag unwrap inside tests', () => {
  const src = '#[cfg(test)]\nmod tests {\n    #[test]\n    fn t() {\n        parse("x").unwrap();\n    }\n}\n';
  assert.ok(!ids(src).includes('true/unwrap-in-library'));
});

test('flags unsafe blocks without a safety comment', () => {
  assert.ok(ids('unsafe { ptr::read(p) }').includes('safe/unsafe-block'));
  assert.ok(!ids('// SAFETY: p is non-null and aligned, checked above.\nunsafe { ptr::read(p) }').includes('safe/unsafe-block'));
});

test('flags SQL interpolation and command injection', () => {
  assert.ok(ids('sqlx::query(&format!("SELECT * FROM t WHERE id = {}", id))').includes('safe/sql-injection'));
  assert.ok(ids('Command::new("sh").arg("-c").arg(user_input)').includes('safe/shell-injection'));
});

test('flags disabled certificate verification and weak randomness', () => {
  assert.ok(ids('.danger_accept_invalid_certs(true)').includes('safe/tls-disabled'));
  assert.ok(ids('let token = rand::random::<u64>();').includes('safe/weak-random'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('println!("here");').includes('alone/debug-leftover'));
  assert.ok(ids('dbg!(value);').includes('alone/debug-leftover'));
  assert.ok(!ids('tracing::info!("started");').includes('alone/debug-leftover'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'rust');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.rs'), 'utf8'),
    { relPath: 'clean.rs', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.rs'), 'utf8'),
    { relPath: 'dirty.rs', config }).length >= 5);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/lang-rust.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — Rust pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.rs'];

const LINE_RULES = [
  {
    id: 'true/unwrap-in-library', rung: 'TRUE',
    re: /\.\s*(?:unwrap|expect)\s*\(/,
    message: 'unwrap/expect panics on the caller',
    fix: 'propagate with ? and a typed error',
  },
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\bquery\w*\s*\(\s*&?format!|\bexecute\s*\(\s*&?format!/,
    message: 'SQL built with format!',
    fix: 'use bind parameters',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /Command::new\s*\(\s*"(?:sh|bash|cmd|powershell)"\s*\)[^;]*\.arg\s*\(\s*"-c"/,
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with separate args',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /danger_accept_invalid_certs\s*\(\s*true\s*\)|danger_accept_invalid_hostnames\s*\(\s*true\s*\)/,
    message: 'TLS certificate verification disabled',
    fix: 'add the proper root certificate instead',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*(?::[^=]+)?=\s*rand::(?:random|thread_rng)\b/i,
    message: 'general-purpose RNG used for a security value',
    fix: 'use a CSPRNG (rand::rngs::OsRng or the ring crate)',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bdbg!\s*\(|\bprintln!\s*\(|\beprintln!\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or use the tracing/log crate',
  },
];

const UNSAFE_BLOCK = /^\s*(?:.*\s)?unsafe\s*\{/;
const SAFETY_COMMENT = /\/\/\s*SAFETY:|\/\/!\s*SAFETY:/i;
const FN_SIGNATURE = /fn\s+\w+\s*(?:<[^>]*>)?\s*\(([^)]*)\)[^{\n]*\{/g;

// unwrap in a #[cfg(test)] module or a #[test] fn is normal and not a finding.
function testRegions(lines) {
  const regions = [];
  lines.forEach((line, index) => {
    if (/#\[cfg\(test\)\]|#\[test\]|#\[tokio::test\]/.test(line)) {
      regions.push([index + 1, lines.length]);
    }
  });
  return regions;
}

function inRegions(regions, lineNo) {
  return regions.some(([start, end]) => lineNo >= start && lineNo <= end);
}

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);
  const tests = testRegions(lines);
  const isTestFile = /(?:^|\/)tests\//.test(String(relPath || '')) || /_test\.rs$/.test(String(relPath || ''));

  lines.forEach((line, index) => {
    const lineNo = index + 1;

    for (const rule of LINE_RULES) {
      if (!rule.re.test(line)) continue;
      if (rule.id === 'true/unwrap-in-library' && (isTestFile || inRegions(tests, lineNo))) continue;
      findings.push(finding({
        rung: rule.rung, id: rule.id, line: lineNo,
        message: rule.message, fix: rule.fix,
      }));
    }

    if (UNSAFE_BLOCK.test(line)) {
      const preceding = lines.slice(Math.max(0, index - 3), index).join('\n');
      if (!SAFETY_COMMENT.test(preceding)) {
        findings.push(finding({
          rung: 'SAFE', id: 'safe/unsafe-block', line: lineNo,
          message: 'unsafe block with no SAFETY comment',
          fix: 'add // SAFETY: stating the invariants that make this sound',
        }));
      }
    }
  });

  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(FN_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'fn',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
```

Write `tests/fixtures/rust/dirty.rs` with one instance of each id, and `clean.rs`
with: `?` propagation, bind parameters, a direct `Command::new("ls")`, a
`SAFETY:` comment above an unsafe block, `OsRng`, and `tracing::info!`.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/lang-rust.test.js`
Expected: PASS (8 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/lang/rust.js tests/lang-rust.test.js tests/fixtures/rust
git commit -m "feat: Rust check pack"
```

---

## Task 10: JVM pack (Java / Kotlin)

**Files:**
- Create: `hooks/checks/lang/jvm.js`
- Test: `tests/lang-jvm.test.js`, `tests/fixtures/jvm/{dirty,clean}.java`

**Interfaces:**
- Consumes: `finding`, `shape` (`analyzeBraces`, `countParams`, `estimateComplexity`, `shapeFindings`).
- Produces: `check(source, {relPath, config}) → Finding[]`, `EXTENSIONS = ['.java','.kt','.kts']`.

- [ ] **Step 1: Write the failing test**

```js
// tests/lang-jvm.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/jvm');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'X.java', config }).map((f) => f.id);

test('owns the JVM extensions', () => {
  assert.ok(EXTENSIONS.includes('.java') && EXTENSIONS.includes('.kt'));
});

test('flags SQL string building', () => {
  assert.ok(ids('stmt.executeQuery("SELECT * FROM t WHERE id = " + id);').includes('safe/sql-injection'));
  assert.ok(ids('String q = String.format("SELECT * FROM t WHERE id = %s", id);\nstmt.executeQuery(q);').includes('safe/sql-injection'));
  assert.ok(!ids('PreparedStatement ps = conn.prepareStatement("SELECT * FROM t WHERE id = ?");').includes('safe/sql-injection'));
});

test('flags unsafe deserialization and XXE-prone parsing', () => {
  assert.ok(ids('ObjectInputStream in = new ObjectInputStream(payload);').includes('safe/unsafe-deserialize'));
  assert.ok(ids('DocumentBuilderFactory.newInstance()').includes('safe/xxe-risk'));
});

test('flags weak hashing and predictable randomness', () => {
  assert.ok(ids('MessageDigest.getInstance("MD5")').includes('safe/weak-hash'));
  assert.ok(ids('String token = String.valueOf(new Random().nextLong());').includes('safe/weak-random'));
  assert.ok(!ids('MessageDigest.getInstance("SHA-256")').includes('safe/weak-hash'));
});

test('flags swallowed exceptions', () => {
  assert.ok(ids('try { go(); } catch (Exception e) { }').includes('true/swallowed-error'));
  assert.ok(ids('catch (IOException e) { e.printStackTrace(); }').includes('true/printstacktrace'));
  assert.ok(!ids('catch (IOException e) { log.error("read failed", e); throw e; }').includes('true/swallowed-error'));
});

test('flags leftover debugging', () => {
  assert.ok(ids('System.out.println("here");').includes('alone/debug-leftover'));
  assert.ok(!ids('log.info("started");').includes('alone/debug-leftover'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'jvm');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.java'), 'utf8'),
    { relPath: 'clean.java', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.java'), 'utf8'),
    { relPath: 'dirty.java', config }).length >= 5);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/lang-jvm.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — Java / Kotlin pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.java', '.kt', '.kts'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /(?:executeQuery|executeUpdate|createQuery|rawQuery|execute)\s*\(\s*(?:"[^"]*"\s*\+|String\.format|\w+\s*\+)|String\.format\s*\(\s*"\s*SELECT/i,
    message: 'SQL built by concatenation or format',
    fix: 'use PreparedStatement with bound parameters',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /new\s+ObjectInputStream\s*\(|XMLDecoder\s*\(|readObject\s*\(\s*\)/,
    message: 'Java native deserialization of untrusted bytes',
    fix: 'use a data format (JSON) with an explicit schema',
  },
  {
    id: 'safe/xxe-risk', rung: 'SAFE',
    re: /DocumentBuilderFactory\.newInstance\s*\(|SAXParserFactory\.newInstance\s*\(|XMLInputFactory\.newInstance\s*\(/,
    message: 'XML parser created without external entities disabled',
    fix: 'setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /MessageDigest\.getInstance\s*\(\s*"(?:MD5|SHA-?1)"/i,
    message: 'weak hash used where a secure one is expected',
    fix: 'use SHA-256, or BCrypt/Argon2 for passwords',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*=\s*[^;]*new\s+Random\s*\(|Math\.random\s*\(\s*\)[^;]*\b(?:token|key|secret)\b/i,
    message: 'java.util.Random used for a security value',
    fix: 'use SecureRandom',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /TrustAllCerts|checkServerTrusted\s*\([^)]*\)\s*\{\s*\}|ALLOW_ALL_HOSTNAME_VERIFIER/,
    message: 'TLS certificate or hostname verification disabled',
    fix: 'trust the proper CA instead of accepting all certificates',
  },
  {
    id: 'true/printstacktrace', rung: 'TRUE',
    re: /\.printStackTrace\s*\(\s*\)/,
    message: 'exception printed to stderr instead of handled',
    fix: 'log with context through the project logger, then rethrow or handle',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /System\.(?:out|err)\.print(?:ln|f)?\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;
const METHOD_SIGNATURE =
  /(?:public|private|protected|internal|fun|static|final|\s)+[\w<>\[\],\s.]+\s+\w+\s*\(([^)]*)\)\s*(?:throws [\w,\s.]+)?\{/g;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);

  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      if (rule.re.test(line)) {
        findings.push(finding({
          rung: rule.rung, id: rule.id, line: index + 1,
          message: rule.message, fix: rule.fix,
        }));
      }
    }
  });

  for (const match of text.matchAll(SWALLOWED)) {
    findings.push(finding({
      rung: 'TRUE', id: 'true/swallowed-error',
      line: text.slice(0, match.index).split('\n').length,
      message: 'exception swallowed by an empty catch',
      fix: 'log with context and rethrow, or handle it explicitly',
    }));
  }

  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(METHOD_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'method',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
```

Write `tests/fixtures/jvm/dirty.java` with one instance of each id, and
`clean.java` with: `PreparedStatement` with `?`, JSON parsing rather than
`ObjectInputStream`, an XML factory with `disallow-doctype-decl` set,
`SHA-256`, `SecureRandom`, a catch that logs with context and rethrows, and
`log.info`.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/lang-jvm.test.js`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/lang/jvm.js tests/lang-jvm.test.js tests/fixtures/jvm
git commit -m "feat: JVM check pack"
```

---

## Task 11: .NET pack and the registry

**Files:**
- Create: `hooks/checks/lang/dotnet.js`, `hooks/checks/registry.js`
- Test: `tests/lang-dotnet.test.js`, `tests/registry.test.js`, `tests/fixtures/dotnet/{dirty,clean}.cs`

**Interfaces:**
- Consumes: every `lang/*.js` pack.
- Produces:
  - `dotnet`: `check(source, {relPath, config}) → Finding[]`, `EXTENSIONS = ['.cs']`
  - `registry`: `packFor(relPath) → pack|null`, `toolFor(relPath) → {name, argv, parse}|null`, `PACKS` array

- [ ] **Step 1: Write the failing test**

```js
// tests/lang-dotnet.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/dotnet');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'X.cs', config }).map((f) => f.id);

test('owns the .cs extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.cs']);
});

test('flags SQL string building', () => {
  assert.ok(ids('new SqlCommand($"SELECT * FROM t WHERE id = {id}", conn);').includes('safe/sql-injection'));
  assert.ok(ids('cmd.CommandText = "SELECT * FROM t WHERE id = " + id;').includes('safe/sql-injection'));
  assert.ok(!ids('cmd.Parameters.AddWithValue("@id", id);').includes('safe/sql-injection'));
});

test('flags unsafe deserialization and weak crypto', () => {
  assert.ok(ids('var f = new BinaryFormatter();').includes('safe/unsafe-deserialize'));
  assert.ok(ids('MD5.Create()').includes('safe/weak-hash'));
  assert.ok(ids('var token = new Random().Next().ToString();').includes('safe/weak-random'));
  assert.ok(!ids('SHA256.Create()').includes('safe/weak-hash'));
});

test('flags disabled certificate validation', () => {
  assert.ok(ids('ServerCertificateValidationCallback = (a, b, c, d) => true;').includes('safe/tls-disabled'));
});

test('flags swallowed exceptions and leftover debugging', () => {
  assert.ok(ids('try { Go(); } catch (Exception) { }').includes('true/swallowed-error'));
  assert.ok(ids('Console.WriteLine("here");').includes('alone/debug-leftover'));
  assert.ok(!ids('_logger.LogInformation("started");').includes('alone/debug-leftover'));
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'dotnet');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.cs'), 'utf8'),
    { relPath: 'clean.cs', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.cs'), 'utf8'),
    { relPath: 'dirty.cs', config }).length >= 4);
});
```

```js
// tests/registry.test.js
const test = require('node:test');
const assert = require('node:assert');
const { packFor, toolFor, PACKS } = require('../hooks/checks/registry');

test('maps every supported extension to exactly one pack', () => {
  const seen = new Map();
  for (const pack of PACKS) {
    for (const ext of pack.EXTENSIONS) {
      assert.ok(!seen.has(ext), `${ext} claimed by two packs`);
      seen.set(ext, pack);
    }
  }
  assert.ok(seen.size >= 12);
});

test('packFor resolves by extension and is case-insensitive', () => {
  assert.ok(packFor('src/a.ts'));
  assert.ok(packFor('src/A.PY'));
  assert.strictEqual(packFor('README.md'), null);
  assert.strictEqual(packFor('Makefile'), null);
});

test('toolFor names the external tool preferred for each language', () => {
  assert.strictEqual(toolFor('a.py').name, 'ruff');
  assert.strictEqual(toolFor('a.ts').name, 'eslint');
  assert.strictEqual(toolFor('a.go').name, 'golangci-lint');
  assert.strictEqual(toolFor('a.rs').name, 'clippy');
  assert.strictEqual(toolFor('README.md'), null);
});

test('each tool entry can parse its own output format', () => {
  const ruff = toolFor('a.py');
  const parsed = ruff.parse(JSON.stringify([
    { filename: 'a.py', location: { row: 7 }, code: 'E722', message: 'do not use bare except' },
  ]));
  assert.strictEqual(parsed.length, 1);
  assert.strictEqual(parsed[0].line, 7);
  assert.match(parsed[0].message, /bare except/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/lang-dotnet.test.js tests/registry.test.js`
Expected: FAIL — both modules not found.

- [ ] **Step 3: Write the implementations**

```js
#!/usr/bin/env node
// procoder — C# / .NET pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.cs'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /(?:SqlCommand|CommandText|ExecuteSqlRaw|FromSqlRaw)\s*(?:\(|=)\s*(?:\$"|"[^"]*"\s*\+|\w+\s*\+)/,
    message: 'SQL built by interpolation or concatenation',
    fix: 'use parameters (cmd.Parameters.AddWithValue) or FromSqlInterpolated',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /new\s+BinaryFormatter\s*\(|NetDataContractSerializer|LosFormatter|TypeNameHandling\s*=\s*TypeNameHandling\.(?:All|Objects|Auto)/,
    message: 'unsafe deserialization of untrusted input',
    fix: 'use System.Text.Json with an explicit contract',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\b(?:MD5|SHA1)\.Create\s*\(|new\s+(?:MD5|SHA1)CryptoServiceProvider\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use SHA256, or PBKDF2/Argon2 for passwords',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*=\s*[^;]*new\s+Random\s*\(/i,
    message: 'System.Random used for a security value',
    fix: 'use RandomNumberGenerator.GetBytes',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /ServerCertificateValidationCallback\s*(?:\+?=)\s*[^;]*=>\s*true|DangerousAcceptAnyServerCertificateValidator/,
    message: 'TLS certificate validation disabled',
    fix: 'validate against the proper CA instead',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /Console\.(?:WriteLine|Write)\s*\(|Debug\.WriteLine\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through ILogger',
  },
];

const SWALLOWED = /catch\s*(?:\([^)]*\))?\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;
const METHOD_SIGNATURE =
  /(?:public|private|protected|internal|static|async|override|\s)+[\w<>\[\],\s.?]+\s+\w+\s*\(([^)]*)\)\s*\{/g;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);

  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      if (rule.re.test(line)) {
        findings.push(finding({
          rung: rule.rung, id: rule.id, line: index + 1,
          message: rule.message, fix: rule.fix,
        }));
      }
    }
  });

  for (const match of text.matchAll(SWALLOWED)) {
    findings.push(finding({
      rung: 'TRUE', id: 'true/swallowed-error',
      line: text.slice(0, match.index).split('\n').length,
      message: 'exception swallowed by an empty catch',
      fix: 'log with context and rethrow, or handle it explicitly',
    }));
  }

  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(METHOD_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'method',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
```

```js
#!/usr/bin/env node
// procoder — extension → pack, and pack → preferred external tool.
//
// The tool entries describe how to INVOKE and PARSE each linter. Whether one is
// actually configured in the project is resolve.js's job.

const path = require('path');
const { finding } = require('./finding');

const ts = require('./lang/ts');
const py = require('./lang/py');
const go = require('./lang/go');
const rust = require('./lang/rust');
const jvm = require('./lang/jvm');
const dotnet = require('./lang/dotnet');

const PACKS = [ts, py, go, rust, jvm, dotnet];

// External findings land on rung TRUE: a configured linter's rules are the
// project's own definition of correct, and procoder defers to them.
function externalFinding(line, message, tool) {
  return finding({
    rung: 'TRUE', id: `true/${tool}`, line,
    message: String(message).slice(0, 120),
    fix: `resolve the ${tool} finding`,
  });
}

const TOOLS = {
  py: {
    name: 'ruff',
    configFiles: ['ruff.toml', '.ruff.toml', 'pyproject.toml', 'setup.cfg'],
    argv: (file) => ['check', '--output-format', 'json', '--force-exclude', file],
    parse: (stdout) => {
      try {
        return JSON.parse(stdout).map((item) =>
          externalFinding(item.location && item.location.row, `${item.code}: ${item.message}`, 'ruff'));
      } catch (e) {
        return [];
      }
    },
  },
  ts: {
    name: 'eslint',
    configFiles: ['eslint.config.js', 'eslint.config.mjs', '.eslintrc', '.eslintrc.json', '.eslintrc.cjs', '.eslintrc.js'],
    argv: (file) => ['--format', 'json', file],
    parse: (stdout) => {
      try {
        const results = JSON.parse(stdout);
        return results.flatMap((result) => (result.messages || []).map((m) =>
          externalFinding(m.line, `${m.ruleId || 'eslint'}: ${m.message}`, 'eslint')));
      } catch (e) {
        return [];
      }
    },
  },
  go: {
    name: 'golangci-lint',
    configFiles: ['.golangci.yml', '.golangci.yaml', '.golangci.toml'],
    argv: (file) => ['run', '--out-format', 'json', file],
    parse: (stdout) => {
      try {
        return (JSON.parse(stdout).Issues || []).map((issue) =>
          externalFinding(issue.Pos && issue.Pos.Line, `${issue.FromLinter}: ${issue.Text}`, 'golangci-lint'));
      } catch (e) {
        return [];
      }
    },
  },
  rust: {
    name: 'clippy',
    configFiles: ['clippy.toml', '.clippy.toml', 'Cargo.toml'],
    argv: () => ['clippy', '--message-format', 'short', '--quiet'],
    parse: (stdout) => String(stdout).split('\n')
      .map((line) => /^[^:]+:(\d+):\d+:\s*(?:warning|error):\s*(.+)$/.exec(line))
      .filter(Boolean)
      .map((m) => externalFinding(Number(m[1]), m[2], 'clippy')),
  },
};

const EXT_TO_TOOL = new Map();
for (const ext of ts.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.ts);
for (const ext of py.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.py);
for (const ext of go.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.go);
for (const ext of rust.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.rust);
// jvm and dotnet have no fast single-file linter worth a 1.5s budget; their
// built-in packs always run instead.

const EXT_TO_PACK = new Map();
for (const pack of PACKS) {
  for (const ext of pack.EXTENSIONS) EXT_TO_PACK.set(ext, pack);
}

function packFor(relPath) {
  return EXT_TO_PACK.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

function toolFor(relPath) {
  return EXT_TO_TOOL.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

module.exports = { PACKS, TOOLS, packFor, toolFor };
```

Write the `.cs` fixture pair mirroring the ids above.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/lang-dotnet.test.js tests/registry.test.js`
Expected: PASS (6 + 4 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/lang/dotnet.js hooks/checks/registry.js \
        tests/lang-dotnet.test.js tests/registry.test.js tests/fixtures/dotnet
git commit -m "feat: .NET check pack and language registry"
```

---

## Task 12: Tooling resolver

**Files:**
- Create: `hooks/checks/resolve.js`
- Test: `tests/resolve.test.js`

**Interfaces:**
- Consumes: `registry` (`toolFor`).
- Produces:
  - `hasTool(name) → boolean` (on PATH)
  - `isConfigured(repoRoot, tool) → boolean` (a config file exists)
  - `runTool(tool, {repoRoot, absPath, timeoutMs}) → Finding[]` (empty on timeout, missing binary, or unparseable output)
  - `resolveFor(relPath, {repoRoot}) → tool|null`

- [ ] **Step 1: Write the failing test**

```js
// tests/resolve.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { hasTool, isConfigured, resolveFor, runTool } = require('../hooks/checks/resolve');
const { TOOLS } = require('../hooks/checks/registry');

function tempRepo(files = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-res-'));
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('hasTool finds node and does not find a nonsense binary', () => {
  assert.strictEqual(hasTool('node'), true);
  assert.strictEqual(hasTool('procoder-definitely-not-a-real-binary'), false);
});

test('isConfigured requires one of the tool config files', () => {
  assert.strictEqual(isConfigured(tempRepo({ '.eslintrc.json': '{}' }), TOOLS.ts), true);
  assert.strictEqual(isConfigured(tempRepo({ 'ruff.toml': '' }), TOOLS.py), true);
  assert.strictEqual(isConfigured(tempRepo(), TOOLS.ts), false);
});

test('resolveFor yields null when the tool is unconfigured', () => {
  assert.strictEqual(resolveFor('a.py', { repoRoot: tempRepo() }), null);
});

test('resolveFor yields null for a file type with no tool', () => {
  assert.strictEqual(resolveFor('a.cs', { repoRoot: tempRepo({ '.eslintrc': '{}' }) }), null);
});

test('runTool returns an empty array when the binary is missing', () => {
  const fake = { name: 'procoder-missing-binary', argv: () => ['x'], parse: () => [{}] };
  assert.deepStrictEqual(runTool(fake, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 500 }), []);
});

test('runTool honours the timeout and returns an empty array', () => {
  const slow = { name: 'node', argv: () => ['-e', 'setTimeout(()=>{}, 10000)'], parse: () => [{}] };
  const started = Date.now();
  const out = runTool(slow, { repoRoot: '/tmp', absPath: '/tmp/x', timeoutMs: 400 });
  assert.deepStrictEqual(out, []);
  assert.ok(Date.now() - started < 3000, 'runTool did not abandon the slow process');
});

test('runTool parses stdout through the tool parser', () => {
  const echo = {
    name: 'node',
    argv: () => ['-e', 'process.stdout.write(JSON.stringify([{filename:"a.py",location:{row:3},code:"E1",message:"boom"}]))'],
    parse: TOOLS.py.parse,
  };
  const out = runTool(echo, { repoRoot: '/tmp', absPath: '/tmp/a.py', timeoutMs: 2000 });
  assert.strictEqual(out.length, 1);
  assert.strictEqual(out[0].line, 3);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/resolve.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — external tooling detection and invocation.
//
// The ladder applies to procoder itself: if the project already has a linter
// configured, that linter's rules ARE the project's definition of correct.
// Re-implementing them would create exactly the duplicate-rule rot rung 4
// forbids. The built-in packs exist only for projects with nothing configured.

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const { toolFor } = require('./registry');

const WHICH = process.platform === 'win32' ? 'where' : 'which';

const toolCache = new Map();

function hasTool(name) {
  if (toolCache.has(name)) return toolCache.get(name);
  let found = false;
  try {
    execFileSync(WHICH, [name], { stdio: 'ignore', timeout: 1000 });
    found = true;
  } catch (e) {
    found = false;
  }
  toolCache.set(name, found);
  return found;
}

function isConfigured(repoRoot, tool) {
  if (!tool || !tool.configFiles) return false;
  return tool.configFiles.some((file) => fs.existsSync(path.join(repoRoot, file)));
}

function resolveFor(relPath, { repoRoot }) {
  const tool = toolFor(relPath);
  if (!tool) return null;
  if (!isConfigured(repoRoot, tool)) return null;
  if (!hasTool(tool.name)) return null;
  return tool;
}

// argv is built from the tool definition and a filesystem path only — never
// from file contents. execFileSync (not exec) means no shell is involved.
function runTool(tool, { repoRoot, absPath, timeoutMs = 1500 }) {
  let stdout = '';
  try {
    stdout = execFileSync(tool.name, tool.argv(absPath), {
      cwd: repoRoot,
      encoding: 'utf8',
      timeout: timeoutMs,
      maxBuffer: 4 * 1024 * 1024,
      stdio: ['ignore', 'pipe', 'ignore'],
    });
  } catch (e) {
    // Linters exit non-zero when they find something — the output is still on
    // stdout and still useful. A timeout or missing binary leaves it empty.
    stdout = (e && e.stdout) ? String(e.stdout) : '';
  }

  if (!stdout.trim()) return [];

  try {
    return tool.parse(stdout).filter((f) => f && f.line > 0);
  } catch (e) {
    return [];
  }
}

module.exports = { hasTool, isConfigured, resolveFor, runTool };
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/resolve.test.js`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/resolve.js tests/resolve.test.js
git commit -m "feat: prefer the project's configured linter over built-in patterns"
```

---

## Task 13: Ratchet baseline

**Files:**
- Create: `hooks/checks/baseline.js`
- Test: `tests/baseline.test.js`

**Interfaces:**
- Consumes: nothing beyond `crypto` and `fs`.
- Produces:
  - `fingerprint(finding, relPath, sourceLine) → string`
  - `loadBaseline(repoRoot, config) → Set<string>`
  - `writeBaseline(repoRoot, config, entries) → void`
  - `suppress(findings, {baseline, relPath, lines}) → Finding[]`
  - `growthCheck(baseline, currentCount) → {ok, delta}`

- [ ] **Step 1: Write the failing test**

```js
// tests/baseline.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  fingerprint, loadBaseline, writeBaseline, suppress, growthCheck,
} = require('../hooks/checks/baseline');
const { finding } = require('../hooks/checks/finding');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS };
const tempRepo = () => fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-base-'));
const f = (line, id = 'alone/orphan-todo') =>
  finding({ rung: 'ALONE', id, line, message: 'm', fix: 'x' });

test('fingerprint ignores line numbers and surrounding whitespace', () => {
  const a = fingerprint(f(10), 'src/a.ts', '  // TODO: later');
  const b = fingerprint(f(99), 'src/a.ts', '// TODO: later');
  assert.strictEqual(a, b, 'a reformat must not change the fingerprint');
});

test('fingerprint distinguishes different files, ids, and content', () => {
  const base = fingerprint(f(1), 'src/a.ts', '// TODO: later');
  assert.notStrictEqual(base, fingerprint(f(1), 'src/b.ts', '// TODO: later'));
  assert.notStrictEqual(base, fingerprint(f(1, 'alone/commented-code'), 'src/a.ts', '// TODO: later'));
  assert.notStrictEqual(base, fingerprint(f(1), 'src/a.ts', '// TODO: something else'));
});

test('baseline round-trips through disk', () => {
  const repo = tempRepo();
  writeBaseline(repo, config, ['aaa', 'bbb']);
  const loaded = loadBaseline(repo, config);
  assert.ok(loaded.has('aaa') && loaded.has('bbb'));
  assert.strictEqual(loaded.size, 2);
});

test('an absent or corrupt baseline file loads as empty, never throws', () => {
  const repo = tempRepo();
  assert.strictEqual(loadBaseline(repo, config).size, 0);
  fs.writeFileSync(path.join(repo, '.procoder-baseline.json'), 'not json');
  assert.strictEqual(loadBaseline(repo, config).size, 0);
});

test('suppress removes baselined findings and keeps new ones', () => {
  const lines = ['// TODO: later', 'const x = 1;', '// TODO: other'];
  const known = new Set([fingerprint(f(1), 'a.ts', lines[0])]);
  const out = suppress([f(1), f(3)], { baseline: known, relPath: 'a.ts', lines });
  assert.strictEqual(out.length, 1);
  assert.strictEqual(out[0].line, 3);
});

test('suppression survives the file being reformatted', () => {
  const before = ['// TODO: later'];
  const after = ['    // TODO: later'];
  const known = new Set([fingerprint(f(1), 'a.ts', before[0])]);
  assert.deepStrictEqual(
    suppress([f(1)], { baseline: known, relPath: 'a.ts', lines: after }), []);
});

test('growthCheck fails only when the count rises', () => {
  const baseline = new Set(['a', 'b', 'c']);
  assert.strictEqual(growthCheck(baseline, 3).ok, true);
  assert.strictEqual(growthCheck(baseline, 1).ok, true);
  assert.strictEqual(growthCheck(baseline, 5).ok, false);
  assert.strictEqual(growthCheck(baseline, 5).delta, 2);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/baseline.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — the ratchet.
//
// Without this, opening procoder on a five-year-old repo produces four thousand
// findings and gets switched off within the hour. Existing violations are
// recorded once and suppressed everywhere; only new and changed code is gated,
// and the recorded count may shrink but never grow.
//
// Fingerprints deliberately exclude line numbers: a reformat must not resurrect
// every suppressed finding.

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

function normalizeLine(text) {
  return String(text || '').trim().replace(/\s+/g, ' ');
}

function fingerprint(finding, relPath, sourceLine) {
  const normalizedPath = String(relPath).replace(/\\/g, '/');
  return crypto.createHash('sha1')
    .update(`${finding.id}\0${normalizedPath}\0${normalizeLine(sourceLine)}`)
    .digest('hex');
}

function baselinePath(repoRoot, config) {
  return path.join(repoRoot, (config.baseline && config.baseline.file) || '.procoder-baseline.json');
}

function loadBaseline(repoRoot, config) {
  try {
    const parsed = JSON.parse(fs.readFileSync(baselinePath(repoRoot, config), 'utf8'));
    return new Set(Array.isArray(parsed.fingerprints) ? parsed.fingerprints : []);
  } catch (e) {
    return new Set();
  }
}

function writeBaseline(repoRoot, config, entries) {
  const payload = {
    version: 1,
    note: 'procoder ratchet. Generated by `procoder baseline`. Shrinking is good; growth fails CI.',
    fingerprints: Array.from(new Set(entries)).sort(),
  };
  fs.writeFileSync(baselinePath(repoRoot, config), JSON.stringify(payload, null, 2) + '\n');
}

function suppress(findings, { baseline, relPath, lines }) {
  if (!baseline || baseline.size === 0) return findings;
  return findings.filter((f) =>
    !baseline.has(fingerprint(f, relPath, lines[f.line - 1])));
}

function growthCheck(baseline, currentCount) {
  const delta = currentCount - baseline.size;
  return { ok: delta <= 0, delta };
}

module.exports = {
  fingerprint, loadBaseline, writeBaseline, suppress, growthCheck, baselinePath,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/baseline.test.js`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/baseline.js tests/baseline.test.js
git commit -m "feat: ratchet baseline so legacy repos stay usable"
```

---

## Task 14: Orchestrator

**Files:**
- Create: `hooks/checks/run.js`
- Test: `tests/run.test.js`

**Interfaces:**
- Consumes: `config`, `registry`, `resolve`, `universal`, `baseline`, `finding`.
- Produces: `checkFile(absPath, {repoRoot, config, maxFindings, applyBaseline}) → {relPath, findings, skipped}`. `skipped` is one of `null | 'excluded' | 'unreadable' | 'unsupported'`. `applyBaseline` defaults to `true`; `procoder verify` passes `false` so the ratchet counts total findings rather than only unsuppressed ones — otherwise fixing one finding would silently pay for introducing another.

- [ ] **Step 1: Write the failing test**

```js
// tests/run.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { checkFile } = require('../hooks/checks/run');
const { loadConfig } = require('../hooks/checks/config');
const { writeBaseline, fingerprint } = require('../hooks/checks/baseline');
const { finding } = require('../hooks/checks/finding');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-run-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('runs both the language pack and the universal pack', () => {
  const repo = repoWith({ 'src/a.ts': 'el.innerHTML = x;\n// TODO: later\n' });
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  const ids = out.findings.map((f) => f.id);
  assert.ok(ids.includes('safe/xss-sink'), 'language pack did not run');
  assert.ok(ids.includes('alone/orphan-todo'), 'universal pack did not run');
});

test('runs the universal pack even for unsupported file types', () => {
  const repo = repoWith({ 'notes.md': 'key = "AKIAIOSFODNN7EXAMPLE"\n' });
  const out = checkFile(path.join(repo, 'notes.md'), { repoRoot: repo, config: loadConfig(repo) });
  assert.ok(out.findings.some((f) => f.id === 'safe/hardcoded-secret'));
});

test('excluded paths are skipped entirely', () => {
  const repo = repoWith({
    '.procoder.toml': '[exclude]\npaths = ["generated/"]\n',
    'generated/a.ts': 'eval(x);\n',
  });
  const out = checkFile(path.join(repo, 'generated/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'excluded');
  assert.deepStrictEqual(out.findings, []);
});

test('an unreadable file yields skipped, not a throw', () => {
  const repo = repoWith({});
  const out = checkFile(path.join(repo, 'nope.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.skipped, 'unreadable');
});

test('findings are sorted by rung and capped', () => {
  const repo = repoWith({
    'src/a.ts': [
      '// TODO: one',
      '// TODO: two',
      'eval(a);',
      'el.innerHTML = b;',
      'debugger;',
      'console.log(1);',
    ].join('\n'),
  });
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), maxFindings: 3 });
  assert.strictEqual(out.findings.length, 3);
  assert.strictEqual(out.findings[0].rung, 'SAFE');
});

test('baselined findings are suppressed', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });
  const config = loadConfig(repo);
  writeBaseline(repo, config, [
    fingerprint(finding({ rung: 'SAFE', id: 'safe/dynamic-eval', line: 1, message: 'm', fix: 'f' }),
      'src/a.ts', 'eval(a);'),
  ]);
  const out = checkFile(path.join(repo, 'src/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.deepStrictEqual(out.findings.map((f) => f.id), []);
});

test('applyBaseline false reports findings the baseline would suppress', () => {
  const repo = repoWith({ 'src/a.ts': 'eval(a);\n' });
  const config = loadConfig(repo);
  writeBaseline(repo, config, [
    fingerprint(finding({ rung: 'SAFE', id: 'safe/dynamic-eval', line: 1, message: 'm', fix: 'f' }),
      'src/a.ts', 'eval(a);'),
  ]);
  const out = checkFile(path.join(repo, 'src/a.ts'),
    { repoRoot: repo, config: loadConfig(repo), applyBaseline: false });
  assert.ok(out.findings.some((f) => f.id === 'safe/dynamic-eval'));
});

test('relPath is repo-relative and uses forward slashes', () => {
  const repo = repoWith({ 'src/deep/a.ts': 'eval(a);\n' });
  const out = checkFile(path.join(repo, 'src/deep/a.ts'), { repoRoot: repo, config: loadConfig(repo) });
  assert.strictEqual(out.relPath, 'src/deep/a.ts');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/run.test.js`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

```js
#!/usr/bin/env node
// procoder — orchestrates one file's checks.
//
// Order: exclusion → read → language pack (or the project's own linter) →
// universal pack (always) → baseline suppression → sort → cap.

const fs = require('fs');
const path = require('path');
const { isExcluded } = require('./config');
const { packFor } = require('./registry');
const { resolveFor, runTool } = require('./resolve');
const { checkUniversal } = require('./universal');
const { loadBaseline, suppress } = require('./baseline');
const { sortFindings, capFindings } = require('./finding');

const MAX_FINDINGS = 5;

function checkFile(absPath, {
  repoRoot, config, maxFindings = MAX_FINDINGS, applyBaseline = true,
} = {}) {
  const relPath = path.relative(repoRoot, absPath).replace(/\\/g, '/');

  if (isExcluded(config, relPath)) {
    return { relPath, findings: [], skipped: 'excluded' };
  }

  let source;
  try {
    source = fs.readFileSync(absPath, 'utf8');
  } catch (e) {
    return { relPath, findings: [], skipped: 'unreadable' };
  }

  const findings = [];

  // Prefer the project's own linter; fall back to the built-in pack.
  const tool = resolveFor(relPath, { repoRoot });
  if (tool) {
    findings.push(...runTool(tool, { repoRoot, absPath }));
  } else {
    const pack = packFor(relPath);
    if (pack) findings.push(...pack.check(source, { relPath, config }));
  }

  // The universal pack runs regardless: no linter checks for credentials in
  // source, PII in logs, or a deprecation with no removal trigger.
  findings.push(...checkUniversal(source, { relPath, config }));

  const lines = source.split(/\r?\n/);
  const kept = applyBaseline
    ? suppress(findings, { baseline: loadBaseline(repoRoot, config), relPath, lines })
    : findings;

  return {
    relPath,
    findings: capFindings(sortFindings(kept), maxFindings),
    skipped: null,
  };
}

module.exports = { checkFile, MAX_FINDINGS };
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/run.test.js`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/run.js tests/run.test.js
git commit -m "feat: check orchestration with tooling preference and ratchet"
```

---

## Task 15: PostToolUse hook and the CLI

**Files:**
- Create: `hooks/procoder-check.js`, `bin/procoder.js`
- Modify: `package.json` (add `"bin": { "procoder": "./bin/procoder.js" }`)
- Test: `tests/check-hook.test.js`, `tests/cli.test.js`

**Interfaces:**
- Consumes: `run.checkFile`, `config.loadConfig`, `config.findRepoRoot`, `finding.formatFindings`, `procoder-runtime` (`readHookInput`, `writeHookOutput`, `readLevel`), `baseline`.
- Produces: the PostToolUse entry point, and a `procoder` CLI with `check <paths...>` and `baseline` subcommands for `/procoder-guard` and CI to call.

- [ ] **Step 1: Write the failing test**

```js
// tests/check-hook.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const HOOK = path.join(__dirname, '..', 'hooks', 'procoder-check.js');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-hook-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

function runHook(repo, filePath, env = {}) {
  const stdout = execFileSync('node', [HOOK], {
    encoding: 'utf8',
    cwd: repo,
    input: JSON.stringify({
      tool_name: 'Write',
      tool_input: { file_path: filePath },
      cwd: repo,
    }),
    env: { ...process.env, CLAUDE_CONFIG_DIR: repo, ...env },
  });
  return stdout.trim() ? JSON.parse(stdout) : {};
}

test('emits findings as PostToolUse additionalContext', () => {
  const repo = repoWith({ 'a.ts': 'el.innerHTML = danger;\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.strictEqual(out.hookSpecificOutput.hookEventName, 'PostToolUse');
  assert.match(out.hookSpecificOutput.additionalContext, /SAFE/);
  assert.match(out.hookSpecificOutput.additionalContext, /a\.ts:1/);
});

test('emits nothing for a clean file', () => {
  const repo = repoWith({ 'a.ts': 'el.textContent = safe;\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.ok(!out.hookSpecificOutput || !out.hookSpecificOutput.additionalContext);
});

test('never blocks — no decision or permission field is ever emitted', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'));
  assert.strictEqual(out.decision, undefined);
  assert.strictEqual(out.permissionDecision, undefined);
  assert.ok(!out.hookSpecificOutput.permissionDecision);
});

test('PROCODER_NO_HOOK disables the hook', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  const out = runHook(repo, path.join(repo, 'a.ts'), { PROCODER_NO_HOOK: '1' });
  assert.deepStrictEqual(out, {});
});

test('malformed input exits cleanly', () => {
  const repo = repoWith({});
  assert.doesNotThrow(() => execFileSync('node', [HOOK], {
    encoding: 'utf8', cwd: repo, input: 'not json',
    env: { ...process.env, CLAUDE_CONFIG_DIR: repo },
  }));
});

test('the hook completes within its 2s budget on a large file', () => {
  const repo = repoWith({ 'big.ts': 'const x = 1;\n'.repeat(20000) });
  const started = Date.now();
  runHook(repo, path.join(repo, 'big.ts'));
  assert.ok(Date.now() - started < 2000, 'hook exceeded its budget');
});
```

```js
// tests/cli.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const CLI = path.join(__dirname, '..', 'bin', 'procoder.js');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-cli-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

function cli(repo, args) {
  try {
    return { code: 0, out: execFileSync('node', [CLI, ...args], { cwd: repo, encoding: 'utf8' }) };
  } catch (e) {
    return { code: e.status, out: String(e.stdout || '') + String(e.stderr || '') };
  }
}

test('check exits non-zero and prints findings for a dirty file', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  const result = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /SAFE/);
});

test('check exits 0 for a clean file', () => {
  const repo = repoWith({ 'a.ts': 'const x = 1;\n' });
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('baseline records findings, after which check passes', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  assert.strictEqual(cli(repo, ['baseline', 'a.ts']).code, 0);
  assert.ok(fs.existsSync(path.join(repo, '.procoder-baseline.json')));
  assert.strictEqual(cli(repo, ['check', 'a.ts']).code, 0);
});

test('a NEW violation still fails after a baseline exists', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\n');
  const result = cli(repo, ['check', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /xss|innerHTML|SAFE/i);
});

test('verify passes when the baseline has not grown', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  assert.strictEqual(cli(repo, ['verify', 'a.ts']).code, 0);
});

test('verify fails when new violations push the count above the baseline', () => {
  const repo = repoWith({ 'a.ts': 'eval(x);\n' });
  cli(repo, ['baseline', 'a.ts']);
  fs.writeFileSync(path.join(repo, 'a.ts'), 'eval(x);\nel.innerHTML = y;\ndebugger;\n');
  const result = cli(repo, ['verify', 'a.ts']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /grew|grow/i);
});

test('unknown subcommand prints usage and exits non-zero', () => {
  const repo = repoWith({});
  const result = cli(repo, ['frobnicate']);
  assert.notStrictEqual(result.code, 0);
  assert.match(result.out, /usage/i);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/check-hook.test.js tests/cli.test.js`
Expected: FAIL — neither entry point exists.

- [ ] **Step 3: Write the implementations**

```js
#!/usr/bin/env node
// procoder — PostToolUse hook.
//
// Runs on the single file Claude just wrote. Emits findings as context, never
// a block: the model fixes them in the same turn, which is cheaper than a
// rejected write and does not strand the user behind a false positive.

const path = require('path');
const { loadConfig, findRepoRoot } = require('./checks/config');
const { checkFile } = require('./checks/run');
const { formatFindings } = require('./checks/finding');
const { readHookInput, writeHookOutput, readLevel } = require('./procoder-runtime');

if (process.env.PROCODER_NO_HOOK === '1') process.exit(0);

readHookInput().then((input) => {
  const level = readLevel();
  if (level === 'off') return;

  const filePath = (input.tool_input && input.tool_input.file_path) || '';
  if (!filePath) return;

  const absPath = path.isAbsolute(filePath)
    ? filePath
    : path.resolve(input.cwd || process.cwd(), filePath);

  const repoRoot = findRepoRoot(path.dirname(absPath));
  const config = loadConfig(repoRoot);

  const { relPath, findings, skipped } = checkFile(absPath, { repoRoot, config });
  if (skipped || findings.length === 0) return;

  const header = `procoder [${level}] — ${findings.length} finding${findings.length === 1 ? '' : 's'} in ${relPath}. Fix these before moving on:`;
  writeHookOutput('PostToolUse', level, header + '\n' + formatFindings(findings, relPath));
}).catch(() => process.exit(0));
```

```js
#!/usr/bin/env node
// procoder — CLI for pre-commit hooks and CI. Same engine as the PostToolUse
// hook, so what fails locally fails in CI for the same reason.
//
// Usage:
//   procoder check <paths...>     exit 1 if any non-baselined finding exists
//   procoder baseline <paths...>  record current findings as accepted

const fs = require('fs');
const path = require('path');
const { loadConfig, findRepoRoot } = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { formatFindings } = require('../hooks/checks/finding');
const { fingerprint, writeBaseline, loadBaseline, growthCheck } = require('../hooks/checks/baseline');

const USAGE = `usage: procoder <check|baseline|verify> <paths...>

  check     report findings not present in the baseline; exit 1 if any
  baseline  record every current finding as accepted, so only new code is gated
  verify    exit 1 only if total findings exceed the baseline — the CI ratchet
`;

function expand(targets) {
  const files = [];
  for (const target of targets) {
    const abs = path.resolve(target);
    let stat;
    try { stat = fs.statSync(abs); } catch (e) { continue; }
    if (stat.isDirectory()) {
      for (const entry of fs.readdirSync(abs)) {
        if (entry === '.git' || entry === 'node_modules') continue;
        files.push(...expand([path.join(abs, entry)]));
      }
    } else {
      files.push(abs);
    }
  }
  return files;
}

function main(argv) {
  const [command, ...targets] = argv;
  if (!['check', 'baseline', 'verify'].includes(command) || targets.length === 0) {
    process.stderr.write(USAGE);
    return 2;
  }

  const files = expand(targets);
  if (files.length === 0) return 0;

  const repoRoot = findRepoRoot(path.dirname(files[0]));
  const config = loadConfig(repoRoot);

  if (command === 'baseline') {
    const entries = Array.from(loadBaseline(repoRoot, config));
    for (const absPath of files) {
      // maxFindings Infinity: a baseline must record everything, not a top-5 sample.
      const { relPath, findings, skipped } = checkFile(absPath, {
        repoRoot, config, maxFindings: Infinity,
      });
      if (skipped) continue;
      const lines = fs.readFileSync(absPath, 'utf8').split(/\r?\n/);
      for (const f of findings) entries.push(fingerprint(f, relPath, lines[f.line - 1]));
    }
    writeBaseline(repoRoot, config, entries);
    process.stdout.write(`procoder: baseline recorded (${entries.length} accepted findings)\n`);
    return 0;
  }

  if (command === 'verify') {
    // The ratchet: accepted debt may shrink, never grow. Counts findings BEFORE
    // baseline suppression, so a fix and a fresh violation do not cancel out.
    const baseline = loadBaseline(repoRoot, config);
    let total = 0;
    for (const absPath of files) {
      const { findings, skipped } = checkFile(absPath, {
        repoRoot, config, maxFindings: Infinity, applyBaseline: false,
      });
      if (!skipped) total += findings.length;
    }
    const { ok, delta } = growthCheck(baseline, total);
    if (!ok) {
      process.stdout.write(
        `procoder: findings grew by ${delta} beyond the baseline (${baseline.size} accepted, ${total} present).\n` +
        'Fix the new findings, or run `procoder baseline <paths>` only if they are genuinely pre-existing.\n');
      return 1;
    }
    process.stdout.write(
      `procoder: ${total} findings against a baseline of ${baseline.size} — ratchet holds.\n`);
    return 0;
  }

  let total = 0;
  for (const absPath of files) {
    const { relPath, findings, skipped } = checkFile(absPath, {
      repoRoot, config, maxFindings: Infinity,
    });
    if (skipped || findings.length === 0) continue;
    total += findings.length;
    process.stdout.write(formatFindings(findings, relPath) + '\n');
  }

  if (total > 0) {
    process.stdout.write(`\nprocoder: ${total} finding${total === 1 ? '' : 's'}. ` +
      'Fix them, or run `procoder baseline <paths>` to accept pre-existing ones.\n');
    return 1;
  }
  return 0;
}

process.exit(main(process.argv.slice(2)));
```

Add to `package.json`: `"bin": { "procoder": "./bin/procoder.js" }`.

- [ ] **Step 4: Run test to verify it passes**

Run: `chmod +x bin/procoder.js && node --test tests/check-hook.test.js tests/cli.test.js`
Expected: PASS (6 + 5 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/procoder-check.js bin/procoder.js package.json \
        tests/check-hook.test.js tests/cli.test.js
git commit -m "feat: PostToolUse check hook and the procoder CLI"
```

---

## Task 16: Dogfood — run procoder on procoder

**Files:**
- Modify: `.github/workflows/ci.yml` (add the self-check step)
- Test: `tests/dogfood.test.js`

**Interfaces:**
- Consumes: `bin/procoder.js`.
- Produces: nothing further; this task closes Plan 2.

- [ ] **Step 1: Write the failing test**

```js
// tests/dogfood.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const path = require('path');

const root = path.join(__dirname, '..');
const CLI = path.join(root, 'bin', 'procoder.js');

test('procoder reports no findings against its own source', () => {
  let out = '';
  let code = 0;
  try {
    out = execFileSync('node', [CLI, 'check', 'hooks', 'bin', 'scripts'],
      { cwd: root, encoding: 'utf8' });
  } catch (e) {
    code = e.status;
    out = String(e.stdout || '');
  }
  assert.strictEqual(code, 0,
    `procoder fails its own rungs:\n${out}\nFix the source, do not baseline it.`);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/dogfood.test.js`
Expected: FAIL — the packs contain credential-shaped regex literals and long
functions, which the universal and shape checks flag.

- [ ] **Step 3: Fix the source until it passes**

Work through the reported findings honestly. The two legitimate categories and
their fixes:

- **Pattern literals that look like credentials** (`SECRET_PATTERNS`, the
  `CREDENTIAL_ASSIGN` regex). These are detection patterns, not credentials.
  Add `hooks/checks/**` to the `[exclude]` list for `safe/hardcoded-secret`
  only — do this by moving the pattern tables into
  `hooks/checks/patterns/secrets.js` and excluding that single file in
  `.procoder.toml`, rather than excluding the whole check directory.
- **Everything else** — long `check()` functions, deep nesting, orphan TODOs —
  is a real finding. Fix the code. Do not baseline it: a tool that exempts
  itself from its own rung 4 has already lost the argument.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test`
Expected: PASS — every suite from Tasks 1–16 green, including dogfood.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: procoder passes its own rungs"
```

Then add to `.github/workflows/ci.yml`, after the existing `npm test` step:

```yaml
      - name: procoder checks itself
        run: node bin/procoder.js check hooks bin scripts
```

---

## Done when

- `npm test` passes, including `tests/dogfood.test.js`.
- Writing a file with `eval(x)` in a live session produces a `[1 SAFE]` finding in
  context within the 2s budget, and Claude fixes it in the same turn.
- In a repo with `.eslintrc` present, the hook reports eslint's findings rather
  than the built-in TypeScript pack's.
- `procoder baseline .` on a legacy repo silences existing findings, while a
  newly introduced violation in the same file still reports.

**Next:** Plan 3 — the command skills (`review`, `audit`, `rot`, `threat`, `deps`,
`debt`, `gain`, `guard`), the MCP server, examples, and install docs.
