#!/usr/bin/env node
// procoder — machine-readable renderings of a run.
//
// The one-line text format is what a human reads and what the model reads, and
// it stays the default. These two exist because a finding that only exists as
// text cannot reach the places a gate has to appear: a pull request's diff, a
// security dashboard, another tool's input.
//
//   json   procoder's own shape, versioned, everything the run knew.
//   sarif  SARIF 2.1.0 — what GitHub code scanning, Azure DevOps and most
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

// `partialFingerprints` is what stops a dashboard reporting the same finding as
// new every time a line moves. procoder already computes a line-independent
// fingerprint for its own ratchet, so the two agree by construction: what the
// baseline accepts is what the dashboard tracks.
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

// A file nothing read is not a file that came back clean, and SARIF used to
// render the two identically: absent from `results` either way. The JSON format
// has always carried `skipped`; the format that feeds GitHub code scanning and
// every security dashboard did not, which made it the widest-reaching version
// of the failure this project keeps fixing — enforcement narrower than the user
// believes.
//
// The honest place for it in the 2.1.0 spec is `invocations[].
// toolExecutionNotifications`: notifications are what a tool says ABOUT its
// run, as opposed to results, which are what it found. A skipped file is not a
// finding — putting it in `results` under some invented rule would report a
// problem in the file, and would fail builds that gate on result count.
//
// The level split is the one the CLI already makes on stderr. `[exclude] paths`
// and a .procoderignore are a decision the project made about scope: coverage
// lost, worth saying, but not a broken run — `warning`. A file that was in
// scope and could not be read (over the size cap, unreadable) is a hole in the
// scope itself — `error`, and it is what drops `executionSuccessful` to false,
// the same distinction `verify` uses when it refuses to claim a ratchet.
const skip = (kind, level, why) => [kind, { id: `procoder/skipped/${kind}`, level, why }];
const SKIPS = new Map([
  skip('excluded', 'warning', 'excluded by [exclude] paths in .procoder.toml'),
  skip('ignored', 'warning', 'covered by a .procoderignore'),
  skip('too-large', 'error', 'larger than [limits] max_file_bytes'),
  skip('unreadable', 'error', 'could not be read'),
]);

// Anything unrecognised is an error, never silently dropped: a new skip reason
// that nobody mapped must show up as a hole, not as a clean file.
const OTHER_SKIP = { id: 'procoder/skipped/other', level: 'error', why: 'not checked' };

const skipKind = (reason) => (String(reason).startsWith('ignored:') ? 'ignored' : String(reason));
const skipDescriptor = (reason) => SKIPS.get(skipKind(reason)) || OTHER_SKIP;

// The spec says a notification's descriptor SHOULD resolve to a descriptor in
// `driver.notifications`. An id a consumer cannot look up is the same class of
// lie as the missing skip itself, so the ones actually used travel with the run.
function sarifNotificationRules(skipped) {
  const rules = new Map();
  for (const s of skipped) {
    const d = skipDescriptor(s.reason);
    if (rules.has(d.id)) continue;
    rules.set(d.id, {
      id: d.id,
      shortDescription: { text: `File ${d.why}.` },
      fullDescription: {
        text: `procoder did not read this file, so nothing in it was checked. `
          + 'It is absent from `results` for that reason, not because it is clean.',
      },
    });
  }
  return Array.from(rules.values());
}

function sarifNotification({ file, reason }) {
  const d = skipDescriptor(reason);
  return {
    descriptor: { id: d.id },
    level: d.level,
    message: { text: `${file} was not checked: ${d.why} (${reason}).` },
    locations: [{ physicalLocation: { artifactLocation: { uri: file } } }],
  };
}

// `runs[].automationDetails` was the other candidate and is deliberately not
// used: GitHub treats its `id` as the category that groups one analysis with
// the next, so an id that appeared only on runs that skipped something would
// split a project's history in two the first time a file went unread.
function sarifReport({ findings, version, skipped = [] }) {
  const driver = {
    name: 'procoder',
    version,
    informationUri: 'https://github.com/azrtydxb/procoder',
    rules: sarifRules(findings),
  };
  const run = { tool: { driver }, results: findings.map(sarifResult) };
  // Nothing skipped means nothing to say, and the document stays byte for byte
  // what it was before this existed: a consumer that ignores notifications is
  // never worse off than it was, and one that reads them learns exactly which
  // files went unread and why.
  if (skipped.length > 0) {
    driver.notifications = sarifNotificationRules(skipped);
    run.invocations = [{
      executionSuccessful: !skipped.some((s) => skipDescriptor(s.reason).level === 'error'),
      toolExecutionNotifications: skipped.map(sarifNotification),
    }];
  }
  return `${JSON.stringify({
    $schema: 'https://json.schemastore.org/sarif-2.1.0.json',
    version: '2.1.0',
    runs: [run],
  }, null, 2)}\n`;
}

const FORMATS = new Set(['text', 'json', 'sarif']);

// Exported so the MCP server answers for a skipped file with the same words
// and the same severity split the SARIF and JSON reports use. A third dialect
// for "this file was not checked" is how one front door ends up quieter than
// the other.
module.exports = { FORMATS, SCHEMA_VERSION, jsonReport, sarifReport, skipDescriptor };
