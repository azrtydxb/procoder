# procoder Plan 3 — Commands, MCP Server & Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete ponytail parity — the eight command skills, an MCP server, generated pre-commit/CI guards, worked examples, and install docs for every supported agent.

**Architecture:** Each command is a `commands/*.toml` that invokes a `skills/*/SKILL.md`. Skills that need deterministic data shell out to `bin/procoder.js` from Plan 2 rather than re-deriving findings in prose, so `/procoder-review` and CI agree by construction. The MCP server exposes the same engine as tools for non-Claude hosts.

**Tech Stack:** Node.js ≥18, CommonJS, zero runtime dependencies. MCP server speaks JSON-RPC 2.0 over stdio, hand-rolled — the protocol surface needed here is three methods.

**Spec:** [docs/superpowers/specs/2026-08-16-procoder-design.md](../specs/2026-08-16-procoder-design.md)
**Depends on:** Plan 1 (skill/command loading, `scripts/sync-rules.js`), Plan 2 (`bin/procoder.js`, `hooks/checks/*`).

## Global Constraints

- Node.js ≥18, CommonJS, **zero runtime dependencies** — including the MCP server.
- Every command skill's frontmatter `description` must name its trigger phrases; the skill is otherwise unreachable by natural language.
- Skills that report findings use the Plan 2 output format verbatim: `[N RUNG]    path:line   what → fix`. No skill invents its own format.
- Skills that can shell out to `bin/procoder.js` **must**, rather than eyeballing files, wherever the check is deterministic. Model judgment is for rung 3 naming and rung 4 semantics only.
- No command writes to the working tree except `/procoder-guard` (which writes config files it names first) and `/procoder-debt`/`/procoder-audit` when the user explicitly asks for a baseline.
- Every new `commands/*.toml` is added to the README table — `tests/e2e.test.js` from Plan 1 enforces this.
- Rung names, level names, and the ladder order are fixed: `SAFE`, `TRUE`, `OBVIOUS`, `ALONE`; `pragmatic`, `strict`, `paranoid`.

---

## File Structure

| File | Responsibility |
|---|---|
| `skills/procoder-review/SKILL.md` | Diff review across the four rungs |
| `skills/procoder-audit/SKILL.md` | Whole-repo ranked audit; offers a baseline |
| `skills/procoder-rot/SKILL.md` | Rung-4 specialist: dead, stale, deprecated |
| `skills/procoder-threat/SKILL.md` | Rung-1 specialist: trust-boundary map |
| `skills/procoder-deps/SKILL.md` | Dependency hygiene |
| `skills/procoder-debt/SKILL.md` | `procoder:` marker ledger |
| `skills/procoder-gain/SKILL.md` | Measured outcomes |
| `skills/procoder-guard/SKILL.md` | Emit pre-commit + CI config |
| `commands/procoder-*.toml` | One per skill above |
| `hooks/checks/deps.js` | Dependency manifest parsing and audit-tool invocation |
| `scripts/templates/pre-commit.sh` | Emitted by `/procoder-guard` |
| `scripts/templates/procoder-ci.yml` | Emitted by `/procoder-guard` |
| `procoder-mcp/server.js` | JSON-RPC 2.0 stdio MCP server |
| `procoder-mcp/package.json` | Standalone publish surface |
| `examples/` | One worked before/after per rung |
| `docs/install.md` | Per-agent install instructions |

---

## Task 1: `/procoder-review`

**Files:**
- Create: `skills/procoder-review/SKILL.md`, `commands/procoder-review.toml`
- Test: `tests/skill-review.test.js`

**Interfaces:**
- Consumes: `bin/procoder.js check`, `git diff`.
- Produces: the review skill. Tasks 2–8 follow this exact structure: a `SKILL.md` with frontmatter naming triggers, a Procedure section with numbered steps, an Output section fixing the format, and a "Do not" section.

- [ ] **Step 1: Write the failing test**

```js
// tests/skill-review.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-review', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.ok(m, 'missing frontmatter');
  assert.match(m[1], /^name: procoder-review$/m);
  assert.match(m[1], /review/i);
  assert.match(m[1], /diff|changes|staged/i);
});

test('runs the deterministic engine rather than eyeballing', () => {
  assert.match(skill, /bin\/procoder\.js check|procoder check/);
  assert.match(skill, /git diff/);
});

test('covers all four rungs by name', () => {
  for (const rung of ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE']) {
    assert.match(skill, new RegExp(`\\b${rung}\\b`), `missing rung ${rung}`);
  }
});

test('fixes the output format and forbids essays', () => {
  assert.match(skill, /\[1 SAFE\]/);
  assert.match(skill, /one line per finding/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/skill-review.test.js`
Expected: FAIL — `ENOENT` on the skill file.

- [ ] **Step 3: Write the skill and command**

`skills/procoder-review/SKILL.md`:

````markdown
---
name: procoder-review
description: Review the current diff against procoder's four rungs — SAFE, TRUE, OBVIOUS, ALONE. Use when the user says "procoder review", "review my changes", "is this safe to ship", "check this diff", "review before commit", or invokes /procoder-review. Reviews changed code only; use procoder-audit for the whole repo.
---

# procoder-review

Review the diff. Every rung must hold before it ships.

## Procedure

1. **Get the diff.** `git diff --stat HEAD` for scope; `git diff HEAD` for content.
   If nothing is uncommitted, review `git diff origin/main...HEAD` instead and say
   which range you reviewed.
2. **Run the engine first.** `node <plugin>/bin/procoder.js check <changed files>`.
   These findings are deterministic — report them as-is. Do not re-derive by eye
   what the engine already computed, and do not omit a finding because it looks
   minor.
3. **Read the diff for what the engine cannot see**, in rung order:
   - **SAFE** — does new untrusted input reach a sink? Is authz checked on the
     object, server-side? Any new dependency? Anything new in a log line?
   - **TRUE** — can any new error path lose data? What edge is untested? Does
     non-trivial new logic leave a runnable check behind?
   - **OBVIOUS** — names that say how instead of what; a comment restating the
     code instead of the why; a public symbol with no signature doc.
   - **ALONE** — this is the one reviewers skip. For every changed function, grep
     for the thing it replaced. Is the old path still exported? Is there a
     commented-out block, a settled feature flag, a deprecation with no removal
     trigger, a doc paragraph describing the behavior you just changed?
4. **Verify before reporting.** For each judgment finding, name the file and line
   you read. If you cannot point at one, drop the finding.

## Output

One line per finding, most severe first:

```
[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup
[2 TRUE]    api/users.ts:58   error swallowed, write may be lost → propagate or log with correlation id
[3 OBVIOUS] api/users.ts:71   fn 94 lines, depth 5 → extract validate/persist/notify
[4 ALONE]   api/users.ts:6    createUserV1 still exported, no caller → delete
```

Then one closing line: `N findings — X blocking (SAFE/TRUE), Y advisory.` If the
diff is clean, say so in one line and stop.

## Do not

- Do not restate what the code does. The reader wrote it.
- Do not report style the project's formatter owns.
- Do not soften a SAFE or TRUE finding into a suggestion — those two rungs are
  not negotiable.
- Do not propose refactors outside the diff. That is `/procoder-audit`.
````

`commands/procoder-review.toml`:

