#!/usr/bin/env node
// procoder — the literal tables for the rung-4 marker checks.
//
// These live apart from the check that uses them for one reason: a table of
// patterns that match orphan TODOs, blanket suppressions and stale deprecations
// is, character for character, a file full of orphan TODOs, blanket
// suppressions and stale deprecations. Detection patterns are not the thing
// they detect, but no regex can tell the two apart.
//
// This file used to be exempted wholesale in .procoder.toml for three rules by
// name. It is not any more: the lines that need it carry a line marker that
// names the one rule they describe, which is narrower, is visible where
// the line is, and generalises to the other four places in this repo with the
// same problem. Keeping the tables here and the logic in universal.js is still
// what keeps the marked set small: every line of behaviour is unmarked.
//
// Anything that is not such a literal belongs in universal.js, not here.

// A marker with no owner and no ticket. The negative lookahead is what
// distinguishes it from an owned one, which names a person or an issue id.
const ORPHAN_MARKER =
  /\b(TODO|FIXME|HACK|XXX)\b(?!\s*[(:]?\s*(?:[A-Z]{2,}-\d+|\([^)]+\)))/;  // procoder: literal alone/orphan-todo the pattern that matches unowned markers
const OWNED_MARKER = /\b(?:TODO|FIXME|HACK|XXX)\b\s*(?:\([^)]+\)|[:\s]*[A-Z]{2,}-\d+)/;  // procoder: literal alone/orphan-todo the pattern that matches owned markers

// The literal marker: this line *describes* a pattern rather than being an
// instance of one. A detection pattern, a doctrine paragraph, a test input, a
// rule id named in config — all read to a regex exactly like the thing they
// talk about, and no regex can tell the two apart.
//
// Shape, and why each part of it:
//
//   <any comment syntax> procoder: literal <rule-id>[, <rule-id>...] <reason>  // procoder: literal alone/blanket-suppression the marker syntax, written out
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
//
// The reason is `\S+\s+\S[^\n]*$` rather than a repeated `(?:\s+\S+)+`: the
// repeated form is a nested quantifier, and on a 300KB minified line that
// happens to contain the word it cost 1.8s — most of the hook's whole budget.
// Two words then anything is the same requirement without the backtracking.

// One check id, as it is spelled everywhere else in the engine: a rung, a
// slash, then the rest. The tail is deliberately wider than `[a-z-]+`, because
// a finding from a project's configured linter carries that tool's own rule id
// — `true/eslint:no-eval`, `true/ruff:E501`, `true/eslint:@typescript-eslint/
// no-explicit-any` — so an id may hold a colon, a digit, an uppercase letter,
// an `@` and a second slash. Under the narrow form a marker naming one parsed
// as a *bare* marker: it silenced nothing and was reported as a blanket
// suppression, with nothing said about the id it actually named. That is the
// same defect .procoder.toml's rule-scoped exclusions were fixed for, where
// the parser split on the last colon instead of the first.
//
// The tail excludes whitespace and commas, which is what terminates an id and
// keeps the list and the reason after it unambiguous — one way to split, so no
// backtracking. Whether a named id exists at all is checked in universal.js;
// shape is all a regex can answer.
const RULE_ID = '[a-z]+\\/[\\w@][\\w.@:/-]*';
const RULE_ID_LIST = `${RULE_ID}(?:\\s*,\\s*${RULE_ID})*`;

const LITERAL_MARKER =
  new RegExp(`procoder:\\s*literal\\s+(${RULE_ID_LIST})\\s+\\S+\\s+\\S[^\\n]*$`);

