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

// Every manifest filename any ecosystem answers to. run.js uses this to decide
// whether a changed file is worth a manifest pass at all.
const MANIFEST_FILES = new Set(
  ECOSYSTEMS.flatMap((eco) => [eco.manifest, ...(eco.altManifests || [])]));

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

const DEP_BLOCKS = ['dependencies', 'devDependencies', 'peerDependencies'];

// Which blocks this package INSTALLS, and so which ones its own lockfile must
// account for. peerDependencies is deliberately not one of them: a peer is by
// definition the consumer's to install and is legitimately absent from the
// declaring package's lock, so reporting it was a blocking rung-1 finding on
// correct code — a library's own manifest. optionalDependencies is one: npm
// installs it when it can, and an entry the lock has never heard of was
// hand-written there exactly like a normal dependency.
const LOCK_BLOCKS = ['dependencies', 'devDependencies', 'optionalDependencies'];

// Anything that is not a concrete version, or a caret on one, floats. A caret
// is npm's own default and the lockfile holds it; `*`, `latest` and open-ended
// comparators do not resolve to anything you audited.
const FLOATING = /^(?:\*|latest|x|\d+\.x|>=|>|\^|~)/;
const CONCRETE = /^\^?\d+\.\d+\.\d+$/;

// Line lookup by declaration text: package.json is often minified onto one
// line, so a line-by-line scan finds nothing. JSON.parse gives the truth; this
// only decorates it with a location.
function lineOf(lines, name) {
  const needle = new RegExp(`"${name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"\\s*:\\s*"`);  // procoder: literal safe/eslint:security/detect-non-literal-regexp the name is regex-escaped on the line above
  const index = lines.findIndex((line) => needle.test(line));
  return index >= 0 ? index + 1 : 1;
}

// A manifest nothing could read is not a manifest with nothing wrong in it.
//
// This is the failure mode the project keeps having to fix: a parse failure
// that silently reduces coverage. Every dependency in the file went unchecked
// against the lockfile, and the original code returned an empty finding list,
// which reads to every caller — the hook, the CLI, CI — as "clean".
//
// Its own id, and NOT safe/manifest-not-locked, which it borrowed while no id
// existed. Those are two different facts. "this dependency is not in your
// lockfile" is a finding about the project; "I could not read this file" is a
// finding about procoder's own coverage, and the two must not share a
// suppression — excluding a noisy dependency check would otherwise silence
// every future "I could not read this", which is the wrong half to lose.
//
// Rung 2 (TRUE), not rung 1, on the precedent true/budget-exhausted already
// set: a coverage gap is the tool failing to do its job, not a security fact
// about the code. Rung 1 was considered — an unverifiable dependency set IS a
// security exposure — and rejected because that argument promotes every
// coverage gap to rung 1, and this project already answered it the other way.
// Nothing is lost in CI by the choice: both rungs default to `error`, both
// block the hook, and `verify` ratchets on any finding at any rung.
function unreadableManifest(base, why) {
  return finding({
    rung: 'TRUE', id: 'true/manifest-unreadable', line: 1,
    message: `${base} could not be parsed (${why}) — nothing in it was checked against the lockfile`,
    fix: 'fix the manifest syntax; until it parses, no dependency here is verified against anything',
  });
}

// The sibling fact, and a separate id for the same reason: a lockfile that
// exists and cannot be READ (a directory under the name, a permission, an I/O
// error) used to throw straight out of checkManifest and into the PostToolUse
// hook — the one thing nothing here may do.
//
// Separate from true/manifest-unreadable because the two have genuinely
// different causes and different suppressions: a package.json held as a
// publish template never parses, and a project excluding that must still hear
// that its lockfile has gone missing under it.
//
// A lockfile that exists, reads, and merely does not PARSE is deliberately not
// this finding: npmTopLevel already degrades to the text match, which is real
// coverage rather than none, and yarn/pnpm locks are never parsed at all.
function unreadableLockfile(base, why) {
  return finding({
    rung: 'TRUE', id: 'true/lockfile-unreadable', line: 1,
    message: `${base} could not be read (${why}) — nothing in this manifest was checked against it`,
    fix: 'restore the lockfile from version control; until it can be read, no dependency here is verified against anything',
  });
}

