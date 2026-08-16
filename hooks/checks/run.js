#!/usr/bin/env node
// procoder — orchestrates one file's checks.
//
// Order: exclusion → read → language pack (or the project's own linter) →
// universal pack (always) → baseline suppression → sort → cap.

const fs = require('fs');
const path = require('path');
const { isExcluded, isRuleExcluded } = require('./config');
const { packFor } = require('./registry');
const { resolveFor, runTool } = require('./resolve');
const { checkUniversal } = require('./universal');
const { loadBaseline, suppress } = require('./baseline');
const { sortFindings, capFindings } = require('./finding');
const { MANIFEST_FILES, checkManifest } = require('./deps');

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

  // Dependency manifests get one extra pass: a floating range or an absent
  // lockfile is a rung-1 finding no language pack looks for.
  if (MANIFEST_FILES.has(path.basename(relPath))) {
    findings.push(...checkManifest(absPath, source));
  }

  const scoped = findings.filter((f) => !isRuleExcluded(config, relPath, f.id));

  const lines = source.split(/\r?\n/);
  const kept = applyBaseline
    ? suppress(scoped, { baseline: loadBaseline(repoRoot, config), relPath, lines })
    : scoped;

  return {
    relPath,
    findings: capFindings(sortFindings(kept), maxFindings),
    skipped: null,
  };
}

module.exports = { checkFile, MAX_FINDINGS };
