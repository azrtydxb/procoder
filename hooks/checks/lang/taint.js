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
//   * One name at a time. No aliasing, no fields, no containers: `a = b` does
//     not carry b's taint, and `obj.q = tainted` is not tracked.
//   * A binding lives in the block it was assigned in and dies when that block
//     closes — brace depth for the five brace packs, indentation column for
//     Python. A sibling function reusing the name starts clean.
//   * A source is a string built from a *non-literal*: concatenation, a
//     template or f-string, `%`, `.format`, `Sprintf`, `String.format`,
//     `format!`. A value built only from literals is never tainted.
//   * Any assignment that is not a source clears the name, so a literal
//     reassignment untaints it.
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

function indentOf(line) {
  return /^[ \t]*/.exec(line)[0].replace(/\t/g, '    ').length;
}

function levelStep(ch, braces) {
  if (ch === '{') return braces + 1;
  if (ch === '}') return Math.max(0, braces - 1);
  return braces;
}

// One statement, the block level it sits at, and its line number.
function unitsOf(lines, stripped, indent) {
  const split = indent ? SEMI_SPLIT : BRACE_SPLIT;
  const units = [];
  let braces = 0;
  let level = 0;
  const push = (text, lineNo) => units.push({ text, level, line: lineNo });

  lines.forEach((line, index) => {
    if (indent && !line.trim()) return;
    const guide = stripped[index] === undefined ? line : stripped[index];
    level = indent ? indentOf(line) : braces;
    let from = 0;
    split.lastIndex = 0;
    for (let m = split.exec(guide); m; m = split.exec(guide)) {
      push(line.slice(from, m.index), index + 1);
      braces = levelStep(m[0], braces);
      if (!indent) level = braces;
      from = m.index + 1;
    }
    push(line.slice(from), index + 1);
  });
  return units;
}

// Bindings made deeper than the level now in scope are gone. A stack, so the
// whole scan stays linear in the number of assignments rather than costing a
// sweep of every live name per statement.
function forget(tainted, open, level) {
  while (open.length && open[open.length - 1].level > level) {
    for (const name of open.pop().names) tainted.delete(name);
  }
}

function remember(tainted, open, unit, name) {
  forget(tainted, open, unit.level);
  let top = open[open.length - 1];
  if (!top || top.level !== unit.level) {
    top = { level: unit.level, names: [] };
    open.push(top);
  }
  top.names.push(name);
  tainted.set(name, unit.line);
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

function applyAssignment(spec, unit, tainted, open) {
  const match = spec.assign.exec(unit.text);
  if (!match) return;
  const rhs = unit.text.slice(match.index + match[0].length);
  if (spec.sources.some((re) => re.test(rhs))) remember(tainted, open, unit, match[1]);
  else tainted.delete(match[1]);
}

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

module.exports = { CONCAT, taintFindings };