// Reads a package.json, or says why it could not — the single place that
// answers that question for npm. It runs for every package.json, lockfile or
// not, which is why the report lives here and not in checkNpmLocked: a manifest
// nothing can read also loses the floating-version pass below, and a repo with
// no lockfile would otherwise hear nothing about it.
//
// null means unreadable and already reported; callers add nothing.
function npmManifest(source, findings) {
  let manifest;
  try {
    manifest = JSON.parse(String(source));
  } catch (e) {
    findings.push(unreadableManifest('package.json', e.message.split('\n')[0]));
    return null;
  }
  // JSON that parses to a string, a number, `null` or an array is not a
  // manifest — it is a file that is not the format its name implies, and
  // reading no dependency blocks out of it is the same zero coverage a syntax
  // error gives, only quieter.
  if (typeof manifest !== 'object' || manifest === null || Array.isArray(manifest)) {
    findings.push(unreadableManifest('package.json', 'not a JSON object'));
    return null;
  }
  return manifest;
}

function checkNpmDeps(source, findings) {
  const manifest = npmManifest(source, findings);
  if (manifest === null) return;
  const lines = String(source).split(/\r?\n/);

  for (const block of DEP_BLOCKS) {
    const deps = manifest[block];
    if (!deps || typeof deps !== 'object') continue;
    for (const [name, spec] of Object.entries(deps)) {
      if (typeof spec !== 'string') continue;
      if (!FLOATING.test(spec) || CONCRETE.test(spec)) continue;
      findings.push(finding({
        rung: 'SAFE', id: 'safe/floating-version', line: lineOf(lines, name),
        message: `${name} declared as ${spec}`,
        fix: 'pin to an exact version; the lockfile alone does not protect consumers',
      }));
    }
  }
}

// Which package names a lockfile knows about.
//
// Read as text, not parsed: package-lock.json, yarn.lock and pnpm-lock.yaml are
// three unrelated formats across five format versions, and what this rule needs
// from all of them is one bit per name — is it in there at all. A name that
// appears anywhere in the lockfile counts as locked, which is the conservative
// direction: the rule only ever reports a name the lockfile does not mention
// once, and never invents a finding out of a format it half-understood.
// existsSync rather than try/catch in findLockfile: an ecosystem lists three or
// four possible lockfile names and at most one of them is there, so "absent" is
// the normal answer and not an error to swallow. A file that exists and cannot
// be read IS an error — but not one to throw into a hook, and not one to turn
// into "no lockfile", which would report every dependency in the manifest as
// unlocked. checkManifest reads it once, and turns a failure into
// true/lockfile-unreadable.

// How far above the manifest to look for the lockfile.
//
// A monorepo commits ONE lockfile, at the root, so a workspace package's own
// directory holds none. Looking only next to the manifest therefore reported
// every workspace package as having no lockfile at all — itself a false
// positive — and returned before a single entry was checked, which is why the
// rule saw no workspace package anywhere.
//
// Bounded, and it stops at the repository root: a walk that leaves the repo
// could answer with a stranger's lockfile. Four levels reaches
// `packages/<group>/<pkg>/` and stops well short of a home directory.
const LOCK_SEARCH_DEPTH = 4;

// Where the walk must stop even though a lockfile may sit above it: a vendored
// package is not part of the project that vendored it, and its own manifest
// must never be measured against that project's lock. Measured, and it is not a
// hypothetical — over 3,390 real manifests the walk without this test produced
// 5,869 findings, every one of them a dependency of a package inside
// `node_modules` checked against the application's lockfile.
const VENDOR_DIRS = new Set(['node_modules', 'vendor', 'third_party', 'site-packages']);

