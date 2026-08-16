#!/usr/bin/env node
// procoder — orchestrates one file's checks.
//
// Order: exclusion → read → SAFE/TRUE rules (always) → shape rules (unless the
// project's own linter covers them) → universal pack (always) → baseline
// suppression → sort → cap.

const fs = require('fs');
const path = require('path');
const { isExcluded, isRuleExcluded } = require('./config');
const { packFor } = require('./registry');
const { resolveFor, runToolResult } = require('./resolve');
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

  // The project's own linter defines this project's shape thresholds, so its
  // findings replace the pack's obvious/* rules. They never replace the pack's
  // SAFE rules: rung 1 is non-negotiable, and eslint/ruff do not check for SQL
  // injection, shell injection or disabled TLS verification by default.
  const tool = resolveFor(relPath, { repoRoot });
  let toolAnswered = false;
  if (tool) {
    const result = runToolResult(tool, { repoRoot, absPath });
    findings.push(...result.findings);
    toolAnswered = result.ok;
  }

  const pack = packFor(relPath);
  if (pack) {
    const packFindings = pack.check(source, { relPath, config });
    // A linter that timed out or crashed answered nothing, so the pack covers
    // the whole file rather than leaving a hole where the shape rules were.
    findings.push(...(toolAnswered
      ? packFindings.filter((f) => !String(f.id).startsWith('obvious/'))
      : packFindings));
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