// Every check id the built-in packs can produce. A marker naming an id outside
// this set names nothing: a typo like `alone/orphan-todos`, or an id renamed
// since the marker was written, used to silence nothing and say nothing, so the
// author went on believing the line was marked.
//
// A list, because the ids are string literals inside `finding({...})` calls
// spread over the packs and are not enumerable at runtime — there is no
// registry object to ask. That is safe here for the reason the ratchet baseline
// relies on: check ids are permanent. It is kept honest by a test that scrapes
// every id literal under hooks/ and fails if one is missing, so a new check
// cannot land without landing here too.
//
// The tools registry.js knows how to invoke, and the shape each one's own rule
// ids have. The tool half is a closed set, so `true/eslnit:no-eval` is caught
// outright. The rule half is the tool's namespace and cannot be enumerated —
// but "cannot be enumerated" is not "cannot be checked". Every one of these
// four tools has a rule-id GRAMMAR that its own documentation states, and a
// grammar is a closed thing even when the set it generates is not:
//
//   ruff           a linter prefix (a run of capitals) then a number — E501,
//                  ANN101, PLR0913 — plus the one non-code `invalid-syntax`
//                  registry.js turns into a decline.
//   eslint         lowercase kebab segments, optionally `plugin/rule`, and
//                  optionally an npm scope: `no-eval`, `import/no-cycle`,
//                  `@typescript-eslint/no-explicit-any`. Any npm package may
//                  contribute a rule, so the SET is unbounded; the spelling is
//                  not.
//   clippy         a rustc/clippy lint name: snake_case, optional `clippy::`.
//   golangci-lint  the linter's own name: lowercase, kebab or snake.
//
// What this catches is the cross-tool copy: `true/ruff:no-eval`,
// `true/eslint:E501`, `true/clippy:E501` — an id pasted under the wrong tool,
// which is a whole-id mistake rather than a one-character one.
//
// What it deliberately does NOT catch, and this is the honest limit: a
// well-formed id for a rule that does not exist. `true/eslint:no-such-rule` and
// `true/ruff:ZZ999` are shaped exactly like real ids and are accepted in
// silence. Closing that needs a rule registry procoder cannot hold: eslint's
// set is open by construction, and pinning ruff's ~60 linter prefixes would
// warn on every CORRECT marker written the week ruff adds one — a warning on a
// correct marker is the same class of defect as a marker that silences nothing,
// and it is the one this project would then have to fix.
//
// Kept honest by the test that scrapes hooks/ for the tool names.
// A Map, not an object literal, and that is not a style preference: the tool
// name comes off a line of somebody's source, and an object literal answers
// `constructor`, `toString` and `hasOwnProperty` out of Object.prototype. A
// marker reading `true/constructor:x` would then hand the caller a function
// where a regex was expected and throw — out of a PostToolUse hook, which is
// the one thing nothing here may do. A Map has no prototype chain to inherit
// from and answers `undefined`, which routes to the unknown-tool warning where
// that id belongs.
const EXTERNAL_RULE_SHAPES = new Map([
  ['eslint', /^(?:@[a-z0-9][\w.-]*\/)?[a-z][\w-]*(?:\/[a-z][\w-]*)*$/],
  ['ruff', /^(?:[A-Z]{1,5}\d{1,4}|invalid-syntax)$/],
  ['golangci-lint', /^[a-z][a-z0-9_-]*$/],
  ['clippy', /^(?:clippy::)?[a-z][a-z0-9_]*$/],
]);

// The names alone, in the order above. Derived rather than spelled a second
// time: a tool present in one table and absent from the other is a tool whose
// rule half goes unchecked, silently.
const EXTERNAL_TOOLS = Object.freeze([...EXTERNAL_RULE_SHAPES.keys()]);

// The bare `true/<tool>` ids are what registry.js emits when a configured
// linter reports a finding with no rule id of its own. They are generated from
// EXTERNAL_TOOLS rather than spelled out, so the two cannot drift into
// disagreeing about which tools exist. The tool's *own* rule ids
// (`true/eslint:no-eval`) are the tool's to define and cannot be listed;
// universal.js accepts those on shape, with the tool half checked against
// EXTERNAL_TOOLS.
const BUILTIN_RULE_IDS = Object.freeze([
  'safe/dynamic-eval', 'safe/floating-version', 'safe/hardcoded-secret',
  'safe/manifest-not-locked', 'safe/missing-lockfile', 'safe/pii-in-log',
  'safe/redaction-marker', 'safe/secret-in-log',
  'safe/shell-injection', 'safe/sql-injection', 'safe/tls-disabled',
  'safe/unsafe-block', 'safe/unsafe-deserialize', 'safe/weak-hash',
  'safe/weak-random', 'safe/xss-sink', 'safe/xxe-risk',
  'true/bare-except', 'true/budget-exhausted', 'true/findings-suppressed',
  'true/ignored-error',
  'true/mutable-default', 'true/panic-in-library', 'true/printstacktrace',
  'true/missing-timeout',
  'true/swallowed-error', 'true/unclosed-resource', 'true/unwrap-in-library',
  'obvious/complexity', 'obvious/function-too-long', 'obvious/nested-ternary',
  'obvious/nesting-depth', 'obvious/too-many-params',
  'alone/blanket-suppression', 'alone/commented-code', 'alone/debug-leftover',
  'alone/deprecated-no-trigger', 'alone/orphan-todo',  // procoder: literal alone/deprecated-no-trigger the id is itself deprecation-shaped
  'alone/unexplained-suppression',
  ...EXTERNAL_TOOLS.map((tool) => `true/${tool}`),
]);