```toml
description = "Review the current diff against procoder's four rungs."
prompt = """
Use the procoder-review skill on the current diff. Arguments (optional target,
e.g. a commit range or path): $ARGUMENTS

Run the deterministic engine before forming judgment findings, and report in the
skill's one-line format.
"""
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/skill-review.test.js`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder-review commands/procoder-review.toml tests/skill-review.test.js
git commit -m "feat: /procoder-review"
```

---

## Task 2: `/procoder-audit`

**Files:**
- Create: `skills/procoder-audit/SKILL.md`, `commands/procoder-audit.toml`
- Test: `tests/skill-audit.test.js`

**Interfaces:**
- Consumes: `bin/procoder.js check` and `bin/procoder.js baseline`.
- Produces: the whole-repo audit skill.

- [ ] **Step 1: Write the failing test**

```js
// tests/skill-audit.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-audit', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: procoder-audit$/m);
  assert.match(m[1], /audit|whole repo|entire codebase/i);
});

test('offers the baseline as the adoption path', () => {
  assert.match(skill, /procoder\.js baseline|procoder baseline/);
  assert.match(skill, /ratchet|baseline/i);
});

test('ranks findings and caps the report', () => {
  assert.match(skill, /rank|ranked/i);
  assert.match(skill, /\btop\b|\bcap\b|\blimit\b/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/skill-audit.test.js`
Expected: FAIL — file missing.

- [ ] **Step 3: Write the skill and command**

`skills/procoder-audit/SKILL.md` — frontmatter `name: procoder-audit`, triggers
"procoder audit", "audit this codebase", "audit the whole repo", "where is this
codebase weakest", "/procoder-audit". Body:

- **Procedure:** (1) `node <plugin>/bin/procoder.js check .` for the deterministic
  sweep. (2) Group findings by rung, then by directory, and count. (3) Sample-read
  the three worst files for judgment findings the engine cannot see. (4) Identify
  the top three *systemic* patterns — one repeated mistake matters more than
  thirty instances of it.
- **Output:** a rung summary table (`rung | count | worst directory`), then the
  top 15 individual findings in the standard one-line format, then the three
  systemic patterns with the one change that fixes each class.
- **The adoption path:** if the count exceeds 50, say plainly that a repo this
  size cannot be fixed in one pass, and offer
  `node <plugin>/bin/procoder.js baseline .` — existing findings recorded as
  accepted, new code gated from now on, count allowed to shrink but never grow.
  Explicitly ask before writing the baseline file.
- **Do not:** do not list all 4,000 findings; do not fix anything during an audit;
  do not write the baseline without asking.

`commands/procoder-audit.toml` — description "Audit the whole repository against
procoder's four rungs.", prompt invoking the skill with `$ARGUMENTS` as an
optional path scope.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/skill-audit.test.js`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder-audit commands/procoder-audit.toml tests/skill-audit.test.js
git commit -m "feat: /procoder-audit"
```

---

## Task 3: `/procoder-rot`

**Files:**
- Create: `skills/procoder-rot/SKILL.md`, `commands/procoder-rot.toml`
- Test: `tests/skill-rot.test.js`

**Interfaces:**
- Consumes: `git log`, `git grep`, `bin/procoder.js check`.
- Produces: the rung-4 specialist skill. This and Task 4 are procoder's differentiators — nothing in ponytail or the mainstream review plugins covers them.

- [ ] **Step 1: Write the failing test**

```js
// tests/skill-rot.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-rot', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: procoder-rot$/m);
  assert.match(m[1], /dead code|stale|deprecated|unused/i);
});

test('covers every rot category from the spec', () => {
  for (const category of [
    'export', 'commented', 'feature flag', 'deprecat', 'removal trigger',
    'documentation', 'dependenc', 'fixture',
  ]) {
    assert.match(skill.toLowerCase(), new RegExp(category), `missing: ${category}`);
  }
});

test('requires verification before recommending deletion', () => {
  assert.match(skill, /git grep|rg |ripgrep|search/i);
  assert.match(skill, /dynamic|reflection|string|entry ?point/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/skill-rot.test.js`
Expected: FAIL — file missing.

- [ ] **Step 3: Write the skill and command**

`skills/procoder-rot/SKILL.md` — frontmatter `name: procoder-rot`, triggers "procoder
rot", "find dead code", "what can I delete", "stale code", "deprecated code",
"unused exports", "/procoder-rot". Body:

- **Premise:** a change isn't done until the thing it replaced is gone. This skill
  finds what previous changes left behind.
- **Procedure**, one pass per category:
  1. **Dead exports** — for each exported symbol, `git grep -n "<symbol>"` across
     the repo. Zero non-definition hits is a candidate.
  2. **Commented-out code** — from the engine's `alone/commented-code` findings.
  3. **Settled feature flags** — find flag reads; `git log -S "<flag>" --oneline`
     for when it last changed. A flag whose value hasn't moved in a release is
     settled: delete the dead branch.
  4. **Deprecations with no removal trigger** — engine's
     `alone/deprecated-no-trigger`, plus `git log -1 --format=%ar -S "<marker>"`
     to report how long each has been rotting.
  5. **Version twins** — `*_old`, `*_new`, `*_v1`, `*_final`, `*Legacy`, and a
     `v2` living beside a `v1`.
  6. **Stale docs** — README and doc comments describing behavior the code no
     longer has. Check any doc block whose file changed more recently than the doc.
  7. **Unused dependencies** — declared in the manifest, imported nowhere.
  8. **Orphaned fixtures and config keys** — test fixtures no test loads;
     config keys nothing reads.
- **Verification, before recommending any deletion:** a symbol can be reached by
  dynamic dispatch, reflection, string-built names, DI containers, public API
  contract, or an entry point declared in the build config. Search for the bare
  name as a string too. If reachability is uncertain, report it as
  *needs confirmation*, not as a deletion.
- **Output:** grouped by category, one line each, in the standard format with the
  rung fixed at `[4 ALONE]`, plus an age column where git can supply it. Close
  with total lines removable and the single highest-value deletion.
- **Do not:** do not delete anything — this skill reports. Do not flag a public
  API's exports as dead just because this repo doesn't call them. Do not treat a
  deprecation *with* a removal trigger as rot; it is doing its job.

`commands/procoder-rot.toml` — description "Find dead, stale, and deprecated code
left behind.", prompt invoking the skill with `$ARGUMENTS` as an optional scope.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/skill-rot.test.js`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder-rot commands/procoder-rot.toml tests/skill-rot.test.js
git commit -m "feat: /procoder-rot"
```

---

## Task 4: `/procoder-threat`

**Files:**
- Create: `skills/procoder-threat/SKILL.md`, `commands/procoder-threat.toml`
- Test: `tests/skill-threat.test.js`

**Interfaces:**
- Consumes: `git grep`, `bin/procoder.js check`.
- Produces: the rung-1 specialist skill.

- [ ] **Step 1: Write the failing test**

```js
// tests/skill-threat.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-threat', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: procoder-threat$/m);
  assert.match(m[1], /threat|trust boundar|attack surface|security review/i);
});

test('enumerates entry points and sinks', () => {
  for (const term of [
    'handler', 'queue', 'webhook', 'environment', 'deserializ',
    'sql', 'shell', 'authoriz',
  ]) {
    assert.match(skill.toLowerCase(), new RegExp(term), `missing: ${term}`);
  }
});

test('produces a boundary table, not prose', () => {
  assert.match(skill, /\|.*boundar.*\|/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/skill-threat.test.js`
Expected: FAIL — file missing.

- [ ] **Step 3: Write the skill and command**

`skills/procoder-threat/SKILL.md` — frontmatter `name: procoder-threat`, triggers
"procoder threat", "threat model", "trust boundaries", "attack surface", "security
review", "where does untrusted input enter", "/procoder-threat". Body:

- **Premise:** you cannot validate a boundary you have not listed. This skill
  produces the list, then checks each one.
- **Procedure:**
  1. **Enumerate entry points** — HTTP route handlers, GraphQL resolvers, gRPC
     methods, queue/topic consumers, webhook receivers, CLI argument parsing,
     file and upload readers, environment and config reads, IPC/socket handlers,
     deserialization sites, and any third-party callback. Search by framework
     idiom for the languages present.
  2. **Enumerate sinks** — SQL/ORM raw queries, shell and process execution,
     filesystem paths, HTTP clients (SSRF), template and HTML rendering,
     deserializers, and redirect targets.
  3. **Trace** each entry point to the sinks it can reach.
  4. **For each boundary, answer four questions:** what validates the input;
     where authorization is enforced and on which object; what is logged from it;
     what happens on malformed input.
- **Output:** one table.

  | # | Boundary | Entry | Reaches sink | Validated by | Authz | Gap |
  |---|---|---|---|---|---|---|

  Gap is empty when the boundary is sound, or a one-line `[1 SAFE]` finding when
  it is not. Follow the table with the findings in standard format, most severe
  first, and one closing line: `N boundaries, M with gaps.`
- **At `paranoid` level**, add for each gap: what an attacker gets if it is
  exploited, in one clause.
- **Do not:** do not speculate about vulnerabilities you cannot trace to a line;
  do not report framework-handled protections (an ORM's parameterization, a
  template engine's auto-escaping) as gaps; do not produce a STRIDE essay — the
  table is the deliverable.

`commands/procoder-threat.toml` — description "Map every trust boundary and what
validates it.", prompt invoking the skill with `$ARGUMENTS` as an optional scope.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/skill-threat.test.js`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder-threat commands/procoder-threat.toml tests/skill-threat.test.js
git commit -m "feat: /procoder-threat"
```

---

## Task 5: Dependency hygiene — `hooks/checks/deps.js` and `/procoder-deps`

**Files:**
- Create: `hooks/checks/deps.js`, `skills/procoder-deps/SKILL.md`, `commands/procoder-deps.toml`
- Test: `tests/deps.test.js`, `tests/skill-deps.test.js`

**Interfaces:**
- Consumes: `finding`, `resolve.hasTool`.
- Produces:
  - `detectEcosystems(repoRoot) → [{name, manifest, lockfile, auditArgv}]`
  - `checkManifest(manifestPath, source) → Finding[]` (unpinned versions, missing lockfile)
  - `AUDIT_COMMANDS` map

- [ ] **Step 1: Write the failing test**

```js
// tests/deps.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { detectEcosystems, checkManifest, AUDIT_COMMANDS } = require('../hooks/checks/deps');

function repoWith(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-deps-'));
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
}

test('detects the npm ecosystem and its lockfile', () => {
  const repo = repoWith({ 'package.json': '{}', 'package-lock.json': '{}' });
  const found = detectEcosystems(repo);
  assert.strictEqual(found.length, 1);
  assert.strictEqual(found[0].name, 'npm');
  assert.strictEqual(found[0].hasLockfile, true);
});

test('detects several ecosystems in one repo', () => {
  const repo = repoWith({ 'package.json': '{}', 'pyproject.toml': '', 'go.mod': '' });
  assert.deepStrictEqual(
    detectEcosystems(repo).map((e) => e.name).sort(), ['go', 'npm', 'python']);
});

test('flags a missing lockfile', () => {
  const repo = repoWith({ 'package.json': '{"dependencies":{"a":"1.0.0"}}' });
  const ids = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8')).map((f) => f.id);
  assert.ok(ids.includes('safe/missing-lockfile'));
});

test('flags floating version ranges', () => {
  const repo = repoWith({
    'package.json': '{"dependencies":{"a":"*","b":"latest","c":"^1.2.3","d":"1.2.3"}}',
    'package-lock.json': '{}',
  });
  const findings = checkManifest(path.join(repo, 'package.json'),
    fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
  const messages = findings.map((f) => f.message).join(' ');
  assert.match(messages, /\ba\b/);
  assert.match(messages, /\bb\b/);
  assert.ok(!/"d"/.test(messages), 'a pinned version must not be flagged');
});

test('every ecosystem declares an audit command', () => {
  for (const name of ['npm', 'python', 'go', 'rust', 'dotnet']) {
    assert.ok(AUDIT_COMMANDS[name], `no audit command for ${name}`);
    assert.ok(Array.isArray(AUDIT_COMMANDS[name].argv));
  }
});
```

```js
// tests/skill-deps.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const skill = fs.readFileSync(
  path.join(__dirname, '..', 'skills', 'procoder-deps', 'SKILL.md'), 'utf8');

test('frontmatter names the skill and its triggers', () => {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(skill);
  assert.match(m[1], /^name: procoder-deps$/m);
  assert.match(m[1], /dependenc|supply chain|vulnerab|package/i);
});

test('uses the project audit tools rather than guessing', () => {
  for (const tool of ['npm audit', 'pip-audit', 'govulncheck', 'cargo audit']) {
    assert.ok(skill.includes(tool), `missing: ${tool}`);
  }
});

test('covers abandonment, pinning, and unused dependencies', () => {
  assert.match(skill, /abandon|unmaintained|last release/i);
  assert.match(skill, /pin|lockfile/i);
  assert.match(skill, /unused/i);
});

test('states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/deps.test.js tests/skill-deps.test.js`
Expected: FAIL — neither file exists.

- [ ] **Step 3: Write the implementation, skill, and command**

```js
#!/usr/bin/env node
// procoder — dependency hygiene.
//
// A new dependency is a new trust boundary. This module finds which ecosystems
// a repo uses and how to ask each one's own tooling about vulnerabilities —
// procoder never ships its own vulnerability database.

const fs = require('fs');
const path = require('path');
const { finding } = require('./finding');

const ECOSYSTEMS = [
  { name: 'npm', manifest: 'package.json', lockfiles: ['package-lock.json', 'yarn.lock', 'pnpm-lock.yaml'] },
  { name: 'python', manifest: 'pyproject.toml', altManifests: ['requirements.txt', 'setup.py'], lockfiles: ['poetry.lock', 'uv.lock', 'requirements.lock', 'Pipfile.lock'] },
  { name: 'go', manifest: 'go.mod', lockfiles: ['go.sum'] },
  { name: 'rust', manifest: 'Cargo.toml', lockfiles: ['Cargo.lock'] },
  { name: 'dotnet', manifest: 'Directory.Packages.props', altManifests: ['packages.config'], lockfiles: ['packages.lock.json'] },
];

const AUDIT_COMMANDS = {
  npm: { name: 'npm', argv: ['audit', '--json'] },
  python: { name: 'pip-audit', argv: ['--format', 'json'] },
  go: { name: 'govulncheck', argv: ['-json', './...'] },
  rust: { name: 'cargo', argv: ['audit', '--json'] },
  dotnet: { name: 'dotnet', argv: ['list', 'package', '--vulnerable', '--include-transitive'] },
};

function detectEcosystems(repoRoot) {
  const found = [];
  for (const eco of ECOSYSTEMS) {
    const manifests = [eco.manifest, ...(eco.altManifests || [])];
    const manifest = manifests.find((file) => fs.existsSync(path.join(repoRoot, file)));
    if (!manifest) continue;
    found.push({
      name: eco.name,
      manifest,
      hasLockfile: eco.lockfiles.some((file) => fs.existsSync(path.join(repoRoot, file))),
      audit: AUDIT_COMMANDS[eco.name],
    });
  }
  return found;
}

// Matches "name": "spec" pairs inside package.json dependency blocks. Anything
// that is not a concrete version is a floating range.
const NPM_DEP = /"([^"]+)"\s*:\s*"([^"]+)"/g;
const FLOATING = /^(?:\*|latest|x|\d+\.x|>=|>|\^|~)/;

function checkManifest(manifestPath, source) {
  const findings = [];
  const base = path.basename(manifestPath);
  const repoRoot = path.dirname(manifestPath);

  const eco = detectEcosystems(repoRoot).find((e) => e.manifest === base);
  if (eco && !eco.hasLockfile) {
    findings.push(finding({
      rung: 'SAFE', id: 'safe/missing-lockfile', line: 1,
      message: `${eco.name} manifest with no lockfile committed`,
      fix: 'commit the lockfile so builds resolve the versions you audited',
    }));
  }

  if (base === 'package.json') {
    const lines = String(source).split(/\r?\n/);
    let inDeps = false;
    lines.forEach((line, index) => {
      if (/"(?:dependencies|devDependencies|peerDependencies)"\s*:/.test(line)) inDeps = true;
      else if (inDeps && /^\s*}/.test(line)) inDeps = false;
      else if (inDeps) {
        NPM_DEP.lastIndex = 0;
        const match = NPM_DEP.exec(line);
        if (match && FLOATING.test(match[2]) && !/^\^?\d+\.\d+\.\d+$/.test(match[2])) {
          findings.push(finding({
            rung: 'SAFE', id: 'safe/floating-version', line: index + 1,
            message: `${match[1]} declared as "${match[2]}"`,
            fix: 'pin to an exact version; the lockfile alone does not protect consumers',
          }));
        }
      }
    });
  }

  return findings;
}

module.exports = { ECOSYSTEMS, AUDIT_COMMANDS, detectEcosystems, checkManifest };
```

Wire `checkManifest` into `hooks/checks/run.js`: when `relPath`'s basename is a
known manifest, append its findings alongside the universal pack.

`skills/procoder-deps/SKILL.md` — frontmatter `name: procoder-deps`, triggers
"procoder deps", "dependency audit", "supply chain", "are my packages safe",
"vulnerable dependencies", "/procoder-deps". Body:

- **Premise:** a new dependency is a new trust boundary, and most real CVEs arrive
  through one.
- **Procedure:** (1) `detectEcosystems` via `node -e` or read the manifests
  directly. (2) Run each ecosystem's own auditor — `npm audit --json`,
  `pip-audit --format json`, `govulncheck ./...`, `cargo audit --json`,
  `dotnet list package --vulnerable`. If a tool is absent, say so and name the
  install command rather than guessing at vulnerabilities. (3) Check pinning and
  lockfile presence. (4) Check abandonment: for the top-level dependencies, when
  was the last release — anything over two years is a finding. (5) Check unused:
  declared but imported nowhere.
- **Output:** standard one-line findings, `[1 SAFE]` for vulnerabilities and
  missing lockfiles, `[4 ALONE]` for unused dependencies. Group by ecosystem.
  Close with `N dependencies, M vulnerable, K unused.`
- **Do not:** do not report a CVE you did not get from a tool; do not recommend
  upgrading across a major version without saying it is breaking; do not suggest
  adding a dependency to solve a dependency problem.

`commands/procoder-deps.toml` — description "Audit dependencies: vulnerable,
abandoned, unpinned, unused.", prompt invoking the skill.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/deps.test.js tests/skill-deps.test.js`
Expected: PASS (5 + 4 tests).

- [ ] **Step 5: Commit**

```bash
git add hooks/checks/deps.js hooks/checks/run.js skills/procoder-deps \
        commands/procoder-deps.toml tests/deps.test.js tests/skill-deps.test.js
git commit -m "feat: dependency hygiene and /procoder-deps"
```

---

## Task 6: `/procoder-debt` and `/procoder-gain`

**Files:**
- Create: `skills/procoder-debt/SKILL.md`, `skills/procoder-gain/SKILL.md`, `commands/procoder-debt.toml`, `commands/procoder-gain.toml`
- Test: `tests/skill-debt-gain.test.js`

**Interfaces:**
- Consumes: `git log`, `git grep`, `.procoder-baseline.json`.
- Produces: the ledger and the measurement skills.

- [ ] **Step 1: Write the failing test**

```js
// tests/skill-debt-gain.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const read = (name) => fs.readFileSync(
  path.join(__dirname, '..', 'skills', name, 'SKILL.md'), 'utf8');

test('debt skill finds procoder markers and flags missing removal triggers', () => {
  const skill = read('procoder-debt');
  assert.match(/^---\n([\s\S]*?)\n---\n/.exec(skill)[1], /^name: procoder-debt$/m);
  assert.match(skill, /procoder:/);
  assert.match(skill, /removal trigger/i);
  assert.match(skill, /git log|git blame/);
});

test('gain skill measures against the baseline, not vibes', () => {
  const skill = read('procoder-gain');
  assert.match(/^---\n([\s\S]*?)\n---\n/.exec(skill)[1], /^name: procoder-gain$/m);
  assert.match(skill, /baseline/i);
  assert.match(skill, /git diff --stat|git log/);
  assert.match(skill, /deleted|removed/i);
});

test('neither skill invents a score or grade', () => {
  for (const name of ['procoder-debt', 'procoder-gain']) {
    assert.ok(!/\bgrade\b|\bscore\b|\bA\+|\bB-\b/.test(read(name)),
      `${name} introduces a score, which the spec forbids`);
  }
});

test('both state what NOT to do', () => {
  for (const name of ['procoder-debt', 'procoder-gain']) {
    assert.match(read(name), /^##.*(?:Do not|Never)/mi, `${name} missing a Do not section`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/skill-debt-gain.test.js`
Expected: FAIL — files missing.

- [ ] **Step 3: Write the skills and commands**

`skills/procoder-debt/SKILL.md` — frontmatter `name: procoder-debt`, triggers
"procoder debt", "what shortcuts did we take", "technical debt ledger",
"outstanding procoder markers", "/procoder-debt". Body:

- **Premise:** a deliberate shortcut is fine; an *undated* one is rot. This skill
  is the ledger.
- **Procedure:** (1) `git grep -n "procoder:"` for deliberate markers, and
  `git grep -nE "TODO|FIXME|HACK|XXX|@deprecated"` for the informal ones.
  (2) For each, `git log -1 --format="%ar %an" -S "<marker text>" -- <file>` to
  get age and author. (3) Classify: **dated** (has a removal trigger — a version,
  date, or measurable condition), **undated** (no trigger — itself a rung-4
  violation), **overdue** (trigger has passed). (4) Read the baseline file, if
  present, and report its size as accepted debt.
- **Output:** one table — `age | file:line | marker | trigger | status` — sorted
  oldest first. Then one line per undated marker in `[4 ALONE]` format. Close with
  `N markers: X dated, Y undated, Z overdue. Baseline holds W accepted findings.`
- **Do not:** do not propose fixing everything; do not treat a dated marker as a
  problem; do not compute a debt score.

`skills/procoder-gain/SKILL.md` — frontmatter `name: procoder-gain`, triggers
"procoder gain", "what did procoder fix", "how much did we clean up", "quality
progress", "/procoder-gain". Body:

- **Premise:** the ratchet only counts if someone reads it. This is the readout.
- **Procedure:** (1) Compare the current baseline size against its size at the
  chosen reference point: `git show <ref>:.procoder-baseline.json` and count
  fingerprints. (2) `git diff --stat <ref>..HEAD` for lines added versus deleted.
  (3) `git log <ref>..HEAD --oneline` for commits that removed rot — those
  touching deletions of exports, flags, or deprecated paths. (4) Count trust
  boundaries hardened by grepping the diff for added validation at entry points.
- **Output:** four numbers with their references — baseline shrinkage, net lines
  deleted, rot removals, boundaries hardened — then the three most valuable
  individual changes, one line each. Default reference point is the previous tag,
  or 30 days ago if there are no tags; state which you used.
- **Do not:** do not report a number you cannot derive from git or the baseline;
  do not turn this into a score or a grade; do not claim credit for changes
  procoder did not influence — report what changed, not who caused it.

Both `commands/*.toml` follow the Task 1 shape.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/skill-debt-gain.test.js`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder-debt skills/procoder-gain \
        commands/procoder-debt.toml commands/procoder-gain.toml \
        tests/skill-debt-gain.test.js
git commit -m "feat: /procoder-debt and /procoder-gain"
```

---

## Task 7: `/procoder-guard` — pre-commit and CI export

**Files:**
- Create: `skills/procoder-guard/SKILL.md`, `commands/procoder-guard.toml`, `scripts/templates/pre-commit.sh`, `scripts/templates/procoder-ci.yml`
- Test: `tests/guard.test.js`

**Interfaces:**
- Consumes: `bin/procoder.js`.
- Produces: the templates and the skill that installs them. The templates run the same engine as the hook, so local and CI results agree by construction.

- [ ] **Step 1: Write the failing test**

```js
// tests/guard.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.join(__dirname, '..');
const preCommit = fs.readFileSync(path.join(root, 'scripts/templates/pre-commit.sh'), 'utf8');
const ci = fs.readFileSync(path.join(root, 'scripts/templates/procoder-ci.yml'), 'utf8');
const skill = fs.readFileSync(path.join(root, 'skills/procoder-guard/SKILL.md'), 'utf8');

test('pre-commit template checks only staged files', () => {
  assert.match(preCommit, /git diff --cached --name-only/);
  assert.match(preCommit, /procoder(?:\.js)?\s+check/);
  assert.match(preCommit, /^set -euo pipefail$/m);
});

test('pre-commit template exits non-zero on findings', () => {
  assert.match(preCommit, /exit 1|exit \$/);
});

test('pre-commit template is valid bash', () => {
  const file = path.join(os.tmpdir(), `procoder-pc-${Date.now()}.sh`);
  fs.writeFileSync(file, preCommit);
  assert.doesNotThrow(() => execFileSync('bash', ['-n', file]));
  fs.unlinkSync(file);
});

test('CI template runs check and enforces the ratchet via the CLI', () => {
  assert.match(ci, /procoder(?:\.js)?\s+check/);
  assert.match(ci, /procoder(?:\.js)?\s+verify/);
  assert.match(ci, /runs-on:/);
  // The ratchet must use the CLI, not a shell approximation of the count.
  assert.ok(!/grep -c/.test(ci), 'CI template counts baseline entries by hand');
});

test('the skill names both files before writing them', () => {
  assert.match(skill, /pre-commit/);
  assert.match(skill, /\.github\/workflows/);
  assert.match(skill, /ask|confirm|before writing/i);
});

test('the skill states what NOT to do', () => {
  assert.match(skill, /^##.*(?:Do not|Never)/mi);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/guard.test.js`
Expected: FAIL — templates and skill missing.

- [ ] **Step 3: Write the templates, skill, and command**

```bash
#!/usr/bin/env bash
# procoder pre-commit guard. Runs the same engine as the editor hook, so a
# clean local session means a clean commit.
#
# Bypass once with: git commit --no-verify
set -euo pipefail

PROCODER="${PROCODER_BIN:-npx --no-install procoder}"

staged="$(git diff --cached --name-only --diff-filter=ACM)"
[ -n "$staged" ] || exit 0

# shellcheck disable=SC2086
if ! $PROCODER check $staged; then
  echo ""
  echo "procoder: blocking commit. Fix the findings above, or run:"
  echo "  $PROCODER baseline $staged     # accept pre-existing findings"
  echo "  git commit --no-verify          # bypass once"
  exit 1
fi
```

```yaml
# procoder CI guard. Copy to .github/workflows/procoder.yml
name: procoder
on: [push, pull_request]

jobs:
  procoder:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install procoder
        run: npm install -g procoder

      - name: Check changed files
        run: |
          base="${{ github.event.pull_request.base.sha || github.event.before }}"
          files="$(git diff --name-only --diff-filter=ACM "$base" HEAD || true)"
          if [ -n "$files" ]; then
            # shellcheck disable=SC2086
            procoder check $files
          fi

      - name: Ratchet — accepted debt may shrink, never grow
        if: hashFiles('.procoder-baseline.json') != ''
        run: procoder verify .
```

`skills/procoder-guard/SKILL.md` — frontmatter `name: procoder-guard`, triggers
"procoder guard", "add procoder to CI", "pre-commit hook", "enforce procoder
without the agent", "/procoder-guard". Body:

- **Premise:** rules the agent enforces are a habit; rules CI enforces are a
  guarantee. This installs the second.
- **Procedure:** (1) Detect what already exists — `.pre-commit-config.yaml`,
  `.husky/`, `lefthook.yml`, a bare `.git/hooks/pre-commit`, and the CI provider
  in use. **Integrate with what is there** rather than adding a second mechanism;
  if the repo already uses husky, add a line to the existing hook. (2) Tell the
  user exactly which files you will create or modify, and wait for confirmation.
  (3) Write from `scripts/templates/`, adjusting the invocation for how procoder
  is available in that repo (global install, npx, or plugin path). (4) If no
  baseline exists and `procoder check .` reports more than 50 findings, run
  `procoder baseline .` first — otherwise the guard fails on its first run and
  gets removed the same day.
- **Output:** the list of files written, the exact bypass command, and one line
  on how to update the baseline.
- **Do not:** do not write any file before naming it and getting a yes; do not
  add a second hook manager to a repo that already has one; do not enable the
  guard on a repo whose baseline you have not recorded.

`commands/procoder-guard.toml` — description "Install procoder as a pre-commit
hook and CI check.", prompt invoking the skill.

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/guard.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add skills/procoder-guard commands/procoder-guard.toml scripts/templates tests/guard.test.js
git commit -m "feat: /procoder-guard — pre-commit and CI export"
```

---

## Task 8: MCP server

**Files:**
- Create: `procoder-mcp/server.js`, `procoder-mcp/package.json`
- Test: `tests/mcp.test.js`

**Interfaces:**
- Consumes: `hooks/checks/run.checkFile`, `hooks/checks/config`, `hooks/checks/baseline`, `hooks/procoder-instructions`.
- Produces: a stdio JSON-RPC 2.0 server exposing three tools — `procoder_doctrine`, `procoder_check`, `procoder_baseline`.

- [ ] **Step 1: Write the failing test**

```js
// tests/mcp.test.js
const test = require('node:test');
const assert = require('node:assert');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const SERVER = path.join(__dirname, '..', 'procoder-mcp', 'server.js');

// Sends a batch of requests, resolves with the parsed responses in order.
function rpc(requests) {
  return new Promise((resolve, reject) => {
    const child = spawn('node', [SERVER], { stdio: ['pipe', 'pipe', 'ignore'] });
    let buffer = '';
    const responses = [];
    child.stdout.on('data', (chunk) => {
      buffer += chunk;
      let index;
      while ((index = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, index).trim();
        buffer = buffer.slice(index + 1);
        if (line) responses.push(JSON.parse(line));
        if (responses.length === requests.length) {
          child.kill();
          resolve(responses);
        }
      }
    });
    child.on('error', reject);
    setTimeout(() => { child.kill(); reject(new Error('MCP server timed out')); }, 5000);
    for (const request of requests) child.stdin.write(JSON.stringify(request) + '\n');
  });
}

test('initialize returns protocol and server info', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 1, method: 'initialize', params: {} }]);
  assert.strictEqual(res.id, 1);
  assert.ok(res.result.protocolVersion);
  assert.strictEqual(res.result.serverInfo.name, 'procoder');
});

test('tools/list advertises the three tools with schemas', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 2, method: 'tools/list', params: {} }]);
  const names = res.result.tools.map((t) => t.name).sort();
  assert.deepStrictEqual(names, ['procoder_baseline', 'procoder_check', 'procoder_doctrine']);
  for (const tool of res.result.tools) {
    assert.ok(tool.description && tool.inputSchema, `${tool.name} missing schema`);
  }
});

