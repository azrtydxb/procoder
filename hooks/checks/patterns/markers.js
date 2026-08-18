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
  // semgrep ids are dotted paths through its rule registry, e.g.
  // `javascript.lang.security.detect-child-process.detect-child-process`.
  ['semgrep', /^[a-z][\w-]*(?:\.[\w-]+)+$/],
  // gosec reports through golangci-lint under its own name, and CodeQL's ids are
  // slash-separated query paths: `go/path-injection`.
  ['gosec', /^G\d{3}$/],
  ['codeql', /^[a-z][\w-]*(?:\/[\w.-]+)+$/],
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
// Every rule procoder still owns, and there are only six — the rest of this
// list described the ~9,500 lines of hand-written rules that were deleted when
// analyzers took the gate over (see hooks/checks/toolchain.js). What survives is
// what no analyzer answers for: whether procoder could look at a file at all,
// whether it ran out of time, and what it knows about a dependency manifest.
//
// A marker may also name an ANALYZER's rule — `safe/eslint:security/detect-…` —
// which is matched by shape rather than by this list, because procoder cannot
// enumerate rules it does not maintain. See EXTERNAL_RULE_SHAPES below.
const BUILTIN_RULE_IDS = Object.freeze([
  // could procoder look at this file?
  'safe/analyzer-missing', 'safe/analyzer-silent', 'safe/ungated-language',
  // did it finish, and did it have to hold anything back?
  'true/budget-exhausted', 'true/findings-suppressed',
  // dependency manifests, which no linter reads for you
  'safe/floating-version', 'safe/manifest-not-locked', 'safe/missing-lockfile',
  'true/lockfile-unreadable', 'true/manifest-unreadable',
  // rung 4 over the whole tree — `procoder rot`
  'alone/dead-export',
  ...EXTERNAL_TOOLS.map((tool) => `true/${tool}`),
]);

// What may precede a standalone marker: comment and markup punctuation only.
// Anything else — code, prose, a table cell — makes it a trailing marker.
const LITERAL_MARKER_ALONE = /^[\s/#*<!;%-]*$/;

module.exports = {
  BUILTIN_RULE_IDS,
  EXTERNAL_RULE_SHAPES,
  LITERAL_MARKER,
  LITERAL_MARKER_ALONE,
};
