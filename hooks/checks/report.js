#!/usr/bin/env node
// procoder — machine-readable renderings of a run.
//
// The one-line text format is what a human reads and what the model reads, and
// it stays the default. These two exist because a finding that only exists as
// text cannot reach the places a gate has to appear: a pull request's diff, a
// security dashboard, another tool's input.
//
//   json   procoder's own shape, versioned, everything the run knew.
//   sarif  SARIF 2.1.0 — what GitHub code scanning, Azure DevOps, and most
//          security dashboards ingest. Uploading it puts every finding on the
//          diff of the pull request that introduced it.
//
// Both are built from the same per-file results the text path prints, so a
// format can never report something the default did not.

const { rungIndex } = require('./finding');

const SCHEMA_VERSION = 1;

// SARIF has three levels and procoder has six rungs, so the mapping is by
// severity rather than by name: what blocks at the active level is an error,
// what is advisory is a warning. `note` is deliberately unused — nothing here
// is informational, and grading half the findings down to a level most
// dashboards hide by default would be a quiet way to lose them.
const sarifLevel = (blocking) => (blocking ? 'error' : 'warning');

// One entry per finding, flat, with everything a consumer needs to route it:
// the rung by name and number, the rule id, the location, and whether it
// blocked at the level this run used.
function jsonReport({ findings, level, summary, skipped }) {
  return `${JSON.stringify({
    version: SCHEMA_VERSION,
    tool: 'procoder',
    level,
    summary,
    findings: findings.map((f) => ({
      rung: f.rung,
      rungNumber: rungIndex(f.rung) + 1,
      id: f.id,
      file: f.file,
      line: f.line,
      message: f.message,
      fix: f.fix,
      blocking: f.blocking,
      fingerprint: f.fingerprint,
    })),
    skipped,
  }, null, 2)}\n`;
}

// One rule object per distinct id actually reported. Dashboards group by rule,
// and a rule with no description is a row of ids nobody can act on — so the fix
// clause travels as the rule's help text.
function sarifRules(findings) {
  const rules = new Map();
  for (const f of findings) {
    if (rules.has(f.id)) continue;
    rules.set(f.id, {
      id: f.id,
      name: f.id.replace(/[^a-zA-Z0-9]+(.)/g, (_m, c) => c.toUpperCase()),
      shortDescription: { text: `${f.rung}: ${f.id}` },
      fullDescription: { text: `procoder rung ${rungIndex(f.rung) + 1} (${f.rung}). ${f.fix}` },
      properties: { rung: f.rung, rungNumber: rungIndex(f.rung) + 1 },
    });
  }
  return Array.from(rules.values());
}

// `partialFingerprints` is what stops a dashboard from reporting the same
// finding as new every time a line moves. procoder already computes a
// line-independent fingerprint for its own ratchet, so the two agree by
// construction: what the baseline accepts is what the dashboard tracks.
function sarifResult(f) {
  const result = {
    ruleId: f.id,
    level: sarifLevel(f.blocking),
    message: { text: `${f.message} → ${f.fix}` },
    locations: [{
      physicalLocation: {
        artifactLocation: { uri: f.file },
        region: { startLine: Math.max(1, f.line) },
      },
    }],
  };
  if (f.fingerprint) result.partialFingerprints = { procoderFingerprint: f.fingerprint };
  return result;
}

function sarifReport({ findings, version }) {
  return `${JSON.stringify({
    $schema: 'https://json.schemastore.org/sarif-2.1.0.json',
    version: '2.1.0',
    runs: [{
      tool: {
        driver: {
          name: 'procoder',
          version,
          informationUri: 'https://github.com/azrtydxb/procoder',
          rules: sarifRules(findings),
        },
      },
      results: findings.map(sarifResult),
    }],
  }, null, 2)}\n`;
}

const FORMATS = new Set(['text', 'json', 'sarif']);

module.exports = { FORMATS, SCHEMA_VERSION, jsonReport, sarifReport };