// A lockfile ABOVE the manifest governs it only if the directory holding it
// declares a workspace. Without that test, any project that happens to sit
// under another one is measured against a stranger's lock — measured, and the
// corpus had one: flatbuffers' `grpc/examples/ts/greeter` is its own little
// project, and the repository root's pnpm lock has never heard of its
// dependencies. A lockfile in the manifest's OWN directory needs no such
// evidence; it is already this project's.
const WORKSPACE_MARKERS = [['package.json', '"workspaces"'], ['Cargo.toml', '[workspace]']];

// A file whose text says so, or null if it is not there. A file that exists and
// cannot be read is left to throw, exactly as lockfileText leaves it: silently
// reading it as "no workspace" would turn an unreadable root into a manifest
// measured against no lockfile at all.
function declares(dir, file, marker) {
  const full = path.join(dir, file);
  try {
    return fs.existsSync(full) && fs.readFileSync(full, 'utf8').includes(marker);
  } catch (e) {
    // Loud, not silent, and that is why this one may be swallowed: no workspace
    // evidence means the walk stops, which means no lockfile is found, which
    // reports safe/missing-lockfile. The alternative is throwing into a hook.
    return false;
  }
}

function declaresWorkspace(dir) {
  return fs.existsSync(path.join(dir, 'pnpm-workspace.yaml'))
    || fs.existsSync(path.join(dir, 'go.work'))
    || WORKSPACE_MARKERS.some(([file, marker]) => declares(dir, file, marker));
}

function findLockfile(startDir, lockfiles) {
  let dir = startDir;
  for (let up = 0; up <= LOCK_SEARCH_DEPTH; up += 1) {
    if (up === 0 || declaresWorkspace(dir)) {
      const found = lockfiles.find((file) => fs.existsSync(path.join(dir, file)));
      if (found) return path.join(dir, found);
    }
    const parent = path.dirname(dir);
    if (parent === dir || fs.existsSync(path.join(dir, '.git'))) break;
    if (VENDOR_DIRS.has(path.basename(parent))) break;
    dir = parent;
  }
  return null;
}

// One name, four lockfile spellings, one question. npm v1 keys the name
// (`"left-pad": {`), npm v2/v3 key a path (`"node_modules/left-pad": {`), yarn
// and pnpm key name@range at the start of a line or after a quote. Matching the
// name bounded by any of those neighbours covers all four without parsing any
// of them, and cannot be satisfied by a name that merely CONTAINS this one —
// which is the false positive that matters, since `pad` must not be answered
// by `left-pad`.
// Built by concatenation from two plain strings, not by interpolating into a
// template literal. A template literal that CONTAINS a quote character —
// ["'/] is exactly that — desynchronises the brace scanner, which counts
// quotes and braces without parsing: the closing brace of this function went
// missing and the next one was swallowed into a 57-line span that does not
// exist. See docs/known-limitations.md.
// Leading whitespace counts as a start, and that is not cosmetic: pnpm v9
// INDENTS its package keys (`  zod@3.23.8:`), so a `^`-anchored match answered
// "not locked" for every dependency of every pnpm workspace. Measured: 92 of
// the 92 findings the workspace walk first produced over a real corpus were
// this, on two monorepos whose lock names every one of them.
const LOCK_BEFORE = '(?:^\\s*|["\'/])';
const LOCK_AFTER = '(?:["\'@:]|\\s*:)';

function lockedIn(lock, name) {
  // Escapes by allowlist — anything that is not a package-name character gets a
  // backslash — rather than by listing the regex metacharacters. The listing
  // form has to spell both curly braces inside a character class, and the shape
  // scanner counts braces without parsing, so those two opened a block that
  // does not exist and swallowed the next function into a 51-line span. Same
  // escaping, no braces on either side of the comment.
  const escaped = name.replace(/[^\w@/.-]/g, (c) => '\\' + c);
  return new RegExp(LOCK_BEFORE + escaped + LOCK_AFTER, 'm').test(lock);  // procoder: literal safe/eslint:security/detect-non-literal-regexp escaped to a closed character class on the line above
}