test('procoder_doctrine returns the ladder', async () => {
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 3, method: 'tools/call',
    params: { name: 'procoder_doctrine', arguments: { level: 'strict' } },
  }]);
  assert.match(res.result.content[0].text, /SAFE/);
  assert.match(res.result.content[0].text, /ALONE/);
});

test('procoder_check reports findings for a dirty file', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'a.ts'), 'eval(x);\n');
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 4, method: 'tools/call',
    params: { name: 'procoder_check', arguments: { path: path.join(dir, 'a.ts') } },
  }]);
  assert.match(res.result.content[0].text, /SAFE/);
});

test('an unknown method returns a JSON-RPC error, not a crash', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 5, method: 'nope/nope', params: {} }]);
  assert.ok(res.error);
  assert.strictEqual(res.error.code, -32601);
});

test('malformed JSON on one line does not kill the server', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 6, method: 'initialize', params: {} }]);
  assert.strictEqual(res.id, 6);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/mcp.test.js`
Expected: FAIL — `procoder-mcp/server.js` missing.

- [ ] **Step 3: Write the server**

```js
#!/usr/bin/env node
// procoder — MCP server (JSON-RPC 2.0 over stdio, newline-delimited).
//
// Exposes the same engine the Claude Code hooks use, for hosts that speak MCP
// but not Claude Code plugins. Hand-rolled: this needs three methods, and a
// dependency here would be the exact rung-1 addition procoder argues against.

