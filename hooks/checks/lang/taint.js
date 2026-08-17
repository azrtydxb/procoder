#!/usr/bin/env node
// procoder — local taint tracking, shared by the six language packs.
//
// The SAFE sink rules read one line and require the untrusted expression to
// sit textually inside the call that consumes it. Real code binds the value to
// a name first — `const q = "SELECT ... id=" + id; db.query(q);` — and that
// shape is at least as common as the inline one, so the flagship rules missed
// the majority form. This closes it.
//
// The scope is deliberately small, and it is the whole design:
//
//   * One file, forward only. No cross-file dataflow: that needs a resolver
//     per language and is out of proportion to a 2s hook. Within the file
//     there *is* one level of call propagation — see "returns" below.
//   * A statement, not a line. Physical lines are gathered into logical
//     statements first — see logicalLines — so a right-hand side a formatter
//     wrapped is one unit and `const q =` / `  "SELECT id=" + id;` reads as
//     the assignment it is. The same reading baseline.js gives a finding's
//     statement, so the flat and the wrapped form are one construct to both.
//   * A *path*, not a bare name. `o.q`, `this.query` and `q` are each a
//     binding in their own right, and a lookup falls back through the path's
//     prefixes, so a transformation at the sink — `db.query(q.trim())` —
//     still finds `q`. No containers are modelled beyond the one shape below.
//   * A binding lives at the level it was **declared** at, not the level the
//     assignment sits on, and dies when that block closes — brace depth for
//     the five brace packs, indentation column for Python. So a name assigned
//     inside a branch or a loop body is still assigned *after* it: a
//     may-analysis, which is the right bias for a security rule, and the only
//     reading under which `if (x) { q = … }` and `for (…) { q = q + p }`
//     mean anything. A declaration (`let`, `var`, `const`, `:=`, a typed
//     local) binds afresh at its own level instead, and so does a
//     *parameter*: a parameter list is a fresh binding, so the names it
//     introduces shadow whatever an enclosing scope bound them to for as long
//     as the block it opens is live. A sibling function reusing the name
//     starts clean, and gives the name back on the way out — clears are
//     recorded and undone exactly like taints, so an inner binding of the
//     name no longer clears the outer one for the rest of the file.
//   * A source is a string built from a *non-literal*: concatenation, a
//     template or f-string, `%`, `.format`, `Sprintf`, `String.format`,
//     `format!`, or a container literal that mixes a literal with a name —
//     `["SELECT id=", id]`, whose `,` plays the part `+` plays in the others.
//     A value built only from literals is never tainted.
//   * An assignment whose right-hand side names a tainted variable carries the
//     taint on. That is how a query is built incrementally — `q = q + id`,
//     `q += id`, `a = b` — and reading those as "not a source, therefore
//     clear" dropped the taint at the exact statement that introduced it. A
//     compound `+=` never clears either, since it reads the old value.
//   * Returns. One pass learns which of the file's own functions return a
//     value that is tainted where it is built; a second uses that, so
//     `const q = build(x)` and `db.query(b(1))` carry the taint of what the
//     helper returns. The second pass runs only when the first found one, so
//     a file with no such helper still costs a single pass. This is not a
//     call graph: it is one level deep, within one file, keyed by the
//     function's own name.
//   * Any other assignment clears the name, so a literal reassignment
//     untaints it.
//
// Everything outside that is a miss, and a miss is the direction this errs in
// on purpose: a tainted-variable rule that fires on safe code is worse than
// one that misses, because it is the whole pack that gets turned off. The
// clean fixtures are the guard — every case added here has its safe
// counterpart there.
//
// The two misses worth naming, because they look like the shapes above and are
// deliberately not closed:
//
//   * A parameter arriving already tainted — `function f(q) { db.query(q) }`.
//     Treating every parameter as data reports every data-access helper in
//     every repository class, including the ones whose callers pass a
//     constant, and no evidence inside this file tells the two apart. It is
//     the single largest false-positive source available here, so it stays
//     shut until there is a cross-file resolver to answer it.
//   * A container read back by index or key — `parts[0]`, `m["q"]`. The
//     container literal itself is a source (above), which is enough for the
//     `join` form; element-wise reads would need a real value model.
//
// The ids are the ones the inline form already reports. A second id for the
// same vulnerability written a second way would be the duplicate rung 4
// forbids, so `safe/sql-injection` is `safe/sql-injection` either way; only
// the message differs, naming the line the value was built on.
//
// procoder: heuristic taint, replace with a real per-language front end if
// missed shapes (fields, aliases, cross-function flow) become the complaint.

const { finding } = require('../finding');