// What may precede a standalone marker: comment and markup punctuation only.
// Anything else — code, prose, a table cell — makes it a trailing marker.
const LITERAL_MARKER_ALONE = /^[\s/#*<!;%-]*$/;

// Suppressions. Silencing a tool is a claim the tool is wrong; an unnamed one
// also swallows every future finding at that location, which is how a codebase
// ends up looking clean while rotting.
//
// The literal marker is one of these and is policed as one: it is listed in all
// three patterns below so that a bare or reasonless marker is reported by the
// same two rules that catch a bare `# noqa`.  // procoder: literal alone/blanket-suppression names a bare suppression as an example
const SUPPRESSION =
  /\beslint-disable(?:-next-line|-line)?\b|#\s*noqa\b|#\s*type:\s*ignore\b|\/\/\s*nolint\b|@SuppressWarnings\s*\(|#pragma\s+warning\s+disable\b|\/\/\s*@ts-(?:ignore|expect-error)\b|#\s*pylint:\s*disable\b|\/\/\s*deepcode\s+ignore\b|procoder:\s*literal\b/i;  // procoder: literal alone/blanket-suppression the table of suppression syntaxes

// The rule identifier that scopes the suppression, per ecosystem. The marker's
// arm is appended from RULE_ID_LIST rather than spelled out a second time, so
// the three patterns cannot drift apart on what a check id may contain — which
// is precisely how a marker naming `true/eslint:no-eval` came to be read as a
// bare one.
const SUPPRESSION_NAMED = new RegExp(
  /eslint-disable(?:-next-line|-line)?\s+[\w@/-]+|#\s*noqa:\s*\w+|#\s*type:\s*ignore\[[^\]]+\]|\/\/\s*nolint:\s*[\w,-]+|@SuppressWarnings\s*\(\s*"(?!all")[^"]+"|#pragma\s+warning\s+disable\s+\w+|#\s*pylint:\s*disable=\s*[\w,-]+/.source  // procoder: literal alone/blanket-suppression the table of rule-naming syntaxes
  + `|procoder:\\s*literal\\s+${RULE_ID_LIST}`, 'i');

// A whole-file disable, or an explicit "everything" target.
const SUPPRESSION_BLANKET = new RegExp(
  /\/\*\s*eslint-disable\s*\*\/|@SuppressWarnings\s*\(\s*"all"\s*\)|\/\/\s*nolint\s*$|#\s*pylint:\s*skip-file\b/.source  // procoder: literal alone/blanket-suppression the table of blanket syntaxes
  + `|procoder:\\s*literal\\b(?!\\s+${RULE_ID})`, 'i');

// The stated reason: substantive human text after the rule identifier.
// Ecosystems spell the separator differently (`--`, `//`, `-`, `:`), or skip a
// separator and just continue with prose. What matters is that *something*
// beyond the rule name follows — so this only requires two runs of non-space
// characters anywhere in the rest of the line, once the rule-naming portion
// itself has been stripped out by the caller.
const SUPPRESSION_REASON = /\S+\s+\S+/;

const DEPRECATION_MARK = /@?\bdeprecated\b|\bDeprecated\s*\(|#\[deprecated/i;  // procoder: literal alone/deprecated-no-trigger the pattern that matches deprecations
const REMOVAL_TRIGGER =
  /\b(?:remove|delete|drop|sunset)\b[^.\n]{0,40}\b(?:after|by|in|once|when)\b|\bv?\d+\.\d+\b|\b20\d\d-\d\d(?:-\d\d)?\b/i;

// Finding text for each of the three. Held here rather than at the call site
// because the ids themselves ("alone/deprecated-no-trigger") are marker-shaped.  // procoder: literal alone/deprecated-no-trigger the rule id is marker-shaped
const ORPHAN_MARKER_FINDING = {
  rung: 'ALONE',
  id: 'alone/orphan-todo',
  message: 'TODO with no owner or ticket',  // procoder: literal alone/orphan-todo the finding text for that rule
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
  id: 'alone/deprecated-no-trigger',  // procoder: literal alone/deprecated-no-trigger the finding id for that rule
  message: 'deprecation with no removal trigger',
  fix: 'add "remove after <version|date|condition>", or delete the old path now',
};

module.exports = {
  BUILTIN_RULE_IDS,
  EXTERNAL_TOOLS,
  EXTERNAL_RULE_SHAPES,
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
