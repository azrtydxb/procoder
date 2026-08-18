#!/usr/bin/env node
// procoder — the `procoder:` marker, and the one filter that reads it.
//
// A line the author marked as DESCRIBING a pattern rather than instancing one
// drops out of every report: a doctrine page quoting an injection sink, a test
// asserting on a credential, an example that has to show the wrong thing to be
// worth reading. The marker names the rule ids it silences and reaches at most
// two lines, so it cannot become a blanket.
//
// This is the last of procoder's own source reading. Everything that used to
// live beside it — the secret patterns, the log-leak rules, the commented-code
// detector, the six language packs and the local taint tracker — was deleted
// when analyzers took the gate over; see toolchain.js for what replaced it and
// what that honestly costs. The marker survives because it is not a rule: it is
// how an author answers one, and it has to keep working for the analyzers'
// findings exactly as it did for procoder's own.

const markers = require('./patterns/markers');

// Rule ids a marker may name. The built-in set is now tiny — procoder owns only
// the two gate findings from toolchain.js — so nearly every marked id is an
// analyzer's, and the shape below is what an analyzer finding is namespaced as:
// `<rung>/<tool>:<rule>`. It admits SAFE as well as TRUE because a security rule
// from ruff or semgrep lands on rung 1, and an author must be able to answer one
// of those the same way — with a reason, on the line, in review.
const KNOWN_RULE_IDS = new Set([
  ...(markers.BUILTIN_RULE_IDS || []),
  'safe/analyzer-missing',
  'safe/ungated-language',
]);
// Every rung an analyzer finding can land on — see registry.js rungFor. The
// list is spelled out rather than left as `\w+` so a typo'd rung in a marker is
// reported as an unknown id instead of silently suppressing nothing.
const EXTERNAL_RULE_ID = /^(?:safe|true|obvious|alone)\/([\w-]+):(.+)$/;

function isExternalRuleId(id) {
  const m = EXTERNAL_RULE_ID.exec(id);
  const shape = m && markers.EXTERNAL_RULE_SHAPES.get(m[1]);
  return Boolean(shape && shape.test(m[2]));
}

// Which unknown ids have already been complained about, keyed by file as well
// as id and line: the same typo on the same line of two files is two mistakes
// in two places, and warning once named neither of them.
//
// Both engine passes over a file supply the path — this pack's own, and
// run.js's over every pack's findings — so they share a key and warn once
// between them. That was not always true. run.js used to call in without a
// path, and because THIS pass returns early when the universal pack found
// nothing, the path-less pass was usually the only one to run: through the CLI
// the warning therefore never named a file, and a typo on the same line of two
// files warned once for both. A diagnostic that cannot say which file to open
// is close to useless on a repository-wide run.
//
// The bare `id@line` key is still claimed alongside, for a caller that supplies
// no path at all (a direct API call). It dedups on that key as before, and a
// file-aware warning satisfies it — one warning per occurrence, never one per
// pass.
const reported = new Set();

// stderr, never stdout: the PostToolUse hook's stdout carries a JSON protocol,
// and stderr is where config.js already reports an exclusion it is dropping.

// stderr, never stdout: the PostToolUse hook's stdout carries a JSON protocol,
// and stderr is where config.js already reports an exclusion it is dropping.
function unknownRuleId(id, lineNo, relPath) {
  if (KNOWN_RULE_IDS.has(id) || isExternalRuleId(id)) return false;
  const bare = `${id}@${lineNo}`;
  const seen = relPath ? `${relPath}|${bare}` : bare;
  if (!reported.has(seen)) {
    reported.add(seen);
    reported.add(bare);
    process.stderr.write(
      `procoder: unknown rule id "${id}" in the literal marker at ${relPath ? `${relPath}:${lineNo}` : `line ${lineNo}`} — it suppresses nothing\n`);
  }
  return true;
}

