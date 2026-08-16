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
//   * One file, forward only. No call graph and no cross-file dataflow: that
//     needs a resolver per language and is out of proportion to a 2s hook.
//   * One name at a time. No fields and no containers: `obj.q = tainted` is
//     not tracked. Names do carry to names — see the propagation rule below.
//   * A binding lives in the block it was assigned in and dies when that block
//     closes — brace depth for the five brace packs, indentation column for
//     Python. A sibling function reusing the name starts clean, and so does a
//     *parameter* reusing it: a parameter list is a fresh binding, so the
//     names it introduces shadow whatever an enclosing scope bound them to for
//     as long as the block it opens is live.
//   * A source is a string built from a *non-literal*: concatenation, a
//     template or f-string, `%`, `.format`, `Sprintf`, `String.format`,
//     `format!`. A value built only from literals is never tainted.
//   * An assignment whose right-hand side names a tainted variable carries the
//     taint on. That is how a query is built incrementally — `q = q + id`,
//     `q += id`, `a = b` — and reading those as "not a source, therefore
//     clear" dropped the taint at the exact statement that introduced it. A
//     compound `+=` never clears either, since it reads the old value.
//   * Any other assignment clears the name, so a literal reassignment
//     untaints it.
//
// Everything outside that is a miss, and a miss is the direction this errs in
// on purpose: a tainted-variable rule that fires on safe code is worse than
// one that misses, because it is the whole pack that gets turned off. The
// clean fixtures are the guard — every case added here has its safe
// counterpart there.
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

const NAME = /[A-Za-z_$][\w$]*/g;

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