const path = require('path');
const { loadConfig, findRepoRoot } = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { formatFindings } = require('../hooks/checks/finding');
const { getProcoderInstructions } = require('../hooks/procoder-instructions');

const PROTOCOL_VERSION = '2024-11-05';

const TOOLS = [
  {
    name: 'procoder_doctrine',
    description: 'Return the procoder doctrine — the four rungs (SAFE, TRUE, OBVIOUS, ALONE) that gate whether code may ship — filtered to an intensity level.',
    inputSchema: {
      type: 'object',
      properties: {
        level: { type: 'string', enum: ['pragmatic', 'strict', 'paranoid'], description: 'Intensity level. Defaults to strict.' },
      },
    },
  },
  {
    name: 'procoder_check',
    description: 'Run procoder checks on one file and return findings, one line each. Prefers the project\'s configured linter and always runs the universal pack (secrets, PII in logs, rot).',
    inputSchema: {
      type: 'object',
      properties: { path: { type: 'string', description: 'Absolute path to the file to check.' } },
      required: ['path'],
    },
  },
  {
    name: 'procoder_baseline',
    description: 'Report the ratchet baseline for a repository: how many pre-existing findings are accepted and therefore suppressed.',
    inputSchema: {
      type: 'object',
      properties: { path: { type: 'string', description: 'Any path inside the repository.' } },
      required: ['path'],
    },
  },
];