// A manifest entry with no lockfile entry was hand-written, not installed.
//
// That is a rung-1 finding and not a style note: nothing resolved the version,
// nothing recorded the tree it pulls in, and what CI installs is therefore not
// what anybody here reviewed. It is also the exact shape of a dependency added
// by an agent editing package.json directly — the reason the doctrine says
// dependencies are added with the package manager.
// The names a package-lock.json records as installed AT THE TOP LEVEL — the
// only place a DIRECT dependency's own edge is recorded.
//
// Text matching cannot tell a direct edge from somebody else's transitive: a
// lockfile that mentions `ms` only inside `debug`'s subtree answered "locked"
// for a direct `ms`, although nothing ever resolved that edge. package-lock is
// JSON, so where it parses the question is answered exactly: npm v2/v3 key
// `node_modules/<name>` and nest as `node_modules/a/node_modules/b`; npm v1
// keys the name at the top of `dependencies` and nests under each entry's own
// `dependencies`. npm hoists, so a direct dependency always has a top-level
// entry even when a conflicting copy is nested below.
//
// null means "not answered here" — yarn, pnpm, or a lockfile that does not
// parse — and the text match stays the fallback for those.
const NODE_MODULES = 'node_modules/';

// `node_modules/left-pad` yes, `node_modules/debug/node_modules/ms` no.
const topLevelName = (key) =>
  (key.startsWith(NODE_MODULES) && key.indexOf(NODE_MODULES, 1) < 0
    ? key.slice(NODE_MODULES.length) : null);

const keysOf = (value) => (value && typeof value === 'object' ? Object.keys(value) : []);

function npmTopLevel(lockPath, text) {
  if (path.basename(lockPath) !== 'package-lock.json') return null;
  let lock;
  try { lock = JSON.parse(String(text)); } catch (e) { return null; }

  const names = new Set(keysOf(lock && lock.dependencies));
  for (const key of keysOf(lock && lock.packages)) {
    const name = topLevelName(key);
    if (name) names.add(name);
  }
  return names.size ? names : null;
}

function checkNpmLocked(source, lock, findings) {
  // Unreadable is checkNpmDeps' finding, already pushed: it runs first and for
  // every package.json. Two findings for one cause is noise.
  const manifest = npmManifest(source, []);
  if (manifest === null) return;
  const lines = String(source).split(/\r?\n/);
  const topLevel = npmTopLevel(lock.path, lock.text);
  const isLocked = (name) => (topLevel ? topLevel.has(name) : lockedIn(lock.text, name));

  for (const block of LOCK_BLOCKS) {
    unlockedInBlock({ deps: manifest[block], block, isLocked, lines }, findings);
  }
}

function unlockedInBlock({ deps, block, isLocked, lines }, findings) {
  if (!deps || typeof deps !== 'object') return;
  for (const name of Object.keys(deps)) {
    if (isLocked(name)) continue;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/manifest-not-locked', line: lineOf(lines, name),
      message: `${name} is in ${block} and not in the lockfile`,
      fix: 'install it with the package manager (npm install <pkg>) so the version resolves and the lockfile records it',
    }));
  }
}

