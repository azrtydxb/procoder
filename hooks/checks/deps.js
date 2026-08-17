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

// Anything that is not a concrete version, or a caret on one, floats. A caret
// is npm's own default and the lockfile holds it; `*`, `latest` and open-ended
// comparators do not resolve to anything you audited.
const FLOATING = /^(?:\*|latest|x|\d+\.x|>=|>|\^|~)/;
const CONCRETE = /^\^?\d+\.\d+\.\d+$/;

// Line lookup by declaration text: package.json is often minified onto one
// line, so a line-by-line scan finds nothing. JSON.parse gives the truth; this
// only decorates it with a location.
function lineOf(lines, name) {
  const needle = new RegExp(`"${name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"\\s*:\\s*"`);
  const index = lines.findIndex((line) => needle.test(line));
  return index >= 0 ? index + 1 : 1;
}

function checkNpmDeps(source, findings) {
  let manifest;
  try {
    manifest = JSON.parse(String(source));
  } catch (e) {
    return; // Not our job to report malformed JSON; the toolchain already does.
  }
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
function lockfileText(repoRoot, lockfiles) {
  // existsSync rather than try/catch: an ecosystem lists three or four possible
  // lockfile names and at most one of them is there, so "absent" is the normal
  // answer and not an error to swallow. A file that exists and cannot be read
  // IS an error, and is left to throw into the caller's own handling rather
  // than being turned into "no lockfile", which would report every dependency
  // in the manifest as unlocked.
  const found = lockfiles.find((file) => fs.existsSync(path.join(repoRoot, file)));
  return found === undefined ? null : fs.readFileSync(path.join(repoRoot, found), 'utf8');
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
const LOCK_BEFORE = '(?:^|["\'/])';
const LOCK_AFTER = '(?:["\'@:]|\\s*:)';

function lockedIn(lock, name) {
  // Escapes by allowlist — anything that is not a package-name character gets a
  // backslash — rather than by listing the regex metacharacters. The listing
  // form has to spell both curly braces inside a character class, and the shape
  // scanner counts braces without parsing, so those two opened a block that
  // does not exist and swallowed the next function into a 51-line span. Same
  // escaping, no braces on either side of the comment.
  const escaped = name.replace(/[^\w@/.-]/g, (c) => '\\' + c);
  return new RegExp(LOCK_BEFORE + escaped + LOCK_AFTER, 'm').test(lock);
}

// A manifest entry with no lockfile entry was hand-written, not installed.
//
// That is a rung-1 finding and not a style note: nothing resolved the version,
// nothing recorded the tree it pulls in, and what CI installs is therefore not
// what anybody here reviewed. It is also the exact shape of a dependency added
// by an agent editing package.json directly — the reason the doctrine says
// dependencies are added with the package manager.
function checkNpmLocked(source, repoRoot, findings) {
  const lock = lockfileText(repoRoot, ECOSYSTEMS[0].lockfiles);
  // No lockfile at all is safe/missing-lockfile's finding, already reported.
  // Two findings for one cause would be noise.
  if (lock === null) return;

  let manifest;
  try { manifest = JSON.parse(String(source)); } catch (e) { return; }
  const lines = String(source).split(/\r?\n/);

  for (const block of DEP_BLOCKS) {
    unlockedInBlock({ deps: manifest[block], block, lock, lines }, findings);
  }
}

function unlockedInBlock({ deps, block, lock, lines }, findings) {
  if (!deps || typeof deps !== 'object') return;
  for (const name of Object.keys(deps)) {
    if (lockedIn(lock, name)) continue;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/manifest-not-locked', line: lineOf(lines, name),
      message: `${name} is in ${block} and not in the lockfile`,
      fix: 'install it with the package manager (npm install <pkg>) so the version resolves and the lockfile records it',
    }));
  }
}

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
    checkNpmDeps(source, findings);
    checkNpmLocked(source, repoRoot, findings);
  }

  return findings;
}

module.exports = { ECOSYSTEMS, AUDIT_COMMANDS, MANIFEST_FILES, detectEcosystems, checkManifest };