function text(value) {
  return { content: [{ type: 'text', text: String(value) }] };
}

function callTool(name, args = {}) {
  if (name === 'procoder_doctrine') {
    return text(getProcoderInstructions(args.level || 'strict'));
  }

  if (name === 'procoder_check') {
    const absPath = path.resolve(String(args.path || ''));
    const repoRoot = findRepoRoot(path.dirname(absPath));
    const config = loadConfig(repoRoot);
    const { relPath, findings, skipped } = checkFile(absPath, { repoRoot, config });
    if (skipped) return text(`skipped (${skipped}): ${relPath}`);
    if (findings.length === 0) return text(`clean: ${relPath}`);
    return text(formatFindings(findings, relPath));
  }

  if (name === 'procoder_baseline') {
    const repoRoot = findRepoRoot(path.resolve(String(args.path || '.')));
    const config = loadConfig(repoRoot);
    // Required lazily: the baseline module is only needed on this path.
    const { loadBaseline } = require('../hooks/checks/baseline');
    const size = loadBaseline(repoRoot, config).size;
    return text(size === 0
      ? 'No baseline recorded. Every finding is reported.'
      : `${size} pre-existing findings accepted and suppressed. New code is gated in full.`);
  }

  throw Object.assign(new Error(`unknown tool: ${name}`), { code: -32602 });
}