// A string literal glued to something that is not one. Two directions,
// because either operand may be the literal, and pure-literal concatenation
// must match neither: the lookahead in the first rejects a following quote,
// and the second needs an identifier — never a quote — before the `+`.
//
// Both stay linear on a long line. The first is entered only at a quote and
// scans to the next quote, so the scans partition the line. The second is
// pinned by `(?<![\w$])` to the first character of an identifier, so an
// unbroken word run admits one starting offset instead of one per character —
// the same pin, for the same reason, as FUNCTION_SIGNATURE in ts.js.
//
// The optional `\)` covers `String::from("SELECT ") + id` and
// `sb.toString() + id`, where a call closes between the literal and the `+`.
const CONCAT = [
  /(?:"[^"\n]*"|'[^'\n]*')\s*\)?\s*\+\s*(?=[A-Za-z_$(])/,
  /(?<![\w$])[A-Za-z_$][\w$]*\s*\+\s*["']/,
];

// Statement and block boundaries are read off the *noise-stripped* line,
// whose string content stripNoise blanks to spaces of the same width, so a
// `;` inside a SQL literal or a `{` inside a template literal cannot split a
// statement. The text handed to the rules is the raw line at the same
// offsets, because the literal is exactly what the source patterns look at.
const BRACE_SPLIT = /[;{}]/g;
const SEMI_SPLIT = /;/g;

// A parameter list — the names it binds start clean inside the block it opens.
// Matched against the statement the block-opening brace ended (Python
// overrides this with its own `def` pattern), so the list is the last
// parenthesised group before the block: `function g(q) `, `rows.forEach((q) =>
// `, `catch (e) `, `void run(String q) `.
const PARAM_LIST = /\(([^()]*)\)[^(){}]*$/;

// A statement that opens a branch rather than a call frame binds nothing, so
// the names in its header keep the taint they carry in — `if (q) {` and
// `if (check(q)) {` alike. Read at the head of the statement, not at the `(`,
// because the list that precedes the block may be nested inside the header.
// `for` and `catch` are deliberately absent: both *do* bind, and a fresh loop
// variable or exception must start clean like any other parameter.
const NOT_A_BINDING = /^\s*(?:else\s+)?(?:if|while|switch|return|yield|await|lock|unless|elif)\b/;

// Two non-literals glued together — `q + id`. The pack's own CONCAT patterns
// need a literal on one side of the `+`, because that is what tells a *string*
// concatenation from an arithmetic one; but `q = q + id`, the way a query is
// built one clause at a time, has no literal in it at all, and reading it as
// "not a source" cleared the name on the statement that should have tainted
// it. It is tested against noise-stripped text, so a `+` inside a literal is
// not one, and pinned by `(?<![\w$])` to an identifier's first character for
// the same reason CONCAT is: one starting offset per word run, not one per
// character.
const NAME_CONCAT = /(?<![\w$])[A-Za-z_$][\w$]*\s*\+\s*[A-Za-z_$]/;

// A container literal that mixes a string literal with a name is the same
// "literal glued to a non-literal" shape CONCAT reads, written with a `,`
// instead of a `+`: `const parts = ["SELECT id=", id]` builds the query, and
// `parts.join("")` only spells it out. Both directions, and both are as linear
// as CONCAT and for the same two reasons — the first is entered only at a
// quote and ends at the next one, the second is pinned to an identifier's
// first character.
const COMMA_MIX = [
  /(?:"[^"\n]*"|'[^'\n]*'|`[^`\n]*`)\s*,\s*(?=[A-Za-z_$])/,
  /(?<![\w$])[A-Za-z_$][\w$]*\s*,\s*["'`]/,
];

// …but only where the value *is* a list rather than one expression. An
// ordinary call's arguments say nothing about what it returns, and reading
// `t("hello", name)` as a source would taint half of every file. So the mix
// counts only inside a container literal, in the spellings the six packs use:
// `[…]` and `{…}`, `vec![…]`, `[]string{…}`, `map[string]string{…}`,
// `new String[]{…}`, `Arrays.asList(…)`, `List.of(…)`, `listOf(…)`.
//
// Anchored at `^`, so every alternative has exactly one starting offset and
// the inner `[^\]\n]*` cannot be retried.
const CONTAINER = new RegExp([
  String.raw`^\s*&?\s*(?:vec!|new\s*[\w.<>]*\s*(?:\[\s*\])?\s*`,
  String.raw`|\[\s*\]\s*[\w.]+\s*|map\s*\[[^\]\n]*\]\s*[\w.]+\s*)?[[{]`,
  String.raw`|^\s*(?:Arrays\.asList|(?:List|Set|Map)\.of|listOf|arrayOf|mutableListOf)\s*\(`,
].join(''));

function containerMix(text) {
  return CONTAINER.test(text) && COMMA_MIX.some((re) => re.test(text));
}

// A dotted path, so `o.q` and `this.query` are read as the bindings they are
// rather than as the bare name at one end of them.
const NAME = /[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*/g;

// The value expression a sink's argument may be, built on a pack's own
// identifier pattern: a name or a dotted path, optionally called, optionally
// followed by method calls on the result. `q`, `o.q`, `build(x)`, `q.trim()`.
//
// The path is what is captured, and a lookup falls back through its prefixes
// (see builtAt) — so `q.trim` finds `q`, which is how a transformation at the
// sink keeps the taint, and `b` in `b(1)` is looked up among the file's own
// tainted-return functions.
const valuePattern = (word) => String.raw`((?:${word})(?:\.${word})*)`
  + String.raw`\s*(?:\([^()]*\))?(?:\s*\.${word}\s*\([^()]*\))*`;

// Identifiers in a fragment, read off noise-stripped text so a word inside a
// string literal — `"SELECT id"` — is not one of them.
function namesIn(code) {
  const names = [];
  NAME.lastIndex = 0;
  for (let m = NAME.exec(code); m; m = NAME.exec(code)) names.push(m[0]);
  return names;
}

function indentOf(line) {
  return /^[ \t]*/.exec(line)[0].replace(/\t/g, '    ').length;
}

function levelStep(ch, braces) {
  if (ch === '{') return braces + 1;
  if (ch === '}') return Math.max(0, braces - 1);
  return braces;
}

// Depth over `(` and `[` only. A line ending in `{` opens a block far more
// often than it wraps an expression, and following it would swallow a whole
// function body into its signature. Same reading as baseline.js's statementAt,
// deliberately: what one calls a statement the other has to call a statement
// too, or a finding's line and its fingerprint stop agreeing.
const GROUP = { '(': 1, '[': 1, ')': -1, ']': -1 };

// Operators a wrapping formatter leaves at the end of a line when it breaks a
// long assignment or concatenation. `?`, `:`, `.` and `,` are absent for the
// reason baseline.js gives: prettier moves those to the START of the
// continuation line, and a trailing `:` is a Python block header whose body
// must not be swallowed.
const CONTINUES = new Set(['+', '-', '*', '/', '%', '=', '&', '|', '^', '\\']);

// A statement longer than this stops being one, and a stray unclosed bracket
// must not let one statement swallow the rest of the file.
const JOIN_MAX_LINES = 10;

function guideAt(lines, stripped, index) {
  return stripped[index] === undefined ? lines[index] : stripped[index];
}

function groupDepth(code) {
  let depth = 0;
  for (let i = 0; i < code.length; i += 1) depth += GROUP[code[i]] || 0;
  return depth;
}

// The last non-space character, read off noise-stripped text so a `+` inside a
// literal is not one.
function lastToken(code) {
  let i = code.length - 1;
  while (i >= 0 && (code[i] === ' ' || code[i] === '\t' || code[i] === '\r')) i -= 1;
  return i < 0 ? '' : code[i];
}

// Physical lines gathered into logical statements: `[from, to]` ranges, each
// extended forward while a `(`/`[` is still open or the line ends on an
// operator a formatter breaks after. A statement already on one line is
// balanced and yields exactly itself, so nothing that fitted on a line before
// reads any differently now.
function logicalLines(guides) {
  const rows = [];
  let index = 0;
  while (index < guides.length) {
    let last = index;
    let depth = groupDepth(guides[index]);
    while (last + 1 < guides.length && last - index + 1 < JOIN_MAX_LINES
      && (depth > 0 || CONTINUES.has(lastToken(guides[last])))) {
      last += 1;
      depth += groupDepth(guides[last]);
    }
    rows.push({ from: index, to: last });
    index = last + 1;
  }
  return rows;
}

// One logical line's text, plus where each of its physical lines starts in it.
// Joined with a single space, so the two spellings stay at identical offsets:
// `text` is the raw slice, because the source patterns read the literal the
// value is built from, and `code` is the same span with string contents
// blanked, which is what structure — an assignment's operator, a parameter
// list, the names on a right-hand side — is read off.
function joinRange(rows, from, to) {
  const starts = [0];
  let text = rows[from];
  for (let i = from + 1; i <= to; i += 1) {
    starts.push(text.length + 1);
    text += ` ${rows[i]}`;
  }
  return { text, starts };
}

// Offset of the first non-space character, or 0 for a slice that is all space.
function firstWord(text) {
  let i = 0;
  while (i < text.length && (text[i] === ' ' || text[i] === '\t')) i += 1;
  return i === text.length ? 0 : i;
}

// Which physical line an offset in the joined text falls on. A statement spans
// at most JOIN_MAX_LINES, so the walk is over ten entries at worst — and it is
// what keeps every finding on the line the author wrote it on, wrapped or not.
function rowOf(starts, offset) {
  let row = 0;
  while (row + 1 < starts.length && starts[row + 1] <= offset) row += 1;
  return row;
}

// Half the languages here spell a container literal with braces —
// `[]string{"SELECT id=", id}`, `map[string]string{…}`, `new[] { … }`,
// `new String[]{…}` — and a brace is exactly what a statement is cut at. So
// the statement ends before the literal it is assigning, and the source that
// makes it a source is in the next unit.
//
// Rather than move the boundary, which is where every binding's level comes
// from, the literal is handed back to the statement as a *tail*: the braces
// still open and close a block and the depth is untouched, and only the source
// test reads it. It is offered when the `{` sits where a composite literal's
// does — right after a `]` or a `new Foo` — and only when the matching `}` is
// on the same logical line, so no unbalanced case can reach it. A block brace
// that happens to qualify (`if m[k] {`) reaches nothing: the tail is read only
// from the right of an assignment, and only by the container test.
// How far either half of the test may look. Both bounds are what keep this
// O(1) per brace on a line with a great many of them: reading the prefix as a
// slice and asking a `$`-anchored regex about it was O(n) per brace and so
// quadratic in line length — 100KB of `function f(a,b){…}` took 19s. A type
// name longer than the first, or a container literal longer than the second,
// gives the tail up rather than the budget.
const HEAD_LOOKBACK = 128;
const LITERAL_MAX = 512;

// Does the `{` at `at` open a composite literal rather than a block? It does
// when a type sits immediately before it — `[]string{`, `map[string]string{`,
// `new[] {`, `new List<string> {`, `new String[]{`. Read backwards from the
// brace, so nothing is copied and nothing is rescanned.
const TYPE_CHAR = /[\w.*<>]/;

// Index of the last character at or before `from` that is not one of `chars`,
// never looking further back than `floor`.
function skipBack(text, from, floor, chars) {
  let i = from;
  while (i >= floor && chars.test(text[i])) i -= 1;
  return i;
}

const SPACE = /[ \t]/;

function compositeHead(text, at) {
  const floor = Math.max(0, at - HEAD_LOOKBACK);
  let i = skipBack(text, at - 1, floor, SPACE);
  i = skipBack(text, i, floor, TYPE_CHAR);
  i = skipBack(text, i, floor, SPACE);
  if (text[i] === ']') return true;
  // `new`, as a whole word, is the other thing a literal may follow.
  return i >= 2 && text.slice(i - 2, i + 1) === 'new' && !WORD.test(text[i - 3] || ' ');
}

function braceLiteral(raw, guide, at) {
  if (!compositeHead(guide.text, at)) return undefined;
  let depth = 0;
  for (let i = at; i < guide.text.length && i - at < LITERAL_MAX; i += 1) {
    if (guide.text[i] === '{') depth += 1;
    else if (guide.text[i] === '}') {
      depth -= 1;
      if (depth === 0) return raw.text.slice(at, i + 1);
    }
  }
  return undefined;
}

// The statements one logical line holds, split at `;{}` (at `;` alone where
// the pack is indentation-scoped). `state` carries the brace depth across
// logical lines, which is what the level of every statement is read from.
function splitUnits(raw, guide, from, state) {
  const units = [];
  const push = (start, end) => {
    const text = raw.text.slice(start, end);
    units.push({
      text,
      code: guide.text.slice(start, end),
      level: state.level,
      // Only a statement that actually spans physical lines has to be located
      // within one: the single-line case is every line of a normal file, and
      // giving it the leading-whitespace scan too was measurable at 2MB.
      line: raw.starts.length === 1 ? from + 1 : from + rowOf(raw.starts, start + firstWord(text)) + 1,
    });
  };

  let start = 0;
  state.split.lastIndex = 0;
  for (let m = state.split.exec(guide.text); m; m = state.split.exec(guide.text)) {
    push(start, m.index);
    // The statement a `{` ended is the one that opens a block, and so the only
    // one whose trailing parenthesised list can be a parameter list.
    if (m[0] === '{') {
      units[units.length - 1].opens = true;
      units[units.length - 1].tail = braceLiteral(raw, guide, m.index);
    }
    state.braces = levelStep(m[0], state.braces);
    if (!state.indent) state.level = state.braces;
    start = m.index + 1;
  }
  push(start, raw.text.length);
  return units;
}

// One statement, the block level it sits at, and its line number.
function unitsOf(lines, stripped, indent) {
  // Materialised once. Built inside the loop it would be one pass over the
  // whole file per statement, which is the quadratic every other pass here
  // was written to avoid.
  const guides = lines.map((_, i) => guideAt(lines, stripped, i));
  const state = { split: indent ? SEMI_SPLIT : BRACE_SPLIT, indent, braces: 0, level: 0 };
  const units = [];

  for (const { from, to } of logicalLines(guides)) {
    const raw = joinRange(lines, from, to);
    if (indent && !raw.text.trim()) continue;
    state.level = indent ? indentOf(lines[from]) : state.braces;
    for (const unit of splitUnits(raw, joinRange(guides, from, to), from, state)) {
      units.push(unit);
    }
  }
  return units;
}

// Bindings made deeper than the level now in scope are gone. A stack, so the
// whole scan stays linear in the number of assignments rather than costing a
// sweep of every live name per statement.
//
// Each entry carries what the name meant *before* it was bound, and closing a
// block restores that rather than deleting: a parameter shadowing an outer
// tainted name has to hand the name back on the way out, or a fresh binding
// inside a function would untaint the caller's variable for the rest of the
// file. Entries are undone last-first, so a name bound twice at one level
// unwinds to what it was before the first of them.
function forget(vars, open, level) {
  while (open.length && open[open.length - 1].level > level) {
    const frame = open.pop();
    for (let i = frame.names.length - 1; i >= 0; i -= 1) {
      const { name, prev } = frame.names[i];
      if (prev === undefined) vars.delete(name);
      else vars.set(name, prev);
    }
  }
}

function frameAt(open, level) {
  let top = open[open.length - 1];
  if (!top || top.level !== level) {
    top = { level, names: [] };
    open.push(top);
  }
  return top;
}

// Is the binding this name currently has outside the function being scanned?
//
// Only asked of a pack with no declarator — Python — where the function
// boundary *is* the declarator: `q = "SELECT 1"` inside a `def` is a new local
// whatever an enclosing scope bound `q` to, and reading it as a write to the
// outer `q` let a nested helper clear a caller's taint for the rest of the
// file. The five packs that spell their declarations answer this with
// `declare` instead, and a closure there really does write the outer binding.
function outsideFunction(scope, name) {
  const live = scope.funcLevel === undefined ? undefined : scope.vars.get(name);
  return Boolean(live) && live.level <= scope.funcLevel;
}

// Give `name` a value. `builtLine` is the line the value was built on, or null
// for a value this scan can prove carries no data.
//
// Which *binding* is written is the whole of the block-aware merge. A
// declaration — and only a declaration — binds afresh at the level the
// statement sits on. Anything else writes the binding that is already live,
// wherever it was declared, so a name assigned inside a branch or a loop body
// is still assigned after the block closes. That is a may-analysis, and it is
// the right bias for a security rule: err toward reporting and let the
// constant-discharge rules suppress the safe cases.
//
// A write to an outer binding needs no undo record: the frame that owns the
// binding already holds what the name meant before it existed, and restoring
// the intermediate values on the way out would mean nothing. A write at this
// level does, and a *clearing* one just as much as a tainting one — recording
// only taints is what let an inner `let q = "SELECT 1"` clear an outer tainted
// `q` for the rest of the file.
function bind(scope, unit, { name, builtLine, declares }) {
  const live = declares || outsideFunction(scope, name) ? undefined : scope.vars.get(name);
  const level = live ? live.level : unit.level;
  if (level === unit.level) {
    frameAt(scope.open, level).names.push({ name, prev: scope.vars.get(name) });
  }
  scope.vars.set(name, { builtLine, level });
}

// A parameter list introduces fresh names, one level in from the statement it
// sits on — which is where the block it opens begins, whether that is the next
// brace level or the next indentation column. They start clean whatever an
// enclosing scope bound them to, and give the name back when the block closes.
function shadow(scope, unit, names) {
  if (!names.length) return;
  const frame = frameAt(scope.open, unit.level + 1);
  for (const name of names) {
    frame.names.push({ name, prev: scope.vars.get(name) });
    scope.vars.set(name, { builtLine: null, level: unit.level + 1 });
  }
}

// The line the value behind `path` was built on, or undefined.
//
// A path is looked up whole and then by its prefixes: `o.q` finds the field's
// own binding, and `q.trim` — the shape a transformation at the sink leaves —
// finds `q`. A prefix that is bound and clean ends the walk, because a local
// of that name is what the expression reads.
//
// The file's own tainted-return functions answer to the same lookup, which is
// how `build(x)` and `b(1)` carry what the helper returns. They are consulted
// before a clean binding ends the walk, since `const build = (id) => …` binds
// the name as well as defining the function.
function builtAt(scope, path) {
  for (let at = path.length; at > 0; at = path.lastIndexOf('.', at - 1)) {
    const entry = scope.vars.get(path.slice(0, at));
    if (entry && entry.builtLine) return entry.builtLine;
    const returned = scope.returns.get(path.slice(0, at));
    if (returned) return returned;
    if (entry) return undefined;
  }
  return undefined;
}

// The parameter list a statement ends with, if it opens a block that binds.
function paramsOf(spec, unit) {
  if (NOT_A_BINDING.test(unit.code)) return [];
  const match = (spec.params || PARAM_LIST).exec(unit.code);
  return match ? namesIn(match[1]) : [];
}

// The one id whose sink verbs — `execute`, `query`, `Query`, `raw` — are also
// ordinary method names. A Command pattern and a job runner both spell their
// entry point `execute`, and reporting SQL injection in a file that contains no
// SQL at all is the rung-1 finding on textbook code that gets a tool switched
// off. Named rather than flagged per rule because the id is permanent and the
// question — "is this really a database call" — is the same wherever it is
// asked: line rule, span rule, taint sink.
const SQL_ID = 'safe/sql-injection';

// Evidence that this file talks to a database at all, in any of the three
// shapes it can take. Any one of them is enough, because each covers what the
// others miss: a query assembled entirely from fragments has no statement text,
// an ORM call has no SQL text either, and a raw driver call has no library name.
//
//   * SQL statement text. The verbs that double as ordinary method names —
//     `update`, `create`, `delete`, `drop` — need their following keyword
//     before they count; `select` and `truncate` do not, since neither is a
//     method name anyone writes by accident.
//   * A database vocabulary word, as a substring rather than a whole word:
//     `sql` alone catches `SqlCommand`, `sqlx`, `mysql`, `sqlalchemy` and
//     `executeSql`, and none of them means anything else.
//   * A receiver that is a canonical database handle, at the shape it is
//     called on: `db.`, `cur.`, `conn.`, `stmt.`, `session.`.
//
// It is deliberately weak, and its only job is to tell a file that talks to a
// database from one that never does. A file that does keeps every finding it
// had; a Command pattern or a job runner, whose `execute` and `query` are
// ordinary method names, loses a rung-1 SQL finding it should never have had.
const DB_EVIDENCE = new RegExp([
  String.raw`\b(?:select|truncate|upsert|insert\s+into|update\s+\w+\s+set|delete\s+from`,
  String.raw`|(?:create|alter|drop)\s+(?:table|index|view)|merge\s+into|replace\s+into)\b`,
  String.raw`|sql|jdbc|psycopg|pymysql|sequelize|prisma|knex|mongoose|postgres|sqlite`,
  String.raw`|database|datasource|repository|entitymanager|dbcontext`,
  String.raw`|\b(?:db|cur|cursor|conn|connection|pool|session|stmt|statement|tx|trx|dao|repo)\s*\.`,
].join(''), 'i');

// The gate above asks about the *file*, and that is too coarse in one
// direction: a genuine injection in a file with no SQL vocabulary in it went
// silent. The shape is ordinary — a thin data-access module whose query text
// arrives as a parameter or as a constant imported from elsewhere, so the sink
// is present and the vocabulary is not:
//
//   import { Client } from 'pg';
//   import { BY_NAME } from './statements';
//   export const find = (client, name) => client.execute(BY_NAME + "'" + name + "'");
//
// Nothing on those lines matches DB_EVIDENCE, and a missed rung-1 injection is
// worse than the false positive the gate exists to prevent. So two further
// pieces of evidence, both weighed *per call* where they can be, since evidence
// about the call beats evidence about the file:
//
//   * DB_METHOD — the method's full call form, where that form is only ever a
//     database call. `execute` and `query` are ordinary method names;
//     `executeQuery`, `executeUpdate`, `rawQuery`, `QueryRowContext`,
//     `executemany` and `CommandText` are not, and no Command pattern or job
//     runner spells its entry point any of those ways. Read at the call, so it
//     licenses that one call and no other line in the file.
//   * DB_DRIVER — a database driver or ORM imported anywhere in the file. This
//     one *is* file-level, and it is used only as the tie-break the per-call
//     evidence cannot settle: a bare `x.execute(q)` on a receiver of unknown
//     shape. A file that imports `pg`, `typeorm` or `gorm` and then calls
//     `execute` on a concatenated string is doing SQL; a job runner is not
//     importing a database driver. The names here are the ones DB_EVIDENCE's
//     substring vocabulary does not already cover — `sql`, `postgres`,
//     `sqlite`, `prisma`, `knex`, `sequelize`, `jdbc`, `psycopg` and the rest
//     are matched there and are deliberately not repeated.
//
// What stays missed, on purpose: `handle.execute(base + name)` in a file with
// no SQL text, no handle-shaped receiver, no database method form and no driver
// import. Nothing in such a file distinguishes it from the Command pattern, and
// guessing would give the false positive back.
const DB_METHOD = new RegExp([
  String.raw`\b(?:execute_?(?:query|update|batch|many)|executemany`,
  String.raw`|raw_?query|query_?rows?|query_?(?:rows?_?)?context|query_?as|query_?scalar`,
  String.raw`|exec_?context|prepare_?statement|create_?(?:native_?)?query|native_?query)\s*\(`,
  String.raw`|\bCommandText\s*[(=]`,
].join(''), 'i');

// An import line naming a driver or ORM. Both halves must hold: the token is
// short and generic on its own (`pg`, `gorm`), and requiring it to sit on a
// line that imports something is what keeps a local variable called `pg` from
// counting. Bounded by word edges for the same reason — `pg` must not match
// `upgrade`. A `.` is a word edge here and not an exclusion, because a dotted
// module path is how half the ecosystems spell an import: `org.hibernate`,
// `psycopg2.pool`, `github.com/jackc/pgx/v5`.
const IMPORT_LINE = /\b(?:import|require|from|use|using|include|extern|open)\b/;
const DB_DRIVER = new RegExp([
  String.raw`(?:^|[^\w$])(?:pg|pgx|pgtype|npgsql|oracledb|tedious|dapper`,
  String.raw`|hibernate|mybatis|jooq|jdbi|typeorm|drizzle|objection|slonik`,
  String.raw`|bookshelf|massive|gorm|diesel|rusqlite|asyncpg|peewee|alembic`,
  String.raw`|duckdb|clickhouse|mariadb|cockroach|libsql|turso)(?:[^\w$]|$)`,
].join(''), 'i');

function hasSql(lines) {
  return lines.some((line) => DB_EVIDENCE.test(line))
    || lines.some((line) => IMPORT_LINE.test(line) && DB_DRIVER.test(line));
}

// Is this one call a database call? The file-level answer where it is already
// yes, and the per-call method form where it is not — so a vocabulary-free file
// still reports at the call whose own form says "database", and stays silent at
// the one whose form says nothing. Ordered so the per-call regex runs only on a
// line a rule has already matched in a file the gate would otherwise close.
// The gate above asks about the *file*, and that is the other half of too
// coarse: a module that legitimately runs SQL lends its evidence to every
// `execute` in it, including `redis.execute(cmd)`, `runner.execute(step)`,
// `cache.execute(cacheKey(id))` and `api.execute(q)`. Those receivers are
// plainly not database handles, and the file's SQL says nothing about them.
//
// So the receiver is read at the call, and a call made on a receiver from this
// list is not a database call whatever the file contains. This subtracts from
// the file-level evidence only — an unknown receiver is unchanged, which is
// what keeps a genuine injection through a real handle firing, and a file with
// no database evidence at all is unchanged too, which is what keeps `execute`
// as an ordinary method name silent.
const NON_DB_RECEIVER = new RegExp([
  String.raw`^(?:redis|valkey|memcache\w*|cache\w*|queue\w*|jobs?|runner|worker`,
  String.raw`|scheduler|tasks?|api|rpc|grpc|http\w*|fetch|mail\w*|smtp|s3|bucket`,
  String.raw`|kafka|rabbit\w*|amqp|broker|pubsub|topic|events?|bus|shell|cmd`,
  String.raw`|child_process|proc|process|browser|page|elastic\w*|opensearch|solr`,
  String.raw`|ldap|mq|nats|sqs|sns|celery|sidekiq)$`,
].join(''), 'i');

// A sink verb that doubles as an ordinary method name, with the receiver it is
// called on. Pinned by a non-path character before the receiver, so an
// unbroken word run admits one starting offset — the same pin, for the same
// reason, as CONCAT.
const RECEIVER_CALL = /(?:^|[^\w$.])([\w$]+)\s*[.:]+\s*(?:query|execute|exec|raw)\w*\s*\(/gi;

// Is every candidate sink call on this line made on a receiver that is not a
// database handle? A line with no such call at all answers no, and is left to
// the evidence above.
function nonDatabaseReceiver(line) {
  RECEIVER_CALL.lastIndex = 0;
  let seen = false;
  for (let m = RECEIVER_CALL.exec(line); m; m = RECEIVER_CALL.exec(line)) {
    if (!NON_DB_RECEIVER.test(m[1])) return false;
    seen = true;
  }
  return seen;
}

function isDatabaseCall(line, ctx) {
  if (nonDatabaseReceiver(line)) return false;
  return ctx.sql !== false || DB_METHOD.test(line);
}

// A sink whose argument is a name this scan has seen built from a non-literal.
// Every capture group is a candidate, which is how Go's `QueryContext(ctx, q)`
// is read as well as `Query(q)`.
//
// The finding is reported at the *sink*, but its real subject is the line the
// value was built on — which is where an author who reads it writes a
// suppression marker. `sourceLine` carries that line as a field, so a
// consumer reads it instead of parsing "built at line N" back out of the
// message; universal.js keyed off the prose before, and prose gets reworded.
//
// It is set only when the two lines differ, and it is added to the object
// after `finding()` has built it: `finding()` validates and normalises the five
// keys every finding has, and this is not one of them. Nothing that identifies
// a finding reads it — the baseline fingerprint is id, path, line text and
// ordinal — so every existing baseline entry stays valid.
function sinkFinding(sink, unit, scope) {
  const match = sink.re.exec(unit.text);
  if (!match) return null;
  let built;
  for (const group of match.slice(1)) {
    built = group && builtAt(scope, group);
    if (built) break;
  }
  if (!built) return null;
  const found = finding({
    rung: 'SAFE',
    id: sink.id,
    line: unit.line,
    message: `${sink.message}, built at line ${built}`,
    fix: sink.fix,
  });
  if (built !== unit.line) found.sourceLine = built;
  return found;
}

// `q = q + id` builds a query out of two names and no literal, so none of the
// source patterns match it — and reading that as "not a source, therefore
// clear" dropped the taint on the statement that introduced it. A right-hand
// side that names a tainted variable carries it on instead, which also covers
// `a = b` and a tainted name inside a template or an f-string.
//
// `+=` reads the old value, so it never clears: appending a literal to a
// tainted query leaves it tainted, and appending anything that is not a
// literal taints it. The operator is read off `code`, so a `+=` inside a
// string is not one.
//
// A right-hand side that is *provably constant* — built only from literals and
// from names this file binds only to literals — never taints, whatever the
// source patterns say about it. `const TABLE = "users"; q = "SELECT * FROM " +
// TABLE` is a concatenation, so CONCAT matches it; it is also not data, and
// reporting it was the same defect as reporting `os.system("ls /tmp")`, one
// level of indirection out.
// Is this one value built from something that is not a literal, or from a name
// that already carries data? The same question an assignment's right-hand side
// and a `return`'s expression both ask, which is why it is one function.
function valueIsTainted(spec, scope, text, code) {
  if (fragmentIsConstant(text, scope.consts)) return false;
  return spec.sources.some((re) => re.test(text))
    || NAME_CONCAT.test(code)
    || containerMix(text)
    || namesIn(code).some((name) => builtAt(scope, name));
}

function applyAssignment(spec, unit, scope) {
  const match = spec.assign.exec(unit.code);
  if (!match) return;
  const from = match.index + match[0].length;
  const code = unit.code.slice(from);
  // The tail is the brace-delimited container literal this statement was cut
  // in front of, where there is one — see braceLiteral.
  const text = unit.text.slice(from) + (unit.tail || '');
  const compound = match[0].includes('+=');
  // A compound `+=` reads the old value, so it is never a declaration and
  // never clears.
  const declares = !compound && Boolean(spec.declare) && spec.declare.test(unit.code);

  const tainted = valueIsTainted(spec, scope, text, code)
    || (compound && namesIn(code).length);
  if (!tainted && compound) return;
  bind(scope, unit, { name: match[1], builtLine: tainted ? unit.line : null, declares });
}

// A wholly constant call — the other half of "a value built only from literals
// is never tainted", applied to the inline sink rules rather than to a name.
//
// `os.system("ls /tmp")` reported safe/shell-injection: the rule keys on the
// call, and nothing untrusted is in it. A rung-1 rule that fires on obviously
// safe code is the one that gets a tool switched off, so a rule marked
// `dataSink` — one whose finding is "untrusted data reaches this call" rather
// than "this API is the defect" — is discharged when the line's calls carry
// only constants.
//
// What counts as data, read in one left-to-right pass:
//
//   * Identifiers *inside* a call: a name, or a nested call's own name, is a
//     value this scan cannot see through. `os.system(cmd)` and
//     `os.system(build())` both report. Identifiers outside every call are the
//     statement's own callee path — `os`, `system`, `new`, `Command` — and say
//     nothing about the arguments.
//   * A keyword-argument name, an object key and the language constants
//     (`true`/`false`/`null`/`nil`/`None`/`undefined`) are not data:
//     `subprocess.run("ls", shell=True)` and `{ shell: true }` are constant.
//   * Interpolation inside a literal — `${…}` anywhere, `$name` in a Kotlin
//     string, `{…}` in an f-string or a C# `$"…"`. A shell literal containing
//     `$HOME` is read as interpolation too, which keeps a finding that a
//     narrower test would have dropped; erring toward reporting is the right
//     direction for the half of this that decides what stays silent.
//   * A string prefix that is not interpolating — `b"…"`, `r"…"`, `u"…"` — is
//     part of the literal. `f"…"` and `$"…"` are not.
//
// One pass over the line, entered only when a rule has already matched it.
const QUOTES = '"\'`';
const WORD = /[\w$]/;
const WORD_START = /[A-Za-z_]/;
const CONSTANT_WORD = new Set([
  'true', 'false', 'null', 'nil', 'none', 'undefined',
  // Operators and binding keywords spelled as words. None of them is a value,
  // so none of them is data: `new Foo(x)` is about `x`, not about `new`.
  'new', 'await', 'async', 'return', 'throw', 'yield', 'typeof', 'instanceof',
  'not', 'and', 'or', 'in', 'is', 'as', 'mut', 'ref', 'out',
]);
// A prefix that does *not* interpolate — the string is exactly its own text.
const LITERAL_PREFIX = new Set(['b', 'r', 'u', 'rb', 'br', 'ur', 'l']);
// A prefix that does: Python's f-string, in every spelling.
const FSTRING_PREFIX = new Set(['f', 'fr', 'rf', 'fb', 'bf']);
const CLOSER = { '(': ')', '[': ']', '{': '}' };

// How deep a constant test follows interpolation nested inside interpolation.
// A hole's text is a strict substring of the literal that holds it, so the
// recursion terminates on its own; the cap is what keeps the *cost* bounded on
// a line that nests them pathologically.
const HOLE_DEPTH = 3;

// Index of the quote that closes the one at `from`, or the last character of
// the line when it never closes.
function stringEnd(line, from) {
  for (let i = from + 1; i < line.length; i += 1) {
    if (line[i] === '\\') i += 1;
    else if (line[i] === line[from]) return i;
  }
  return line.length - 1;
}

// Index of the bracket that closes the one at `at`, or the last character of
// the text. Brackets inside a string literal are not structure.
function closeBracket(text, at) {
  const open = text[at];
  const close = CLOSER[open];
  let depth = 0;
  for (let i = at; i < text.length; i += 1) {
    if (QUOTES.includes(text[i])) i = stringEnd(text, i);
    else if (text[i] === open) depth += 1;
    else if (text[i] === close) {
      depth -= 1;
      if (depth === 0) return i;
    }
  }
  return text.length - 1;
}

// Index of the first character at or after `from` that is not a space.
function skipSpace(line, from) {
  let i = from;
  while (line[i] === ' ' || line[i] === '\t') i += 1;
  return i;
}

// The expressions a literal interpolates, in every spelling the six packs
// meet: `${…}` (JS, Kotlin), `$name` (Kotlin), and — only where the literal's
// own syntax says so, an f-string or a C# `$"…"` — a bare `{…}`. `{{` is an
// escaped brace and holds nothing.
//
// A shell literal's `$HOME` is read as a hole, as it was before: the hole's
// text is then `HOME`, which is not a constant this file binds, so the literal
// still counts as data. Erring toward reporting is the right direction here.
function holeAt(body, at, braced) {
  if (body[at] === '$' && body[at + 1] === '{') {
    const end = closeBracket(body, at + 1);
    return { text: body.slice(at + 2, end), end };
  }
  if (body[at] === '$' && WORD_START.test(body[at + 1] || '')) {
    const end = wordEnd(body, at + 1);
    return { text: body.slice(at + 1, end), end: end - 1 };
  }
  if (!braced || body[at] !== '{') return null;
  // `{{` is an escaped brace and holds nothing.
  if (body[at + 1] === '{') return { text: '', end: at + 1 };
  const end = closeBracket(body, at);
  return { text: body.slice(at + 1, end), end };
}

function holesOf(body, braced) {
  const holes = [];
  for (let i = 0; i < body.length; i += 1) {
    const hole = holeAt(body, i, braced);
    if (!hole) continue;
    if (hole.text) holes.push(hole.text);
    i = hole.end;
  }
  return holes;
}

// An identifier that is not itself a value: a keyword-argument name
// (`shell=True`), an object or path key (`shell: true`, `Command::new`), a
// language constant, or a name this file binds only to constants.
function constantWord(word, line, after, consts) {
  if (consts && consts.has(word)) return true;
  if (CONSTANT_WORD.has(word.toLowerCase())) return true;
  const at = skipSpace(line, after);
  return line[at] === ':' || (line[at] === '=' && line[at + 1] !== '=');
}

// Where the token starting at `at` ends, and whether it is data.
// An interpolating literal is data wherever it sits — `always` — because a
// rule may key on an assignment rather than on a call, and `Arguments = $"/c
// {cmd}"` has no enclosing call to be inside of.
//
// A literal whose holes are *all* constant is not data: a column list, a table
// name or an allow-list lookup spliced into an otherwise parameterized query is
// the recommended way to write a query with a dynamic column, and it is
// correct — the value is not user data.
// `scope` is `{ consts, depth }` — the file's constant names, and how far into
// nested interpolation this test already is.
function stringToken(line, at, braced, scope) {
  const end = stringEnd(line, at);
  const holes = holesOf(line.slice(at + 1, end), braced);
  const inner = { ...scope, depth: scope.depth + 1 };
  const constant = scope.depth < HOLE_DEPTH
    && holes.every((hole) => constantFragment(hole, inner));
  return { end, always: true, data: holes.length > 0 && !constant };
}

function wordEnd(line, at) {
  let end = at + 1;
  while (end < line.length && WORD.test(line[end])) end += 1;
  return end;
}

function wordToken(line, at, scope) {
  const end = wordEnd(line, at);
  const word = line.slice(at, end);
  if (QUOTES.includes(line[end])) {
    const lower = word.toLowerCase();
    if (FSTRING_PREFIX.has(lower)) return stringToken(line, end, true, scope);
    return { end: end - 1, data: !LITERAL_PREFIX.has(lower) };
  }
  return { end: end - 1, data: !constantWord(word, line, end, scope.consts) };
}

function tokenAt(line, at, scope) {
  if (QUOTES.includes(line[at])) return stringToken(line, at, line[at - 1] === '$', scope);
  if (WORD_START.test(line[at])) return wordToken(line, at, scope);
  return null;
}

// Is *this one value* built only from things that are provably constant?
//
// constantLine below asks a different question — "does this whole statement
// carry any data anywhere" — and can therefore ignore the identifiers outside
// its calls, which are the statement's own callee path. Here the fragment *is*
// one value: a query's SQL argument, an assignment's right-hand side, an
// interpolation hole. A bare identifier in it is the value, so it has to be
// judged, and the callee path has to be told apart from it by shape:
//
//   * a word followed by `.` or `::` is a path segment, and a word followed by
//     `{` is a type being constructed. Neither is a value;
//   * a word followed by `(` is a callee. A *formatter* is transparent — what
//     `format!("SELECT * FROM {}", TABLE)` produces is its arguments — so the
//     walk carries on into them and judges each on its own. Any other call is
//     opaque: what `request.getParameter("c")` returns is user data, and
//     nothing about its arguments says otherwise.
//
//     `strict` decides what "opaque" costs. Where the fragment is the value a
//     SQL rule keyed on, or a name's whole right-hand side, an opaque call is
//     data and the fragment is not constant — that is the half that must never
//     go quiet on a real hole. Where it is only the right of an assignment
//     being checked for a dataSink discharge, an opaque call is judged by its
//     arguments instead, which keeps `x = os.system("ls /tmp")` as silent as it
//     was before; an *empty* argument list has nothing to judge and is data
//     either way;
//   * a word followed by `[` on a *constant* name is the allow-list form:
//     whatever the index, the result is one of that constant's own values, so
//     the index is skipped;
//   * anything else is data unless it is constant by constantWord.
//
// Each branch answers with the index to carry on from, or -1 for "this is data".
const PATH_SEGMENT = /^(?:\.|::|\{)/;
const CALL_OPEN = /^!?\(/;

// The string-formatting constructs the SQL rules key on — `format!(…)`,
// `String.format(…)`, `"…".format(…)`, `fmt.Sprintf(…)`, `String.Format(…)`.
const FORMATTERS = new Set(['format', 'sprintf']);

// ---- What makes taint die -------------------------------------------------
//
// A wrapping call at a binding — `q = sanitizeSql(q)`, `const id =
// mysql.escape(raw)`, `const safe = escapeHtml(x)` — was read as
// taint-preserving, because the rebuild made *every* wrapping call preserving
// so that a transformation at the sink (`db.query(q.trim())`) would keep it.
// That reported the three shapes above, each of which is the correct code.
//
// Two directions were available, and they trade a false positive against a
// false negative:
//
//   * Invert the default: a wrapping call *clears* unless it is a known
//     preserving transformation (`trim`, `toUpperCase`, `strip`, `concat`,
//     slicing). Zero false positives on any escaper, named or local — and a
//     false negative on every call this file cannot see through, which is
//     nearly all of them. `q = wrap(q)` for any project-local `wrap`, and
//     `db.query(helper(q))`, both go silent. That is the whole taint scan
//     giving up on one unknown call, and it is not a trade a rung-1 rule may
//     make.
//   * Allow-list the escapers: taint dies at a call whose callee is *named*
//     as sanitising, or is a real driver escape in one of the six ecosystems.
//     Everything else stays preserving, exactly as now.
//
// This takes the allow-list. What it costs is written down and is real: a
// project-local sanitiser whose name carries no sanitising verb —
// `clean(q)`, `safen(q)`, `harden(q)` — still reports, and that false positive
// stays. The direction is deliberate: a miss here is a shipped injection,
// a false positive here is one line a reader can see is wrong.
//
// The verb must *begin* the callee's last segment. Matching it anywhere would
// take `notAnEscaper` — the unsafe twin of defect 1 — for an escaper, and a
// helper whose name merely mentions escaping is not one.
const SANITIZER_VERB = new RegExp([
  String.raw`^(?:sanitiz|sanitis|escap|quot|purif`,
  // Names where the verb is not first but the whole word is unambiguous:
  // `htmlEscape`, `HtmlEncode`, `HTMLEscapeString`, `htmlspecialchars`,
  // `addslashes`, `encodeForHTML`, `encode_text` (html_escape, Rust).
  String.raw`|htmlescap|htmlencod|urlencod|htmlspecialchars|addslashes`,
  String.raw`|encodefor|encode_text|encode_safe|encode_quoted)`,
].join(''), 'i');

// `escape`, `quote` and `sanitize` on their own are not evidence: the JS
// global `escape()` is a URL encoder, a bare `quote()` is anybody's string
// helper, and a bare `sanitize()` says nothing about what it sanitises *for* —
// a form validator and an HTML escaper are both spelled that way, and only one
// of them makes markup safe. Spelled bare, they count only on a receiver that
// names the library — which is exactly how those libraries document them:
// `mysql.escape`, `sqlstring.escape`, `conn.escape`, `DOMPurify.sanitize`.
// Spelled with anything after the verb — `escapeHtml`, `sanitizeHtml`,
// `escape_string` — the name is unambiguous and no receiver is needed.
const BARE_VERB = /^(?:escape|escaped|quote|quoted|sanitize|sanitise|sanitized|sanitised)$/i;
const ESCAPE_RECEIVER = new RegExp([
  String.raw`(?:^|[.:])(?:mysql\d?|mysql2|sqlstring|mariadb|sqlite\d?|pg|pgp`,
  String.raw`|knex|sequelize|conn|connection|pool|client|driver|cursor|cur`,
  String.raw`|db|dbh|handle|escaper`,
  // The HTML side: DOMPurify and the sanitiser objects framework code binds.
  String.raw`|dompurify|purify|purifier|sanitizer|sanitiser|xss)$`,
].join(''), 'i');

// The per-ecosystem escapes whose names carry no verb at all, so nothing above
// can reach them. Each is the documented way its ecosystem escapes a value it
// cannot bind:
//
//   * Python — `html.escape`, `markupsafe.escape`, `saxutils.escape`,
//     `cgi.escape`; `psycopg.sql.Literal` / `sql.Identifier`; `shlex.quote`
//     and its `pipes.quote` predecessor; `bleach.clean`.
//   * JS/TS — `pg-format`'s `format.literal` / `format.ident`.
//   * Go — `strconv.Quote`. (`pq.QuoteLiteral`, `pq.QuoteIdentifier` and
//     `template.HTMLEscapeString` are already named by their verb.)
//   * Java — OWASP `Encode.forHtml` and friends. (`StringEscapeUtils.escapeSql`
//     and `HtmlUtils.htmlEscape` are named by their verb.)
//   * Rust — `ammonia::clean`, `html_escape::encode_text`.
//   * C# — `HttpUtility.HtmlEncode`, `SqlCommandBuilder.QuoteIdentifier` are
//     named by their verb; `SqlParameter` is not a wrapping call at all and
//     needs no entry, and neither does Java's `PreparedStatement` — both are
//     the parameterized path, where no value is concatenated to begin with.
const NAMED_CLEANER = new RegExp([
  String.raw`(?:^|[.:])(?:`,
  String.raw`html[.:]+escape|markupsafe[.:]+escape|saxutils[.:]+escape|cgi[.:]+escape`,
  String.raw`|sql[.:]+(?:Literal|Identifier)|format[.:]+(?:literal|ident)`,
  String.raw`|shlex[.:]+quote|pipes[.:]+quote|strconv[.:]+Quote`,
  String.raw`|bleach[.:]+clean|ammonia[.:]+clean`,
  String.raw`|Encode[.:]+for\w+`,
  String.raw`)$`,
].join(''), 'i');

// How far back a callee path is read. A path longer than this gives the
// sanitiser test up rather than the linear-scan budget — the same bound, for
// the same reason, as HEAD_LOOKBACK above.
const PATH_LOOKBACK = 64;
const PATH_CHAR = /[\w$.:]/;

// The dotted (or `::`-separated) callee path whose last segment starts at
// `at`. Read backwards, so nothing is copied and nothing is rescanned.
function calleePath(text, at, end) {
  const floor = Math.max(0, at - PATH_LOOKBACK);
  let i = at;
  while (i > floor && PATH_CHAR.test(text[i - 1])) i -= 1;
  return text.slice(i, end);
}

// Does a call to `path` remove whatever the value carried?
function isSanitizer(path) {
  if (NAMED_CLEANER.test(path)) return true;
  const cut = Math.max(path.lastIndexOf('.'), path.lastIndexOf(':'));
  const last = path.slice(cut + 1);
  if (!SANITIZER_VERB.test(last)) return false;
  return BARE_VERB.test(last) ? ESCAPE_RECEIVER.test(path.slice(0, Math.max(cut, 0))) : true;
}

// A sanitising call is opaque in the one direction that matters: whatever went
// in, what comes out is not data. So the whole argument list is skipped — with
// `q = sanitizeSql(q)` the `q` inside must not be judged on its own, or the
// value would be data again on the very statement that cleaned it.
function fragmentCall(text, at, end, scope) {
  const from = skipSpace(text, end);
  const open = text[from] === '!' ? from + 1 : from;
  if (isSanitizer(calleePath(text, at, end))) return closeBracket(text, open);
  if (!FORMATTERS.has(text.slice(at, end).toLowerCase()) && scope.strict) return -1;
  return text.slice(open + 1, closeBracket(text, open)).trim() ? end - 1 : -1;
}

function fragmentWord(text, at, scope) {
  const end = wordEnd(text, at);
  if (QUOTES.includes(text[end])) {
    const token = wordToken(text, at, scope);
    return token.data ? -1 : token.end;
  }
  const from = skipSpace(text, end);
  const after = text.slice(from, from + 2);
  if (PATH_SEGMENT.test(after)) return end - 1;
  if (CALL_OPEN.test(after)) return fragmentCall(text, at, end, scope);
  if (after[0] === '[' && scope.consts && scope.consts.has(text.slice(at, end))) {
    return closeBracket(text, from);
  }
  return constantWord(text.slice(at, end), text, end, scope.consts) ? end - 1 : -1;
}

function constantFragment(text, scope) {
  for (let i = 0; i < text.length; i += 1) {
    if (QUOTES.includes(text[i])) {
      const token = stringToken(text, i, text[i - 1] === '$', scope);
      if (token.data) return false;
      i = token.end;
    } else if (WORD_START.test(text[i])) {
      const next = fragmentWord(text, i, scope);
      if (next === -1) return false;
      i = next;
    }
  }
  return true;
}

// The entry point everything outside this section uses: a fragment, and the
// file's constant names.
const fragmentIsConstant = (text, consts) => constantFragment(text, { consts, depth: 0, strict: true });

// The looser reading, used only for the right of an assignment a dataSink rule
// matched: an opaque call is judged by its arguments rather than counted as
// data. See fragmentWord for why the two differ.
const assignedIsConstant = (text, consts) => constantFragment(text, { consts, depth: 0 });

// Not an assignment: `==`, `!=`, `<=`, `>=`, `===` and the arrow `=>`.
function assignsAt(line, at) {
  return line[at + 1] !== '=' && line[at + 1] !== '>' && !'=!<>'.includes(line[at - 1]);
}

// A wholly constant call — the other half of "a value built only from literals
// is never tainted", applied to the inline sink rules rather than to a name.
//
// `os.system("ls /tmp")` reported safe/shell-injection: the rule keys on the
// call, and nothing untrusted is in it. A rung-1 rule that fires on obviously
// safe code is the one that gets a tool switched off, so a rule marked
// `dataSink` — one whose finding is "untrusted data reaches this call" rather
// than "this API is the defect" — is discharged when the line's calls carry
// only constants.
//
// Identifiers *inside* a call are data; identifiers outside every call are the
// statement's own callee path — `os`, `system`, `new`, `Command` — and say
// nothing about the arguments.
//
// A sink that is an *assignment* breaks that: `el.innerHTML = value` puts the
// data at depth 0, where the callee-path reading would have discharged the very
// rule that exists to report it, and safe/xss-sink could not be opted in to
// this discharge at all until it was fixed. So a top-level `=` splits the line:
// what is left of it is a target and is read as a statement, and what is right
// of it is one value and is read as a fragment.
function callDepthConstant(line, consts) {
  let depth = 0;
  for (let i = 0; i < line.length; i += 1) {
    if (line[i] === '(') depth += 1;
    else if (line[i] === ')') depth = Math.max(0, depth - 1);
    else {
      const token = tokenAt(line, i, { consts, depth: 0 });
      if (!token) continue;
      if (token.data && (depth > 0 || token.always)) return false;
      i = token.end;
    }
  }
  // A call still open at end of line has its arguments somewhere else — a
  // wrapped `eval(\n  userInput,\n)` — and nothing on this line says what they
  // are. The rule matched, so the answer has to be "report".
  return depth === 0;
}

// Offset of the `=` of a top-level assignment, or -1. Inside a call it is a
// keyword argument, not an assignment: `subprocess.run(cmd, shell=True)`.
function topLevelAssign(line) {
  let depth = 0;
  for (let i = 0; i < line.length; i += 1) {
    if (QUOTES.includes(line[i])) i = stringEnd(line, i);
    else if (line[i] === '(') depth += 1;
    else if (line[i] === ')') depth = Math.max(0, depth - 1);
    else if (line[i] === '=' && depth === 0 && assignsAt(line, i)) return i;
  }
  return -1;
}

// One pass over the line, entered only when a rule has already matched it.
function constantLine(line, consts) {
  const at = topLevelAssign(line);
  if (at === -1) return callDepthConstant(line, consts);
  return callDepthConstant(line.slice(0, at), consts)
    && assignedIsConstant(line.slice(at + 1), consts);
}

// The value a SQL rule keyed on: from the delimiter that opens the call — or
// the `=` of an assignment sink like `cmd.CommandText = …` — to the top-level
// `,`, `)` or `;` that ends it.
//
// The rest of the line is precisely what makes the query *safe*: `[id]`,
// `(id,)`, `, id`, `.bind(id)` are the bound parameters, which are untrusted by
// design and say nothing about whether the SQL text itself was built from data.
// Testing the whole line instead — as the dataSink discharge does — could never
// discharge a parameterized query at all, which is why every one of them
// reported.
const ARGUMENT_END = ',;';
const CLOSING = ')]}';

// Where the argument that starts at `start` ends: the first `,`, `;` or closing
// bracket that is not inside a nested group or a literal.
function argumentEnd(line, start) {
  let depth = 0;
  for (let i = start; i < line.length; i += 1) {
    const ch = line[i];
    if (QUOTES.includes(ch)) i = stringEnd(line, i);
    else if (CLOSER[ch]) depth += 1;
    else if (depth === 0 && (CLOSING.includes(ch) || ARGUMENT_END.includes(ch))) return i;
    else if (CLOSING.includes(ch)) depth -= 1;
  }
  return line.length;
}

function sqlArgument(line, re) {
  const match = re.exec(line);
  const open = match && /[(=]/.exec(match[0]);
  if (!open) return '';
  const start = match.index + open.index + 1;
  return line.slice(start, argumentEnd(line, start));
}

// The discharge a pack hands to lineRuleFindings, and the same test spans.js
// applies to a span rule. `ctx` is the pack's whole-file context — see
// packContext.
function skipConstant(rule, line, ctx = {}) {
  if (rule.id === SQL_ID && rule.re) {
    if (!isDatabaseCall(line, ctx)) return true;
    const argument = sqlArgument(line, rule.re);
    if (argument.trim() && fragmentIsConstant(argument, ctx.consts)) return true;
  }
  return Boolean(rule.dataSink) && constantLine(line, ctx.consts);
}

// How many times the constant set is recomputed. Each round can only add names
// — one built from names the previous round proved — so the set converges;
// three rounds past the literal-only base is more indirection than any real
// column-list or table-name constant has.
const CONSTANT_ROUNDS = 4;

// The names this file binds *only* to values it can prove constant.
//
// Whole-file and order-independent, deliberately: a name assigned anything
// unprovable anywhere in the file is not constant, so a later
// `col = req.query.col` disqualifies an earlier literal binding and no scope
// tracking is needed. A name that appears in any parameter list is out for the
// same reason — a parameter is the caller's value, whatever the body later
// assigns to it.
//
// The right-hand side is read off the whole *line*, not off the brace-split
// unit the taint scan walks: an object literal is a perfectly good constant —
// `const ORDER = { created: "created_at ASC" }` — and the unit boundary falls
// in the middle of it. The assignment itself is matched on the noise-stripped
// line, so an `=` inside a literal is not one.
//
// A parameter is the caller's value whatever the body later assigns to it, so
// a name any parameter list binds is out. That list is read only from a
// statement that actually opens a block — `paramsOf`'s generic "the last list
// before the block" cannot otherwise tell `void run(String q) {` from
// `stmt.executeQuery(q)`, and in the taint scan it does not have to, because
// the shadow it creates dies with the block. Here it would disqualify a
// genuinely constant name for the whole file.
function boundNames(spec, unit) {
  if (!spec.params && !unit.opens) return [];
  return paramsOf(spec, unit);
}

// Every right-hand side each name is ever assigned, by name.
function assignedValues({ lines, stripped, spec }) {
  const values = new Map();
  lines.forEach((line, index) => {
    const guide = stripped[index] === undefined ? line : stripped[index];
    const match = spec.assign.exec(guide);
    if (!match) return;
    const from = match.index + match[0].length;
    // The path's last segment, which is the only part fragmentWord judges:
    // every segment before a `.` is read there as callee path and skipped, so
    // `this.table = "users"` makes `table` the constant, exactly as a bare
    // `table = "users"` does.
    const name = match[1].slice(match[1].lastIndexOf('.') + 1);
    const list = values.get(name) || [];
    list.push(line.slice(from));
    values.set(name, list);
  });
  return values;
}

function constantNames(parts) {
  const values = assignedValues(parts);
  const bound = new Set();
  for (const unit of parts.units) {
    for (const name of boundNames(parts.spec, unit)) bound.add(name);
  }

  const provable = (list, consts) => list
    .every((value) => value.trim() && fragmentIsConstant(value, consts));

  let consts = new Set();
  for (let round = 0; round < CONSTANT_ROUNDS; round += 1) {
    const next = new Set();
    for (const [name, list] of values) {
      if (!bound.has(name) && provable(list, consts)) next.add(name);
    }
    if (next.size === consts.size) return next;
    consts = next;
  }
  return consts;
}

// One finding per id per line: two sinks on one line describe one hole.
function collect(state, found) {
  if (!found) return;
  const key = `${found.id}:${found.line}`;
  if (state.seen.has(key)) return;
  state.seen.add(key);
  state.findings.push(found);
}

const RETURN = /^\s*return\b/;

// A value that is nothing but one name or dotted path, optionally parenthesised
// — `a`, `(q)`, `this.q`. Whether it carries data is one lookup.
const PLAIN_VALUE = /^[\s(]*([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)[\s);]*$/;

// A block header that binds no function name, whatever the pack's `func`
// pattern makes of it. `if (x) {` is a call-shaped statement in half these
// languages, and reading it as a function would attribute the `return` inside
// it to a function called `if`.
const CONTROL = new RegExp([
  String.raw`^\s*(?:\}\s*)?(?:@\w+\s*)*(?:else\s+)?`,
  String.raw`\b(?:if|else|while|for|foreach|do|try|catch|finally|switch|match|loop`,
  String.raw`|with|using|lock|unless|elif|when|return|yield|guard|synchronized)\b`,
].join(''));

// The name of the function this block opens, or null for a block that opens no
// function. Only consulted on a statement that actually opens a block —
// except where the pack is indentation-scoped and there is no brace to see.
function functionName(spec, unit) {
  if (!spec.func || CONTROL.test(unit.code)) return null;
  if (!unit.opens && !spec.indent) return null;
  const match = spec.func.exec(unit.code);
  return match ? match[1] : null;
}

// A `return` whose expression is tainted marks the function it sits in. The
// nearest *named* frame owns it — an `if` inside a function returns from the
// function — and the first such return is the one recorded, since one is
// already enough to make every call site tainted.
function recordReturn(spec, unit, scope, { funcs, found }) {
  if (!RETURN.test(unit.code)) return;
  let owner = null;
  for (let i = funcs.length - 1; i >= 0 && !owner; i -= 1) owner = funcs[i].name;
  if (!owner || found.has(owner)) return;
  const at = unit.code.indexOf('return') + 'return'.length;
  const code = unit.code.slice(at);
  // `return a;` — one name and nothing else — is most of the returns in most
  // files, and the full test would run eight patterns over it to arrive at the
  // one lookup that decides it. Answering it directly is what keeps the return
  // pass off the 2MB budget.
  const plain = PLAIN_VALUE.exec(code);
  const tainted = plain
    ? Boolean(builtAt(scope, plain[1]))
    : valueIsTainted(spec, scope, unit.text.slice(at), code);
  if (tainted) found.set(owner, unit.line);
}

// Every sink this statement matches. The SQL gate is asked only about a sink
// that actually matched, and only about the SQL id — the one whose verbs
// double as ordinary method names.
function collectSinks(spec, unit, scope, { state, ctx }) {
  for (const sink of spec.sinks) {
    const hit = sinkFinding(sink, unit, scope);
    if (hit && hit.id === SQL_ID && !isDatabaseCall(unit.text, ctx)) continue;
    collect(state, hit);
  }
}

// One forward pass. The sink is tested before the assignment on the same
// statement, so `q = q + x` reports against the value q already held.
//
// A SQL sink with no database evidence at the call and none in the file is not
// a SQL sink at all — `execute`, `query` and `Query` are ordinary method names,
// and a Command pattern or a job runner spells its entry point exactly that
// way. See isDatabaseCall for what counts as evidence and in what order.
//
// The pass also answers the question the *next* pass needs: which of this
// file's own functions return a value that is tainted where it is built.
// `open` frames stack per block; `funcs` stacks alongside them, with a null
// name for a block that is not a function, so a `return` is attributed to the
// nearest enclosing function rather than to the nearest brace.
function scan(spec, ctx, returns) {
  const scope = { vars: new Map(), open: [], consts: ctx.consts, returns };
  const state = { findings: [], seen: new Set() };
  const funcs = [];
  const found = new Map();

  for (const unit of ctx.units) {
    forget(scope.vars, scope.open, unit.level);
    while (funcs.length && funcs[funcs.length - 1].level >= unit.level) funcs.pop();

    collectSinks(spec, unit, scope, { state, ctx });
    recordReturn(spec, unit, scope, { funcs, found });
    // Only where the pack has no declarator — see outsideFunction.
    if (!spec.declare) scope.funcLevel = funcs.length ? funcs[funcs.length - 1].level : undefined;
    applyAssignment(spec, unit, scope);
    shadow(scope, unit, paramsOf(spec, unit));
    const name = functionName(spec, unit);
    if (name || unit.opens) funcs.push({ name, level: unit.level });
  }
  return { findings: state.findings, returns: found };
}

// Everything a pack's rules need about the whole file, computed once and shared
// by the line rules, the span rules and the taint scan: the statement units, the
// names bound only to constants, and whether the file contains SQL at all.
function packContext({ lines, stripped, spec }) {
  const guide = stripped || lines;
  const units = unitsOf(lines, guide, Boolean(spec.indent));
  return {
    units,
    consts: constantNames({ lines, stripped: guide, units, spec }),
    sql: hasSql(lines),
  };
}

// `existing` is the pack's own line-rule findings: where the inline rule
// already reported the same id on the same line, this adds nothing.
// Two passes at most: one to learn which of the file's own functions return a
// tainted value, one to use that. The second runs only when the first found
// one, so the common file — no such helper anywhere in it — still costs a
// single pass, and no file costs more than two whatever it contains. That is
// what keeps this a fixed factor rather than a fixpoint.
function taintFindings({ spec, ctx, existing = [] }) {
  const already = new Set(existing.map((f) => `${f.id}:${f.line}`));
  const first = scan(spec, ctx, new Map());
  const last = first.returns.size ? scan(spec, ctx, first.returns) : first;
  return last.findings.filter((f) => !already.has(`${f.id}:${f.line}`));
}

module.exports = {
  CONCAT, constantLine, packContext, skipConstant, taintFindings, valuePattern,
};
