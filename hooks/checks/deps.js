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

  if (base === 'package.json') checkNpmDeps(source, findings);

  return findings;
}

module.exports = { ECOSYSTEMS, AUDIT_COMMANDS, MANIFEST_FILES, detectEcosystems, checkManifest };
