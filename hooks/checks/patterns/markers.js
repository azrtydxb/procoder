#!/usr/bin/env node
// procoder — the literal tables for the rung-4 marker checks.
//
// These live apart from the check that uses them for one reason: a table of
// patterns that match orphan TODOs, blanket suppressions and stale deprecations
// is, character for character, a file full of orphan TODOs, blanket
// suppressions and stale deprecations. Detection patterns are not the thing
// they detect, but no regex can tell the two apart.
//
// So this file — and only this file — is exempted in .procoder.toml, for those
// three rules by name and nothing else. Keeping the tables here and the logic
// in universal.js is what keeps that exemption narrow: every line of behaviour
// is still checked, only the literals are not.
//
// Anything that is not such a literal belongs in universal.js, not here.

// A marker with no owner and no ticket. The negative lookahead is what
// distinguishes it from an owned one, which names a person or an issue id.
const ORPHAN_MARKER =
  /\b(TODO|FIXME|HACK|XXX)\b(?!\s*[(:]?\s*(?:[A-Z]{2,}-\d+|\([^)]+\)))/;
const OWNED_MARKER = /\b(?:TODO|FIXME|HACK|XXX)\b\s*(?:\([^)]+\)|[:\s]*[A-Z]{2,}-\d+)/;

// The literal marker: this line *describes* a pattern rather than being an
// instance of one. A detection pattern, a doctrine paragraph, a test input, a
// rule id named in config — all read to a regex exactly like the thing they
// talk about, and no regex can tell the two apart.
//
// Shape, and why each part of it:
//
//   <any comment syntax> procoder: literal <rule-id>[, <rule-id>...] <reason>
//
// * It names its rules. There is no wildcard and no bare form: an unnamed
//   suppression is a rung-4 violation by this project's own doctrine, so the
//   bare form is routed into SUPPRESSION_BLANKET below and reported.
// * It states a reason — two words at minimum, same bar as every other
//   suppression here. A marker with no reason does not parse, suppresses
//   nothing, and is reported as an unexplained suppression.
// * The reason runs to end of line, so the marker is always last on its line.
//   That is what lets the caller tell a trailing marker (applies to that line
//   only) from a standalone one (applies to the line below), which is the
//   narrowest pair of scopes that covers both a test assertion and a line —
//   YAML frontmatter, a markdown table row — that cannot carry a comment.
//
// It is deliberately not shaped like any linter's suppression: no tool
// silences on it, and it silences no tool but this one.
const LITERAL_MARKER =
  /procoder:\s*literal\s+([a-z]+\/[a-z-]+(?:\s*,\s*[a-z]+\/[a-z-]+)*)\s+\S+(?:\s+\S+)+\s*$/;

// What may precede a standalone marker: comment and markup punctuation only.
// Anything else — code, prose, a table cell — makes it a trailing marker.
const LITERAL_MARKER_ALONE = /^[\s/#*<!;%-]*$/;

// Suppressions. Silencing a tool is a claim the tool is wrong; an unnamed one
// also swallows every future finding at that location, which is how a codebase
// ends up looking clean while rotting.
//
// The literal marker is one of these and is policed as one: it is listed in all
// three patterns below so that a bare or reasonless marker is reported by the
// same two rules that catch a bare `# noqa`.
const SUPPRESSION =
  /\beslint-disable(?:-next-line|-line)?\b|#\s*noqa\b|#\s*type:\s*ignore\b|\/\/\s*nolint\b|@SuppressWarnings\s*\(|#pragma\s+warning\s+disable\b|\/\/\s*@ts-(?:ignore|expect-error)\b|#\s*pylint:\s*disable\b|\/\/\s*deepcode\s+ignore\b|procoder:\s*literal\b/i;

// The rule identifier that scopes the suppression, per ecosystem.
const SUPPRESSION_NAMED =
  /eslint-disable(?:-next-line|-line)?\s+[\w@/-]+|#\s*noqa:\s*\w+|#\s*type:\s*ignore\[[^\]]+\]|\/\/\s*nolint:\s*[\w,-]+|@SuppressWarnings\s*\(\s*"(?!all")[^"]+"|#pragma\s+warning\s+disable\s+\w+|#\s*pylint:\s*disable=\s*[\w,-]+|procoder:\s*literal\s+[a-z]+\/[a-z-]+(?:\s*,\s*[a-z]+\/[a-z-]+)*/i;

// A whole-file disable, or an explicit "everything" target.
const SUPPRESSION_BLANKET =
  /\/\*\s*eslint-disable\s*\*\/|@SuppressWarnings\s*\(\s*"all"\s*\)|\/\/\s*nolint\s*$|#\s*pylint:\s*skip-file\b|procoder:\s*literal\b(?!\s+[a-z]+\/[a-z-]+)/i;

// The stated reason: substantive human text after the rule identifier.
// Ecosystems spell the separator differently (`--`, `//`, `-`, `:`), or skip a
// separator and just continue with prose. What matters is that *something*
// beyond the rule name follows — so this only requires two runs of non-space
// characters anywhere in the rest of the line, once the rule-naming portion
// itself has been stripped out by the caller.
const SUPPRESSION_REASON = /\S+\s+\S+/;

const DEPRECATION_MARK = /@?\bdeprecated\b|\bDeprecated\s*\(|#\[deprecated/i;
const REMOVAL_TRIGGER =
  /\b(?:remove|delete|drop|sunset)\b[^.\n]{0,40}\b(?:after|by|in|once|when)\b|\bv?\d+\.\d+\b|\b20\d\d-\d\d(?:-\d\d)?\b/i;

// Finding text for each of the three. Held here rather than at the call site
// because the ids themselves ("alone/deprecated-no-trigger") are marker-shaped.
const ORPHAN_MARKER_FINDING = {
  rung: 'ALONE',
  id: 'alone/orphan-todo',
  message: 'TODO with no owner or ticket',
  fix: 'add TODO(owner) or a ticket id, or do it now',
};

const BLANKET_SUPPRESSION_FINDING = {
  rung: 'ALONE',
  id: 'alone/blanket-suppression',
  message: 'suppression names no specific rule, or disables a whole file',
  fix: 'fix the code instead; if it is genuinely a false positive, name the rule and scope it to this line',
};

const UNEXPLAINED_SUPPRESSION_FINDING = {
  rung: 'ALONE',
  id: 'alone/unexplained-suppression',
  message: 'suppression states no reason',
  fix: 'say what makes this a false positive, on the same line',
};

const STALE_DEPRECATION_FINDING = {
  rung: 'ALONE',
  id: 'alone/deprecated-no-trigger',
  message: 'deprecation with no removal trigger',
  fix: 'add "remove after <version|date|condition>", or delete the old path now',
};

module.exports = {
  LITERAL_MARKER,
  LITERAL_MARKER_ALONE,
  ORPHAN_MARKER,
  OWNED_MARKER,
  SUPPRESSION,
  SUPPRESSION_NAMED,
  SUPPRESSION_BLANKET,
  SUPPRESSION_REASON,
  DEPRECATION_MARK,
  REMOVAL_TRIGGER,
  ORPHAN_MARKER_FINDING,
  BLANKET_SUPPRESSION_FINDING,
  UNEXPLAINED_SUPPRESSION_FINDING,
  STALE_DEPRECATION_FINDING,
};