// --- go.mod against go.sum --------------------------------------------------
//
// go.sum carries a hash line for every module the build resolves, so a
// `require` with no go.sum entry is a hand-edited go.mod: nothing resolved it
// and nothing verified what it pulls in.
//
// A module the file `replace`s is skipped. A replacement to a local directory
// has no go.sum entry by construction, and one to another module is recorded
// under the target's name, never the source's — so requiring an entry for the
// replaced name reports correct code.
const GO_ENTRY = /^(\S+)\s+(v\S+)/;
const GO_DIRECTIVE = /^(require|replace|exclude|retract)\s*(\(?)\s*(.*)$/;

// One line of go.mod, resolved against the block it is inside: `{kind, body}`
// for a line that says something, null for one that does not. `state.block`
// carries the parenthesised form (`require (` … `)`) across lines.
function goLine(state, line) {
  if (state.block) {
    if (line === ')') state.block = null;
    return state.block ? { kind: state.block, body: line } : null;
  }
  const directive = GO_DIRECTIVE.exec(line);
  if (!directive) return null;
  if (directive[2] === '(') state.block = directive[1];
  return directive[3] ? { kind: directive[1], body: directive[3] } : null;
}

function goModules(source) {
  const required = [];
  const replaced = new Set();
  const state = { block: null };

  String(source).split(/\r?\n/).forEach((raw, index) => {
    const line = raw.replace(/\/\/.*$/, '').trim();
    if (!line) return;
    const entry = goLine(state, line);
    if (!entry) return;
    if (entry.kind === 'replace') replaced.add(entry.body.split(/\s+/)[0]);
    const module = entry.kind === 'require' && GO_ENTRY.exec(entry.body);
    if (module) required.push({ name: module[1], line: index + 1 });
  });

  return required.filter((mod) => !replaced.has(mod.name));
}

function checkGoLocked(source, lock, findings) {
  const locked = new Set(lock.text.split(/\r?\n/).map((line) => line.split(/\s+/)[0]).filter(Boolean));

  for (const mod of goModules(source)) {
    if (locked.has(mod.name)) continue;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/manifest-not-locked', line: mod.line,
      message: `${mod.name} is required in go.mod and not in go.sum`,
      fix: 'run go mod tidy so the module resolves and its hashes are recorded',
    }));
  }
}

// --- Cargo.toml against Cargo.lock ------------------------------------------
//
// Cargo.lock names every crate in the resolved graph, so a dependency it does
// not name was never resolved. Read with a section scan rather than the TOML
// parser in ./toml.js: that parser exists for .procoder.toml, warns on stderr
// about every construct it does not support, and a real Cargo.toml is full of
// them — arrays of tables above all.
//
// `package = "..."` inside a dependency's own table is a RENAME: the lock knows
// the crate it renames and has never heard of the local name.
const CARGO_DEP_SECTIONS = new Set(['dependencies', 'dev-dependencies', 'build-dependencies']);
const CARGO_KEY = /^([\w-]+)(?:\.[\w-]+)*\s*=\s*(.*)$/;
const CARGO_RENAME = /\bpackage\s*=\s*"([^"]+)"/;

// What a `[…]` header puts the following lines inside: a list of dependencies
// (`[dependencies]`, `[workspace.dependencies]`, `[target.'cfg(x)'.dependencies]`)
// or one dependency's own table (`[dependencies.serde]`). Anything else ends
// both.
function cargoSection(line, index) {
  const segments = line.replace(/^\[+|\]+$/g, '').split('.').map((s) => s.replace(/["']/g, '').trim());
  const at = segments.findLastIndex((s) => CARGO_DEP_SECTIONS.has(s));
  if (at < 0) return { inList: false, table: null };
  if (at === segments.length - 1) return { inList: true, table: null };
  if (at === segments.length - 2) {
    return { inList: false, table: { name: segments[at + 1], line: index + 1 } };
  }
  return { inList: false, table: null };
}

function cargoDependencies(source) {
  const deps = new Map();
  let at = { inList: false, table: null };

  String(source).split(/\r?\n/).forEach((raw, index) => {
    const line = raw.replace(/\s*#.*$/, '').trim();
    if (!line) return;

    if (line.startsWith('[')) {
      at = cargoSection(line, index);
      if (at.table) deps.set(at.table.name, { ...at.table });
      return;
    }

    const pair = CARGO_KEY.exec(line);
    if (!pair) return;
    // Inside a `[dependencies.foo]` table, a `package = "bar"` line of its own
    // renames it — that is how `alloc`, `core` and `libzstd` are declared
    // across the crates.io corpus, and reading only the inline
    // `foo = { package = "bar" }` form reported 217 of them as unlocked.
    const renamed = CARGO_RENAME.exec(at.table ? line : pair[2]);
    if (at.table && renamed) deps.set(at.table.name, { name: renamed[1], line: at.table.line });
    else if (at.inList) deps.set(pair[1], { name: renamed ? renamed[1] : pair[1], line: index + 1 });
  });

  return [...deps.values()];
}

const CARGO_LOCK_NAME = /^name\s*=\s*"([^"]+)"/gm;

function checkCargoLocked(source, lock, findings) {
  const locked = new Set([...lock.text.matchAll(CARGO_LOCK_NAME)].map((m) => m[1]));

  for (const dep of cargoDependencies(source)) {
    if (locked.has(dep.name)) continue;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/manifest-not-locked', line: dep.line,
      message: `${dep.name} is a Cargo dependency and not in Cargo.lock`,
      fix: 'run cargo build (or cargo update -p <crate>) so the crate resolves and the lockfile records it',
    }));
  }
}