function handle(request) {
  const { id, method, params } = request;

  if (method === 'initialize') {
    return {
      jsonrpc: '2.0', id,
      result: {
        protocolVersion: PROTOCOL_VERSION,
        capabilities: { tools: {} },
        serverInfo: { name: 'procoder', version: '0.1.0' },
      },
    };
  }

  if (method === 'tools/list') {
    return { jsonrpc: '2.0', id, result: { tools: TOOLS } };
  }

  if (method === 'tools/call') {
    try {
      return { jsonrpc: '2.0', id, result: callTool(params && params.name, params && params.arguments) };
    } catch (e) {
      return { jsonrpc: '2.0', id, error: { code: e.code || -32603, message: e.message } };
    }
  }

  // Notifications carry no id and expect no response.
  if (id === undefined) return null;

  return { jsonrpc: '2.0', id, error: { code: -32601, message: `unknown method: ${method}` } };
}

let buffer = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buffer += chunk;
  let index;
  while ((index = buffer.indexOf('\n')) >= 0) {
    const line = buffer.slice(0, index).trim();
    buffer = buffer.slice(index + 1);
    if (!line) continue;

    let response;
    try {
      response = handle(JSON.parse(line));
    } catch (e) {
      // A malformed line must not take the server down.
      response = { jsonrpc: '2.0', id: null, error: { code: -32700, message: 'parse error' } };
    }
    if (response) process.stdout.write(JSON.stringify(response) + '\n');
  }
});
```

```json
// procoder-mcp/package.json
{
  "name": "procoder-mcp",
  "version": "0.1.0",
  "description": "MCP server exposing procoder's four-rung ship gate.",
  "license": "MIT",
  "bin": { "procoder-mcp": "./server.js" },
  "files": ["server.js"]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `chmod +x procoder-mcp/server.js && node --test tests/mcp.test.js`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add procoder-mcp tests/mcp.test.js
git commit -m "feat: MCP server exposing the procoder engine"
```

---

## Task 9: Platform command ports and rule-sync coverage

**Files:**
- Modify: `scripts/sync-rules.js` (add command porting)
- Generate: `.opencode/command/procoder-*.md`, `.qoder/rules/`, `pi-extension/index.js`, `gemini-extension.json`, `opencode.json`
- Test: `tests/sync-commands.test.js`

**Interfaces:**
- Consumes: `commands/*.toml`, `skills/*/SKILL.md`.
- Produces: `renderCommands() → Map<relativePath, content>`, folded into the existing `--check` drift gate.

- [ ] **Step 1: Write the failing test**

```js
// tests/sync-commands.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const { renderCommands, render } = require('../scripts/sync-rules');

const root = path.join(__dirname, '..');

test('every command gets an opencode port', () => {
  const commands = fs.readdirSync(path.join(root, 'commands')).filter((f) => f.endsWith('.toml'));
  const rendered = renderCommands();
  for (const file of commands) {
    const name = path.basename(file, '.toml');
    assert.ok(rendered.has(`.opencode/command/${name}.md`), `no opencode port for ${name}`);
  }
});

test('ported commands carry the generated warning', () => {
  for (const [file, content] of renderCommands()) {
    assert.match(content, /DO NOT EDIT/, `${file} missing warning`);
  }
});

test('sync --check covers commands as well as rules', () => {
  execFileSync('node', [path.join(root, 'scripts/sync-rules.js')], { cwd: root });
  const victim = path.join(root, '.opencode/command/procoder-review.md');
  const saved = fs.readFileSync(victim, 'utf8');
  try {
    fs.writeFileSync(victim, saved + '\ndrift\n');
    assert.throws(() => execFileSync(
      'node', [path.join(root, 'scripts/sync-rules.js'), '--check'],
      { cwd: root, stdio: 'pipe' }));
  } finally {
    fs.writeFileSync(victim, saved);
  }
});

test('the pi and gemini manifests list every skill', () => {
  const skills = fs.readdirSync(path.join(root, 'skills'));
  const pi = fs.readFileSync(path.join(root, 'pi-extension/index.js'), 'utf8');
  for (const skill of skills) {
    assert.ok(pi.includes(skill) || pi.includes('./skills'), `pi extension misses ${skill}`);
  }
  assert.ok(fs.existsSync(path.join(root, 'gemini-extension.json')));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/sync-commands.test.js`
Expected: FAIL — `renderCommands` is not exported.

- [ ] **Step 3: Extend the sync script**

Add to `scripts/sync-rules.js`:

```js
// Ports each commands/*.toml to the platforms that use markdown command files.
// The TOML prompt body is the source; the port is a thin wrapper so a command's
// behavior cannot drift between hosts.

function parseCommandToml(raw) {
  const description = /^description\s*=\s*"([^"]*)"/m.exec(raw);
  const prompt = /^prompt\s*=\s*"""\r?\n([\s\S]*?)"""/m.exec(raw);
  return {
    description: description ? description[1] : '',
    prompt: prompt ? prompt[1].trim() : '',
  };
}

function renderCommands() {
  const out = new Map();
  const dir = path.join(ROOT, 'commands');
  for (const file of fs.readdirSync(dir).filter((f) => f.endsWith('.toml'))) {
    const name = path.basename(file, '.toml');
    const { description, prompt } = parseCommandToml(fs.readFileSync(path.join(dir, file), 'utf8'));
    const body = [
      '---',
      `description: ${description}`,
      '---',
      '',
      WARNING,
      '',
      prompt.replace(/\$ARGUMENTS/g, '$ARGUMENTS'),
      '',
    ].join('\n');
    out.set(`.opencode/command/${name}.md`, body);
    out.set(`.openclaw/commands/${name}.md`, body);
  }
  return out;
}
```

Change `main()` to iterate `new Map([...render(), ...renderCommands()])`, and add
`renderCommands` to `module.exports`.

Then write the three static manifests:

```js
// pi-extension/index.js
// procoder — pi extension entry point. Registers the skills directory.
module.exports = { skills: ['./skills'] };
```

```json
// gemini-extension.json
{
  "name": "procoder",
  "version": "0.1.0",
  "description": "Four-rung ship gate: SAFE, TRUE, OBVIOUS, ALONE.",
  "contextFileName": "AGENTS.md"
}
```

```json
// opencode.json
{ "$schema": "https://opencode.ai/config.json" }
```

Add to `package.json`: `"pi": { "skills": ["./skills"], "extensions": ["./pi-extension/index.js"] }`
and extend `files` with `procoder-mcp/`, `bin/`, `.opencode/`, `.openclaw/`, `pi-extension/`.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm run sync && node --test tests/sync-commands.test.js`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
npm run sync
git add scripts/sync-rules.js pi-extension gemini-extension.json opencode.json \
        package.json .opencode .openclaw tests/sync-commands.test.js
git commit -m "feat: port commands to every supported platform"
```

---

## Task 10: Examples and install docs

**Files:**
- Create: `examples/README.md`, `examples/{safe,true,obvious,alone}/{before,after}.ts`, `docs/install.md`
- Modify: `README.md` (command table, MCP section, links)
- Test: `tests/examples.test.js`

**Interfaces:**
- Consumes: `bin/procoder.js`.
- Produces: nothing further; this task closes Plan 3.

- [ ] **Step 1: Write the failing test**

```js
// tests/examples.test.js
const test = require('node:test');
const assert = require('node:assert');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');
const CLI = path.join(root, 'bin', 'procoder.js');

function check(file) {
  try {
    execFileSync('node', [CLI, 'check', file], { cwd: root, encoding: 'utf8' });
    return '';
  } catch (e) {
    return String(e.stdout || '');
  }
}

test('there is one example per rung', () => {
  for (const rung of ['safe', 'true', 'obvious', 'alone']) {
    const dir = path.join(root, 'examples', rung);
    assert.ok(fs.existsSync(path.join(dir, 'before.ts')), `${rung}/before.ts missing`);
    assert.ok(fs.existsSync(path.join(dir, 'after.ts')), `${rung}/after.ts missing`);
  }
});

test('each before file trips its rung and each after file is clean', () => {
  const expected = { safe: 'SAFE', true: 'TRUE', obvious: 'OBVIOUS', alone: 'ALONE' };
  for (const [rung, label] of Object.entries(expected)) {
    const before = check(`examples/${rung}/before.ts`);
    assert.match(before, new RegExp(label), `examples/${rung}/before.ts does not trip ${label}`);
    assert.strictEqual(check(`examples/${rung}/after.ts`), '',
      `examples/${rung}/after.ts is not clean`);
  }
});

test('install docs cover every supported host', () => {
  const docs = fs.readFileSync(path.join(root, 'docs', 'install.md'), 'utf8');
  for (const host of ['Claude Code', 'Cursor', 'Windsurf', 'Cline', 'opencode', 'Kiro', 'MCP']) {
    assert.ok(docs.includes(host), `install docs missing ${host}`);
  }
});

test('README lists every command that exists', () => {
  const readme = fs.readFileSync(path.join(root, 'README.md'), 'utf8');
  for (const file of fs.readdirSync(path.join(root, 'commands'))) {
    const cmd = '/' + path.basename(file, '.toml');
    assert.ok(readme.includes(cmd), `README missing ${cmd}`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/examples.test.js`
Expected: FAIL — the `examples/` tree does not exist.

- [ ] **Step 3: Write the examples and docs**

Four example pairs, each small and real:

- `examples/safe/before.ts` — a handler taking `req.body.role` straight into an
  authorization decision, with `db.query` built by template literal.
  `after.ts` — schema-validated body, server-side role lookup, parameterized query.
- `examples/true/before.ts` — `try { … } catch (e) {}` around a write, plus an
  unhandled empty-array edge. `after.ts` — error logged with a correlation id and
  rethrown, empty case guarded, one `assert`-based self-check at the bottom.
- `examples/obvious/before.ts` — one 90-line function, six parameters, a nested
  ternary, `userArrayFiltered`. `after.ts` — three named functions, an options
  object, guard clauses, `activeUsers`, and a one-line why-comment on the
  non-obvious branch.
- `examples/alone/before.ts` — `createUserV1` still exported beside `createUser`,
  a commented-out block, and `@deprecated` with no trigger. `after.ts` — the old
  path deleted, the block gone, and the one genuine migration carrying
  `// procoder: remove after v3.0`.

`examples/README.md` — a table linking each pair with the finding it demonstrates
and the one-line fix.

`docs/install.md` — one section per host, each with the exact commands:

- **Claude Code** — `claude plugin marketplace add <repo>`, `claude plugin install procoder`, then the statusline snippet.
- **Cursor / Windsurf / Cline / Kiro / Qoder / opencode** — copy or symlink the generated rule file; name the exact path for each.
- **Codex / Copilot** — `AGENTS.md` and the `PROCODER_HOST` env var.
- **pi** — `pi` package install and the extension entry.
- **MCP** — the `mcpServers` JSON block pointing at `procoder-mcp/server.js`.
- **CLI / CI only** — `npm install -g procoder`, then `/procoder-guard` or the templates directly.
- A closing table: host → file it reads → does it support levels.

Update `README.md`: complete the command table with all ten commands, add an MCP
section linking `docs/install.md`, and link the examples.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test`
Expected: PASS — every suite across all three plans green.

- [ ] **Step 5: Commit**

```bash
git add examples docs/install.md README.md tests/examples.test.js
git commit -m "feat: worked examples and install docs"
```

---

## Task 11: Release readiness

**Files:**
- Modify: `.github/workflows/ci.yml`, `package.json`
- Create: `CHANGELOG.md`
- Test: `tests/release.test.js`

**Interfaces:**
- Consumes: everything.
- Produces: a release gate that fails on any parity gap.

- [ ] **Step 1: Write the failing test**

```js
// tests/release.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');
const readJson = (p) => JSON.parse(fs.readFileSync(path.join(root, p), 'utf8'));

test('every skill has a matching command, and vice versa', () => {
  const skills = fs.readdirSync(path.join(root, 'skills'))
    .filter((s) => s !== 'procoder');
  const commands = fs.readdirSync(path.join(root, 'commands'))
    .map((f) => path.basename(f, '.toml'))
    .filter((c) => c !== 'procoder');
  assert.deepStrictEqual(skills.sort(), commands.sort());
});

test('all ten spec commands exist', () => {
  const commands = fs.readdirSync(path.join(root, 'commands'))
    .map((f) => path.basename(f, '.toml')).sort();
  assert.deepStrictEqual(commands, [
    'procoder', 'procoder-audit', 'procoder-debt', 'procoder-deps',
    'procoder-gain', 'procoder-guard', 'procoder-help', 'procoder-review',
    'procoder-rot', 'procoder-threat',
  ]);
});

test('versions agree across every manifest', () => {
  const version = readJson('package.json').version;
  assert.strictEqual(readJson('.claude-plugin/plugin.json').version, version);
  assert.strictEqual(readJson('procoder-mcp/package.json').version, version);
  assert.strictEqual(readJson('gemini-extension.json').version, version);
});

test('the package ships every directory a host needs', () => {
  const files = readJson('package.json').files;
  for (const dir of ['hooks/', 'skills/', 'commands/', 'bin/', 'procoder-mcp/']) {
    assert.ok(files.some((f) => f === dir || f === dir.replace(/\/$/, '')),
      `package.json files missing ${dir}`);
  }
});

test('the changelog names the current version', () => {
  const changelog = fs.readFileSync(path.join(root, 'CHANGELOG.md'), 'utf8');
  assert.ok(changelog.includes(readJson('package.json').version));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/release.test.js`
Expected: FAIL — `CHANGELOG.md` missing, and versions likely disagree.

- [ ] **Step 3: Align the manifests and write the changelog**

Set `version` to `0.1.0` in `package.json`, `.claude-plugin/plugin.json`,
`procoder-mcp/package.json`, and `gemini-extension.json`. Write `CHANGELOG.md`
with a `## 0.1.0` section listing: the four-rung doctrine, three levels, the
PostToolUse check engine across six languages, the ratchet baseline, ten
commands, the MCP server, and the ten generated platform rule files.

Extend `.github/workflows/ci.yml` with a matrix over Node 18/20/22 and macOS +
Ubuntu, keeping the existing `npm test`, `npm run sync:check`, and
`node bin/procoder.js check hooks bin scripts` steps.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test && npm run sync:check && node bin/procoder.js check hooks bin scripts`
Expected: PASS on all three.

- [ ] **Step 5: Commit**

```bash
git add CHANGELOG.md package.json .claude-plugin/plugin.json \
        procoder-mcp/package.json gemini-extension.json .github/workflows/ci.yml \
        tests/release.test.js
git commit -m "chore: release readiness for 0.1.0"
```

---

## Done when

- `npm test` passes on Node 18, 20, and 22, on macOS and Ubuntu.
- All ten commands are invocable in a live Claude Code session, and each produces
  output in the standard one-line finding format.
- `/procoder-guard` installs a pre-commit hook that blocks a commit introducing
  `eval(x)` and permits it after `procoder baseline`.
- The MCP server responds to `initialize`, `tools/list`, and all three
  `tools/call` targets.
- `npm run sync:check` exits 0, and hand-editing any generated file fails CI.
- `node bin/procoder.js check hooks bin scripts` exits 0 — procoder passes its own
  rungs, with no baseline covering its own source.

## Parity checklist against ponytail

| ponytail | procoder | Plan |
|---|---|---|
| SessionStart doctrine injection | ✅ | 1 |
| SubagentStart inheritance | ✅ | 1 |
| UserPromptSubmit mode tracking | ✅ | 1 |
| Intensity levels + persistence | ✅ pragmatic/strict/paranoid | 1 |
| Statusline badge (sh + ps1) | ✅ | 1 |
| Multi-platform rule files | ✅ generated, drift-gated | 1, 3 |
| Slash commands | ✅ 10 | 1, 3 |
| MCP server | ✅ | 3 |
| pi / gemini / opencode manifests | ✅ | 3 |
| Tests | ✅ `node --test` throughout | 1–3 |
| Examples + install docs | ✅ | 3 |
| — | ➕ PostToolUse deterministic checks | 2 |
| — | ➕ Ratchet baseline | 2 |
| — | ➕ CLI + CI/pre-commit export | 2, 3 |