// One statement, the block level it sits at, and its line number. `text` is the
// raw slice, because the source patterns read the literal the value is built
// from; `code` is the same span with string contents blanked, which is what
// structure — an assignment's operator, a parameter list, the names on a
// right-hand side — is read off, so a word or a bracket inside a literal is
// never mistaken for one.
function unitsOf(lines, stripped, indent) {
  const split = indent ? SEMI_SPLIT : BRACE_SPLIT;
  const units = [];
  let braces = 0;
  let level = 0;
  const push = (text, code, lineNo) => units.push({ text, code, level, line: lineNo });

  lines.forEach((line, index) => {
    if (indent && !line.trim()) return;
    const guide = stripped[index] === undefined ? line : stripped[index];
    level = indent ? indentOf(line) : braces;
    let from = 0;
    split.lastIndex = 0;
    for (let m = split.exec(guide); m; m = split.exec(guide)) {
      push(line.slice(from, m.index), guide.slice(from, m.index), index + 1);
      braces = levelStep(m[0], braces);
      if (!indent) level = braces;
      from = m.index + 1;
    }
    push(line.slice(from), guide.slice(from), index + 1);
  });
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
function forget(tainted, open, level) {
  while (open.length && open[open.length - 1].level > level) {
    const frame = open.pop();
    for (let i = frame.names.length - 1; i >= 0; i -= 1) {
      const { name, prev } = frame.names[i];
      if (prev === undefined) tainted.delete(name);
      else tainted.set(name, prev);
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

function remember(tainted, open, unit, name) {
  forget(tainted, open, unit.level);
  frameAt(open, unit.level).names.push({ name, prev: tainted.get(name) });
  tainted.set(name, unit.line);
}

// A parameter list introduces fresh names, one level in from the statement it
// sits on — which is where the block it opens begins, whether that is the next
// brace level or the next indentation column. They start clean whatever an
// enclosing scope bound them to, and give the name back when the block closes.
function shadow(tainted, open, unit, names) {
  if (!names.length) return;
  const frame = frameAt(open, unit.level + 1);
  for (const name of names) {
    frame.names.push({ name, prev: tainted.get(name) });
    tainted.delete(name);
  }
}

// The parameter list a statement ends with, if it opens a block that binds.
function paramsOf(spec, unit) {
  if (NOT_A_BINDING.test(unit.code)) return [];
  const match = (spec.params || PARAM_LIST).exec(unit.code);
  return match ? namesIn(match[1]) : [];
}

// A sink whose argument is a name this scan has seen built from a non-literal.
// Every capture group is a candidate, which is how Go's `QueryContext(ctx, q)`
// is read as well as `Query(q)`.
function sinkFinding(sink, unit, tainted) {
  const match = sink.re.exec(unit.text);
  if (!match) return null;
  const name = match.slice(1).find((group) => group && tainted.has(group));
  if (!name) return null;
  return finding({
    rung: 'SAFE',
    id: sink.id,
    line: unit.line,
    message: `${sink.message}, built at line ${tainted.get(name)}`,
    fix: sink.fix,
  });
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
function applyAssignment(spec, unit, tainted, open) {
  const match = spec.assign.exec(unit.code);
  if (!match) return;
  const from = match.index + match[0].length;
  const code = unit.code.slice(from);
  const names = namesIn(code);
  const compound = match[0].includes('+=');

  if (spec.sources.some((re) => re.test(unit.text.slice(from)))
    || NAME_CONCAT.test(code)
    || names.some((name) => tainted.has(name))
    || (compound && names.length)) remember(tainted, open, unit, match[1]);
  else if (!compound) tainted.delete(match[1]);
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
const CONSTANT_WORD = new Set(['true', 'false', 'null', 'nil', 'none', 'undefined']);
const LITERAL_PREFIX = new Set(['b', 'r', 'u', 'rb', 'br', 'ur', 'l']);
const INTERPOLATION = /\$[{A-Za-z_]/;

// Index of the quote that closes the one at `from`, or the last character of
// the line when it never closes.
function stringEnd(line, from) {
  for (let i = from + 1; i < line.length; i += 1) {
    if (line[i] === '\\') i += 1;
    else if (line[i] === line[from]) return i;
  }
  return line.length - 1;
}

// Index of the first character at or after `from` that is not a space.
function skipSpace(line, from) {
  let i = from;
  while (line[i] === ' ' || line[i] === '\t') i += 1;
  return i;
}

// An identifier inside a call that is not itself a value: a keyword-argument
// name (`shell=True`), an object or path key (`shell: true`, `Command::new`),
// a language constant, or a non-interpolating string prefix.
function constantWord(word, line, after) {
  if (CONSTANT_WORD.has(word.toLowerCase())) return true;
  if (QUOTES.includes(line[after])) return LITERAL_PREFIX.has(word.toLowerCase());
  const at = skipSpace(line, after);
  return line[at] === ':' || (line[at] === '=' && line[at + 1] !== '=');
}

// Where the token starting at `at` ends, and whether it is data.
// An interpolating literal is data wherever it sits — `always` — because a
// rule may key on an assignment rather than on a call, and `Arguments = $"/c
// {cmd}"` has no enclosing call to be inside of. An identifier is read only
// inside a call, where it is an argument; outside one it is the statement's
// own callee path or its assignment target.
function stringToken(line, at) {
  const end = stringEnd(line, at);
  const body = line.slice(at + 1, end);
  return {
    end,
    always: true,
    data: INTERPOLATION.test(body) || (line[at - 1] === '$' && body.includes('{')),
  };
}

function wordToken(line, at) {
  let end = at + 1;
  while (end < line.length && WORD.test(line[end])) end += 1;
  return { end: end - 1, data: !constantWord(line.slice(at, end), line, end) };
}

function tokenAt(line, at) {
  if (QUOTES.includes(line[at])) return stringToken(line, at);
  if (WORD_START.test(line[at])) return wordToken(line, at);
  return null;
}

function constantLine(line) {
  let depth = 0;
  for (let i = 0; i < line.length; i += 1) {
    if (line[i] === '(') depth += 1;
    else if (line[i] === ')') depth = Math.max(0, depth - 1);
    else {
      const token = tokenAt(line, i);
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

// The discharge a pack hands to lineRuleFindings, and the same test spans.js
// applies to a span rule.
const skipConstant = (rule, line) => Boolean(rule.dataSink) && constantLine(line);

// One finding per id per line: two sinks on one line describe one hole.
function collect(state, found) {
  if (!found) return;
  const key = `${found.id}:${found.line}`;
  if (state.seen.has(key)) return;
  state.seen.add(key);
  state.findings.push(found);
}

// One forward pass. The sink is tested before the assignment on the same
// statement, so `q = q + x` reports against the value q already held.
function scan(units, spec) {
  const tainted = new Map();
  const open = [];
  const state = { findings: [], seen: new Set() };

  for (const unit of units) {
    forget(tainted, open, unit.level);
    for (const sink of spec.sinks) collect(state, sinkFinding(sink, unit, tainted));
    applyAssignment(spec, unit, tainted, open);
    shadow(tainted, open, unit, paramsOf(spec, unit));
  }
  return state.findings;
}

// `existing` is the pack's own line-rule findings: where the inline rule
// already reported the same id on the same line, this adds nothing.
function taintFindings({ lines, stripped, spec, existing = [] }) {
  const already = new Set(existing.map((f) => `${f.id}:${f.line}`));
  return scan(unitsOf(lines, stripped || lines, Boolean(spec.indent)), spec)
    .filter((f) => !already.has(`${f.id}:${f.line}`));
}

module.exports = { CONCAT, constantLine, skipConstant, taintFindings };