// Which per-entry check reads which manifest. The ecosystems that are absent
// are absent deliberately, and each reason is stated here rather than left as
// a note for later:
//
//   python  requirements.txt IS the pinned artefact in most repos, and a
//           pyproject name maps to a poetry/uv lock entry only through PEP 503
//           normalization, extras and environment markers. Half-understanding
//           that produces false positives on correct code.
//   dotnet  packages.lock.json is opt-in and rarely committed; without it the
//           question is safe/missing-lockfile's, which already answers.
//   maven,  no lockfile in the default toolchain at all. There is nothing to
//   gradle  compare a declaration against.
const PER_ENTRY = {
  'package.json': checkNpmLocked,
  'go.mod': checkGoLocked,
  'Cargo.toml': checkCargoLocked,
};

// go.mod and Cargo.toml have no parse step to fail — both are read by a line
// scan, so a file that is empty, binary, or a YAML document under the name
// yields no dependencies and reports nothing, which is the same zero coverage
// an unparseable package.json used to give, only quieter.
//
// One header apiece answers it, and the threshold is measured rather than
// assumed: over 1,619 real go.mod and 1,017 real Cargo.toml from the module
// cache and the crates.io registry, every single one carries its header. A
// heuristic that fires on none of the corpus and on every non-manifest is the
// only kind worth having here — a warning on a correct manifest is the same
// class of defect as silence on a broken one.
const MANIFEST_SHAPE = {
  'go.mod': { has: /^\s*module\s+\S/m, why: 'no module directive' },
  'Cargo.toml': { has: /^\s*\[(?:package|workspace)\b/m, why: 'no [package] or [workspace] table' },
};

// The lockfile, read once, here — so that one which exists and cannot be read
// is one finding rather than an exception out of a PostToolUse hook. No
// lockfile at all never reaches this: that is safe/missing-lockfile's finding.
function checkAgainstLock(base, lockPath, source, findings) {
  let text;
  try {
    text = fs.readFileSync(lockPath, 'utf8');
  } catch (e) {
    findings.push(unreadableLockfile(path.basename(lockPath), e.code || e.message.split('\n')[0]));
    return;
  }
  PER_ENTRY[base](source, { path: lockPath, text }, findings);
}

function checkManifest(manifestPath, source) {
  const findings = [];
  const base = path.basename(manifestPath);
  const dir = path.dirname(manifestPath);

  const eco = detectEcosystems(dir).find((e) => e.manifest === base);
  const spec = eco && ECOSYSTEMS.find((e) => e.name === eco.name);
  const lockPath = spec ? findLockfile(dir, spec.lockfiles) : null;
  if (eco && lockPath === null) {
    findings.push(finding({
      rung: 'SAFE', id: 'safe/missing-lockfile', line: 1,
      message: `${eco.name} manifest with no lockfile committed`,
      fix: 'commit the lockfile so builds resolve the versions you audited',
    }));
  }

  if (base === 'package.json') checkNpmDeps(source, findings);

  const shape = MANIFEST_SHAPE[base];
  if (shape && !shape.has.test(String(source))) {
    findings.push(unreadableManifest(base, shape.why));
    return findings;
  }

  if (lockPath !== null && PER_ENTRY[base]) checkAgainstLock(base, lockPath, source, findings);
  return findings;
}

module.exports = { ECOSYSTEMS, AUDIT_COMMANDS, MANIFEST_FILES, detectEcosystems, checkManifest };