function markedLines(lines, relPath) {
  const marked = new Map();
  lines.forEach((line, index) => {
    const m = markers.LITERAL_MARKER.exec(line);
    if (!m) return;
    const ids = m[1].split(',')
      .map((id) => id.trim())
      .filter((id) => !unknownRuleId(id, index + 1, relPath));
    const last = markers.LITERAL_MARKER_ALONE.test(line.slice(0, m.index)) ? index + 2 : index + 1;
    for (let lineNo = index + 1; lineNo <= last; lineNo += 1) {
      if (!marked.has(lineNo)) marked.set(lineNo, new Set());
      ids.forEach((id) => marked.get(lineNo).add(id));
    }
  });
  return marked;
}

// Drops the findings an author marked as descriptions. Exported because the
// marker has to reach every pack's findings, not just this one's: most of what
// a test file or a doctrine page quotes is an injection sink or a debug
// statement. checkFile applies it once over the whole set.
//
// The `indexOf` guard is what keeps this free on the files that have no marker
// at all, which is nearly all of them: no split, no regex, no allocation.
// `relPath` is optional and only names the file in an unknown-id warning. Every
// caller inside the engine supplies it — run.js included, over findings from
// every pack at once — because a warning that cannot name the file cannot be
// acted on. It stays optional for a direct API call that has no path to give.

// Drops the findings an author marked as descriptions. Exported because the
// marker has to reach every pack's findings, not just this one's: most of what
// a test file or a doctrine page quotes is an injection sink or a debug
// statement. checkFile applies it once over the whole set.
//
// The `indexOf` guard is what keeps this free on the files that have no marker
// at all, which is nearly all of them: no split, no regex, no allocation.
// `relPath` is optional and only names the file in an unknown-id warning. Every
// caller inside the engine supplies it — run.js included, over findings from
// every pack at once — because a warning that cannot name the file cannot be
// acted on. It stays optional for a direct API call that has no path to give.
function filterMarkedLiterals(source, findings, relPath) {
  if (!findings.length || String(source).indexOf('procoder:') < 0) return findings;
  const marked = markedLines(String(source).split(/\r?\n/), relPath);
  if (!marked.size) return findings;
  return findings.filter((f) => !markedIds(marked, f).has(f.id));
}

const EMPTY_SET = new Set();

// A cross-line finding — the taint scan's — is reported at the SINK, and its
// message names the line the value was built on. Both lines are the finding:
// the sink is where the hole opens, the build line is where the string was
// assembled, and which of the two an author reads as the subject depends on
// the code. Marking the build line used to suppress nothing and say nothing,
// so an author who read the message, went to the line it named and marked it
// there got no effect and no explanation — the failure mode this whole
// mechanism exists to prevent.
//
// So the marker reaches either line. It still names its rule, still states a
// reason, and still reaches no line the finding does not already name: the
// scope is the finding's own two lines, not a range between them.
//
// The build line arrives as `sourceLine` on the finding — set by taint.js
// wherever a finding's sink and its source are different lines. It used to be
// read back out of the message text with a `built at line (\d+)` regex, which
// worked and was tested, and which would have broken silently the day anyone
// reworded the message. A field cannot be reworded.

// A cross-line finding — the taint scan's — is reported at the SINK, and its
// message names the line the value was built on. Both lines are the finding:
// the sink is where the hole opens, the build line is where the string was
// assembled, and which of the two an author reads as the subject depends on
// the code. Marking the build line used to suppress nothing and say nothing,
// so an author who read the message, went to the line it named and marked it
// there got no effect and no explanation — the failure mode this whole
// mechanism exists to prevent.
//
// So the marker reaches either line. It still names its rule, still states a
// reason, and still reaches no line the finding does not already name: the
// scope is the finding's own two lines, not a range between them.
//
// The build line arrives as `sourceLine` on the finding — set by taint.js
// wherever a finding's sink and its source are different lines. It used to be
// read back out of the message text with a `built at line (\d+)` regex, which
// worked and was tested, and which would have broken silently the day anyone
// reworded the message. A field cannot be reworded.
function markedIds(marked, f) {
  const at = marked.get(f.line) || EMPTY_SET;
  const also = f.sourceLine ? marked.get(f.sourceLine) : undefined;
  return also ? new Set([...at, ...also]) : at;
}

module.exports = { filterMarkedLiterals, markedLines, isExternalRuleId };
