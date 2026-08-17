#!/usr/bin/env node
// procoder — shape metrics shared by every language pack.
//
// Not a parser. A brace/indent counter that is right often enough to flag a
// 90-line function, and cheap enough to run inside a 2s hook budget.
//
// procoder: heuristic scanner, replace with a real parser per language if
// false positives on shape checks become the top complaint.

const { finding } = require('./finding');
const { stripComments } = require('./lang/comments');

const blankInside = (m) => m[0] + ' '.repeat(Math.max(0, m.length - 2)) + m[0];

// Removes string, comment and regex content so their braces do not count.
// Replaces rather than deletes, so line numbers survive.
//
// Comments and strings cannot be found by two independent passes in either
// order: a `"` inside a comment breaks a string-first pass, and a `/*` inside a
// string — a glob, a regex, a URL — breaks a comment-first one, blanking
// everything up to the next `*/` and fusing whole functions together. Only a
// single left-to-right scan that knows which construct it is inside gets both
// right, and comments.js already is that scan for the six packs, so it is
// reused rather than written a second time. It deliberately keeps string
// literals (the packs' sink rules need them); blanking those afterwards is
// safe, because no quote out of a comment or a regex survives its pass.
//
// `style` is comments.js's, and is what decides whether `#` opens a comment.
// This used to blank from any `#` to end of line in every language, which is
// right for Python, Ruby, shell, TOML and YAML and wrong for JS/TS, where `#`
// opens a private class member: `#wide(a, b, c) {` lost its parameters and its
// block-opening brace, so the method was measured as nothing at all. Asking
// comments.js instead is the same single language-aware pass the packs already
// use — a second `#` mechanism here is the duplicate logic rung 4 forbids.
function stripNoise(source, style = 'js') {
  return stripComments(source, style)
    .replace(/"(?:[^"\\\n]|\\.)*"/g, blankInside)
    .replace(/'(?:[^'\\\n]|\\.)*'/g, blankInside)
    .replace(/`(?:[^`\\]|\\.)*`/g, (m) => m.replace(/[^\n]/g, ' '));
}

// A `{` sitting in expression position opens a data literal, not a block. That
// distinction matters for depth: the fix for deep nesting is "invert the
// conditions into guard clauses", and an object literal passed as an argument
// has no condition to invert. Blocks — function bodies, callbacks, if/for/while
// — still count, so callback pyramids are reported as before.
const LITERAL_BRACE = /[(,=:[?&|!+]$|\breturn$/;

// `case 1: {` and `default: {` end in a colon too, but open a block. Without
// this a switch nested to depth 4 reports 3, and switch-heavy C#, Java and
// TypeScript under-report throughout. Plain `label:` statements are left
// alone: a line reading `  key:` is an object key far more often than a label,
// and treating it as a block would count data literals as nesting again.
const CASE_LABEL = /^\s*(?:case\b[^:]*|default)\s*:/;

// `const { a } = obj` and `import { x } from …` open a binding pattern, not a
// block. LITERAL_BRACE only catches literals in expression position — to the
// right of `=` — so a destructuring pattern, which sits to the left of it,
// slipped through and counted as a level of nesting. That reads a flat
// function as one deeper than it is, which is the wrong direction: it invents
// a rung-3 finding rather than missing one.
const BINDING_BRACE = /\b(?:const|let|var|import|export)\s*$/;

// Every brace on one line, in order, each tagged with whether it opens a block
// rather than a data literal.
//
// Both tests above only ever look at a bounded window, so neither needs the
// text before the brace materialised. LITERAL_BRACE is end-anchored and reaches
// at most `return` plus the character that gives it a word boundary: seven
// characters ending at the last non-space decide it. CASE_LABEL is start-
// anchored and must cover the whole prefix, so at most one offset per line can
// satisfy it — one exec finds that offset, lazily, and only for a brace whose
// prefix ends in the colon a label would leave there. Slicing the prefix per
// brace instead — the obvious way — is quadratic, and a minified file is one
// long line where every slice spans the entire file.
function bracesInLine(line, lineNo) {
  const braces = [];
  let caseColon;
  const labelOpensBlock = (at) => {
    if (caseColon === undefined) {
      const label = CASE_LABEL.exec(line);
      caseColon = label ? label[0].length - 1 : -1;
    }
    return at === caseColon;
  };

  let lastCode = -1;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === '}') braces.push({ open: false, lineNo });
    else if (ch === '{') {
      const before = line.slice(Math.max(0, lastCode - 6), lastCode + 1);
      const literal = LITERAL_BRACE.test(before) || BINDING_BRACE.test(before);
      braces.push({ open: true, lineNo, isBlock: !literal || labelOpensBlock(lastCode) });
    }
    if (!/\s/.test(ch)) lastCode = i;
  }
  return braces;
}

function analyzeBraces(source) {
  const braces = stripNoise(source).split(/\r?\n/)
    .flatMap((line, index) => bracesInLine(line, index + 1));
  const stack = [];
  const blocks = [];
  let depth = 0;
  let maxDepth = 0;

  for (const brace of braces) {
    if (brace.open) {
      if (brace.isBlock) maxDepth = Math.max(maxDepth, (depth += 1));
      stack.push(brace);
    } else if (stack.length) {
      const opened = stack.pop();
      if (opened.isBlock) depth = Math.max(0, depth - 1);
      blocks.push({
        startLine: opened.lineNo,
        endLine: brace.lineNo,
        length: brace.lineNo - opened.lineNo + 1,
      });
    }
  }

  return { maxDepth, blocks };
}

const DEF_OR_CLASS = /^\s*(?:def|class|async def)\s/;

function indentBlock(open, endLine) {
  return { ...open, endLine, length: endLine - open.startLine + 1 };
}

// Nesting depth as a stack of enclosing indentation columns: `columns` holds
// the column of every statement this one is nested inside, and its height *is*
// the depth. A statement indented wider than the one before opens a level, one
// indented back to or past an enclosing column closes every level it passed.
//
// This is what depth is — a property of the nesting structure — and it needs
// no unit at all. Depth used to be `column / step` for some step read off the
// file, which answers a different question: "how many steps wide is this
// indent". The two agree only when every level in the file is exactly one step
// wide, and they came apart in three ways, all real:
//
//   too wide  — a step had to be 8 columns or narrower to be a candidate at
//               all, so a file indented 9, 10, 12 or 16 columns per level fell
//               back to tabWidth and counted every real level twice or four
//               times. Correct code, reported as over-nested at rung 3.
//   two units — the step was one number for the whole file, the commonest one,
//               so a region indented differently was measured against someone
//               else's: a six-level two-space function in a four-space file
//               read as three and passed a limit of three. Silent, and the
//               worse half — a genuine violation reported as nothing.
//   tabs      — a tab level and a two-space level land in the same bucket, so
//               two real levels read as one. Python 3 rejects such a file
//               (TabError), so there is no "correct" tab width to recover.
//
// The ordering of enclosing to enclosed is the one thing every reading of a
// file agrees on, whatever its indentation is made of, and it is all a depth
// count needs. tabWidth survives with its own, narrower meaning — what a tab
// expands to — because ordering still has to compare a tab against spaces.
function changeDepth(columns, column) {
  while (columns.length && columns[columns.length - 1] >= column) columns.pop();
  if (column > 0) columns.push(column);
  return columns.length;
}

// A line inside an unclosed bracket continues the line above rather than
// starting a statement, and its indentation says nothing about nesting.
// Without that distinction a `def` whose parameters wrap ends its block on the
// `):` line — back at the def's own column — so every wrapped def measures as
// short as its signature, however long the body is. This is the indentation
// half of the brace languages' wrapped-signature gap.
//
// Python's other continuation is an explicit backslash, and it is the same
// case: `if a or \` puts the rest of one condition on the next line, aligned
// under the first, and CPython's idlelib/calltip_w.py aligns three such lines
// one column past the `if` — which counting them as statements reads as a level
// of nesting that is not there.
const LINE_CONTINUES = /\\\s*$/;

function bracketBalance(line) {
  let balance = 0;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === '(' || ch === '[' || ch === '{') balance += 1;
    else if (ch === ')' || ch === ']' || ch === '}') balance -= 1;
  }
  return balance;
}

// Indentation column of every line, with the lines that continue the line
// above marked and blank lines left out. A continuation's indentation is a
// hanging alignment under an open bracket: it says nothing about nesting, and
// nothing about what one level is worth either.
function indentRows(lines, tabWidth) {
  const rows = [];
  let open = 0;
  let continued = false;
  for (const line of lines) {
    if (!line.trim()) {
      rows.push(null);
      continue;
    }
    const leading = /^[ \t]*/.exec(line)[0];
    rows.push({
      column: leading.replace(/\t/g, ' '.repeat(tabWidth)).length,
      continuation: open > 0 || continued,
    });
    open = Math.max(0, open + bracketBalance(line));
    continued = LINE_CONTINUES.test(line);
  }
  return rows;
}

// Indentation *is* the block structure, so this runs only for the indentation
// pack — Python — and says so: `#` opens a comment there, and a docstring is a
// comment spelled as a literal. The brace packs never reach it; they go through
// analyzeBraces and stripNoise, which keep the 'js' default.
function analyzeIndent(source, { tabWidth = 4 } = {}) {
  const lines = stripNoise(source, 'py').split(/\r?\n/);
  const rows = indentRows(lines, tabWidth);
  const blocks = [];
  const columns = [];
  let maxDepth = 0;
  let openBlock = null;

  lines.forEach((line, index) => {
    const row = rows[index];
    // A continuation's column is an alignment under an open bracket, not a
    // level — `compute(a,` aligns its second argument under the first, at
    // whatever column that lands on, and counting it would read one call as
    // several levels of nesting. Statements are what nest, so only statements
    // are counted.
    if (!row || row.continuation) return;
    const depth = changeDepth(columns, row.column);
    maxDepth = Math.max(maxDepth, depth);

    if (DEF_OR_CLASS.test(line)) {
      if (openBlock) blocks.push(indentBlock(openBlock, index));
      openBlock = { startLine: index + 1, baseDepth: depth };
      return;
    }
    if (openBlock && depth <= openBlock.baseDepth) {
      blocks.push(indentBlock(openBlock, index));
      openBlock = null;
    }
  });

  if (openBlock) blocks.push(indentBlock(openBlock, lines.length));

  return { maxDepth, blocks };
}

// Where each `(` in a text closes, and how many top-level parameters it holds
// — every paren, in one left-to-right pass.
//
// The packs used to capture the parameter list as a *bounded span*
// (`([^)]{0,500})`) and count commas in the captured text. An unbounded
// `[^)]*` is retried from every signature start and, with no `)` ahead, runs
// to end of file each time — quadratic, and tens of seconds on a 400KB
// minified line. Bounding it made the scan linear and silently dropped every
// signature whose parameters ran past the bound: a 60-parameter function
// produced no finding at all where a 6-parameter one did, so
// obvious/too-many-params — and function-too-long and complexity with it,
// since an unmatched signature drops its whole block from measurement — lost
// precisely the functions they exist to report. The worse the code, the less
// likely it was seen.
//
// The checks only ever needed a *count*, never the text. Counting commas at
// bracket depth zero while scanning forward needs no captured span, and doing
// it for every paren in one pass costs O(source) per file however long,
// however many and however deeply nested the parameter lists are. There is no
// ceiling left to lose the extreme cases to.
//
// `<`/`>` are tracked per frame and clamped at zero, so `Map<string, number>`
// is one parameter while a stray `>` — `=>` in a default, a comparison —
// cannot drive the count below zero and swallow every comma after it.
const OPENS = '([{';
const CLOSES = ')]}';

// A list with nothing but separators in it has no parameters, and a trailing
// comma — which prettier, black, gofmt and rustfmt all add to a wrapped
// signature — closes the last parameter rather than opening another.
function frameParams(frame) {
  if (!frame.content) return 0;
  return frame.last === ',' ? frame.commas : frame.commas + 1;
}

// What `/\s/` matches, decided without a regex call. This is the hottest loop
// in the engine — once per character of every file every pack reads — and a
// regex test there was two thirds of its cost. ASCII whitespace is six
// comparisons; anything above ASCII is rare enough to still ask the regex, so
// the answer is the one `/\s/` gave, character for character.
function isSpace(ch) {
  return ch === ' ' || ch === '\n' || ch === '\t' || ch === '\r'
    || ch === '\f' || ch === '\v' || (ch > '\x7f' && /\s/.test(ch));
}

function countAt(frame, ch) {
  if (ch === ',' && frame.generic === 0) frame.commas += 1;
  else if (ch === '<') frame.generic += 1;
  else if (ch === '>' && frame.generic > 0) frame.generic -= 1;
}

// Closes the innermost frame, records its span when it was a parameter list,
// and answers the frame that is innermost now.
function closeFrame(stack, spans, at) {
  const frame = stack.pop();
  if (frame && frame.ch === '(') spans.set(frame.at, { end: at, params: frameParams(frame) });
  return stack.length ? stack[stack.length - 1] : null;
}

function paramSpans(text) {
  const spans = new Map();
  const stack = [];
  // The innermost frame, carried rather than looked up: this loop runs once per
  // character of every file the engine sees, and `stack[stack.length - 1]` on
  // each of them is the difference between 30ms and 200ms on a 2MB file.
  let top = null;
  // Whitespace never reaches here — the loop steps over it — so `seen` no
  // longer has to ask whether this character is any.
  const seen = (ch) => {
    if (!top) return;
    top.last = ch;
    top.content = top.content || ch !== ',';
  };

  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    // Whitespace is most of a source file and says nothing about any of the
    // three things this tracks, so it is stepped over before anything else
    // looks at it.
    if (isSpace(ch)) continue;
    if (OPENS.includes(ch)) {
      seen(ch);
      top = { ch, at: i, commas: 0, generic: 0, last: '', content: false };
      stack.push(top);
    } else if (CLOSES.includes(ch)) {
      top = closeFrame(stack, spans, i);
      seen(ch);
    } else {
      if (top) countAt(top, ch);
      seen(ch);
    }
  }
  return spans;
}

// Parameters in a signature's own text. The packs go through paramSpans
// directly; this is for the packs that already hold the text — Python's `def`,
// which has no brace to key on — and it is the same count by the same rule,
// not a second one.
//
// A wrapped signature almost always carries a trailing comma — prettier,
// black, gofmt and rustfmt all add one — and frameParams drops it, or every
// wrapped signature would report one parameter more than it has.
function countParams(signatureText) {
  const inner = String(signatureText || '').replace(/^\s*\(|\)\s*$/g, '');
  const span = paramSpans('(' + inner + ')').get(0);
  return span ? span.params : 0;
}

const BRANCH = /\b(?:if|else\s+if|elif|for|foreach|while|case|catch|except|when|rescue)\b|\?\s*[^:]+:|&&|\|\||\band\b|\bor\b/g;

function estimateComplexity(bodyText) {
  const matches = stripNoise(bodyText).match(BRANCH);
  return 1 + (matches ? matches.length : 0);
}

function shapeFindings({ blocks = [], maxDepth = 0, thresholds, kind = 'function' }) {
  const findings = [];

  for (const block of blocks) {
    if (block.length > thresholds.function_lines) {
      findings.push(finding({
        rung: 'OBVIOUS', id: 'obvious/function-too-long', line: block.startLine,
        message: `${kind} is ${block.length} lines (limit ${thresholds.function_lines})`,
        fix: 'extract the distinct steps into named functions',
      }));
    }
    if (block.params > thresholds.params) {
      findings.push(finding({
        rung: 'OBVIOUS', id: 'obvious/too-many-params', line: block.startLine,
        message: `${block.params} parameters (limit ${thresholds.params})`,
        fix: 'group them into an options object or struct',
      }));
    }
    if (block.complexity > thresholds.complexity) {
      findings.push(finding({
        rung: 'OBVIOUS', id: 'obvious/complexity', line: block.startLine,
        message: `cyclomatic complexity ~${block.complexity} (limit ${thresholds.complexity})`,
        fix: 'split the branches, or replace the chain with a lookup',
      }));
    }
  }

  if (maxDepth > thresholds.nesting_depth) {
    findings.push(finding({
      rung: 'OBVIOUS', id: 'obvious/nesting-depth', line: 1,
      message: `nesting depth ${maxDepth} (limit ${thresholds.nesting_depth})`,
      fix: 'invert the conditions into guard clauses and return early',
    }));
  }

  return findings;
}

// Applies a pack's line-rule table. `onCode` on a rule runs it against the
// noise-stripped line instead of the raw one: set it where the pattern
// describes code shape, so a regex literal or a string that merely quotes the
// pattern is not a hit. `skip(rule, line, lineNo)` lets a pack discharge a rule
// it has extra context for, such as a lookahead that finds the missing guard.
function lineRuleFindings(rules, lines, { codeLines = lines, skip } = {}) {
  const findings = [];
  lines.forEach((line, index) => {
    for (const rule of rules) {
      if (!rule.re.test(rule.onCode ? codeLines[index] : line)) continue;
      if (skip && skip(rule, line, index + 1)) continue;
      findings.push(finding({
        rung: rule.rung, id: rule.id, line: index + 1,
        message: rule.message, fix: rule.fix,
      }));
    }
  });
  return findings;
}

// How far past its parameter list's `)` a block-opening `{` may sit. This is a
// bound on the *tail* — a return type, `throws`, a `where` clause — and not on
// the wrap: the parameter list is matched to its own `)` by the single
// paramSpans pass over the file, so a signature wrapped over 25 or 500 lines is
// measured either way. That is the whole point of the split; the old ten-line
// lookback bounded the wrap itself, and one parameter per line is what every
// formatter produces, so it lost the functions with the most parameters —
// exactly what obvious/too-many-params exists to report.
//
// Four lines reaches rustfmt's widest shape: `) -> T`, `where`, one bound per
// line, `{`. 2000 characters is past any formatter's line width several times
// over, and is what keeps the work constant on a minified line, where the text
// after a `)` is the rest of the file: the budget is checked before any of it
// is materialised, so a candidate that cannot fit a tail costs one subtraction.
const TAIL_LOOKAHEAD_LINES = 4;
const TAIL_MAX_CHARS = 2000;

// Offset where each line starts. One pass to build, one binary search per
// lookup, and — unlike a forward cursor — it answers offsets in any order,
// which a signature scan needs: a callback nested inside a parameter list is
// matched after the signature that encloses it and starts before it ends.
function lineStarts(text) {
  const starts = [0];
  for (let at = text.indexOf('\n'); at !== -1; at = text.indexOf('\n', at + 1)) starts.push(at + 1);
  return starts;
}

function lineAtOffset(starts, offset) {
  let low = 0;
  let high = starts.length - 1;
  while (low < high) {
    const mid = (low + high + 1) >> 1;
    if (starts[mid] <= offset) low = mid;
    else high = mid - 1;
  }
  return low + 1;
}

// The line of the `{` a signature's tail reaches, or 0. The tail is re-applied
// to the text from the parameter list's `)` to the end of each candidate line
// in turn, newlines flattened to spaces and the window cut at its first `{` —
// which is what lets a pack's own tail work here unchanged, `$` anchors (Java,
// C#) and `[^{\n]` spans (Go, Rust) alike. Every pack writes its tail against a
// single line, because that is the only shape it ever saw before a signature
// was allowed to wrap.
//
// The cut used to be end of line, with the brace required to be the last thing
// in the window. That is the same line the `$` in Java's and C#'s tails is
// anchored to, so a method whose body opens and closes on the signature's own
// line — `public int size() { return n; }`, `public void noop() {}` — matched
// no tail at all, got no signature, and was dropped from measurement entirely:
// length, nesting, parameters and complexity all skipped it. Cutting at the
// brace instead asks the tail what it was always meant to ask — "does this
// reach the block-opening brace" — rather than "does the line end there". No
// pack's tail can cross a `{` (they are `[^{\n]`, `[^{=]` or literal keyword
// spans), so the first brace in the window is the only one any of them could
// ever have reached, and a brace that did end its line is matched exactly as
// before.
function braceLineAfter(text, starts, tail, from) {
  const first = lineAtOffset(starts, from);
  for (let line = first; line <= starts.length && line - first <= TAIL_LOOKAHEAD_LINES; line += 1) {
    const end = starts[line] === undefined ? text.length : starts[line] - 1;
    if (end - from > TAIL_MAX_CHARS) return 0;
    const span = text.slice(from, Math.max(from, end)).replace(/\n/g, ' ');
    const brace = span.indexOf('{');
    const window = brace === -1 ? span.trimEnd() : span.slice(0, brace + 1);
    tail.lastIndex = 0;
    if (tail.exec(window) && tail.lastIndex === window.length
      && !CONTROL_IN_TAIL.test(window)) return line;
  }
  return 0;
}

// `public int Size() => n;` — an expression-bodied declaration has no block at
// all, so analyzeBraces has nothing to hand it and every shape rule skipped it:
// measured directly, a C# method with 300 parameters reported nothing. It is
// one line by construction, so it is measured as a one-line block at its own
// line. Same family as the same-line body above — a declaration the recogniser
// dropped because it assumed a block on a later line — and the same answer.
//
// Only where the declaration starts its line and the statement ends on it. The
// packs whose head is scanned over the whole file match calls as readily as
// declarations — `arr.map((x) => x * 2)` is a head, a parameter list and an
// arrow — and synthesising a function there would hang the whole line's
// complexity on a callback, which is the false positive this pass exists to
// remove. A statement that begins at the margin and ends in `;` is a
// declaration in the languages that have this form.
const EXPRESSION_BODY = /^\s*(?:=>|->)\s*[^;]*;\s*$/;

// Nothing but indentation before `at` on its line. Bounded by the same budget
// as the tail, so a minified line — where a head can sit half a megabyte into
// its only line — costs one subtraction rather than a scan of the file.
function atMargin(text, lineStart, at) {
  if (at - lineStart > TAIL_MAX_CHARS) return false;
  for (let i = at - 1; i >= lineStart; i -= 1) if (!/\s/.test(text[i])) return false;
  return true;
}

function isExpressionBody(text, starts, { startLine, headAt }, from) {
  if (!atMargin(text, starts[startLine - 1], headAt)) return false;
  const end = starts[startLine] === undefined ? text.length : starts[startLine] - 1;
  if (end - from > TAIL_MAX_CHARS) return false;
  return EXPRESSION_BODY.test(text.slice(from, Math.max(from, end)));
}

// A pack's signature pattern, applied at one offset: the head match locates
// the parameter list's `(`, paramSpans says where it closes and how many
// parameters it holds, and the tail — sticky, so it can only match right where
// the list closed — carries on through the return type to the block-opening
// `{`. Splitting the pattern there is what removes the ceiling: the head and
// tail are short and bounded, and nothing has to capture the parameters.
function signatureAt(text, spans, tail, match) {
  const span = spans.get(match.index + match[0].length - 1);
  if (!span) return null;
  tail.lastIndex = span.end + 1;
  if (!tail.exec(text)) return null;
  // What the tail crossed to get to the brace has to be a tail and not a
  // statement — see CONTROL_IN_TAIL.
  if (CONTROL_IN_TAIL.test(text.slice(span.end + 1, tail.lastIndex))) return null;
  return { params: span.params, end: tail.lastIndex };
}

// Signature line number → its parameter count. Packs supply a `{ head, tail }`
// pair: either a global head scanned across the whole stripped source, or a
// line-anchored one that must be exec'd per line to stay clear of catastrophic
// backtracking. The parameter counts come from one paramSpans pass over
// whatever text the head is scanned over, so they cost the same whether the
// file holds one signature or ten thousand.
//
// `wrapped` rides along on the map — brace line → the signature that brace
// opens — because the packs pass this straight into measureFunctions and
// nowhere else: it is the one place that holds both the stripped source and
// the pattern, and adding it here changes no pack.
//
// The wrapped case used to be answered the other way round, by walking back up
// to ten lines from every unattributed brace and re-running the head over the
// flattened span. That search is what carried the bound, and the bound is what
// dropped the most-wrapped — so the most-parametered — signatures. Going
// forward from the head instead needs no search: the parameter list's `)` is
// already known from the paramSpans pass, whatever line it landed on, and only
// the short tail after it has to be re-read.
// Every head in the file, as { startLine, headAt, parenAt, params, braceLine },
// with braceLine 0 where the pack's own tail did not reach a brace. A global
// head is scanned over the whole source and its tail may cross lines; a
// line-anchored one is exec'd per line — at most one match per line, as before
// — and its tail is read on that line alone, so a brace it reaches is on that
// line. `headAt` is where the head itself starts, which is what says whether a
// declaration begins at the margin.
function headMatch(where, found, braceLine) {
  return {
    ...where,
    params: found ? found.params : 0,
    braceLine: found ? braceLine : 0,
  };
}

// `switch (kind) {`, `if (ok) {`, `while (more) {`, `} catch (err) {` — every
// control-flow keyword that takes a parenthesised head has exactly the shape a
// pack's signature head looks for, `<name>(args) {`. A function whose body
// opened with one was measured a second time at the keyword's own line, so one
// function reported at two line numbers; `switch` is the one an adversarial
// false-positive hunt caught, and every sibling keyword had it too. The
// line-anchored heads (Java, C#) want two identifiers before the parens and
// find them in `else if (…) {`.
//
// Swept here, in the one scan every pack routes through, rather than in six
// heads that would each have to grow the same negative lookahead — and this is
// the only place that holds both the head's text and the character before it.
//
// Statement position only. A keyword reached through a member access is a
// method: `p.catch(err => { … })`, `s.match(re)`, `list.for(…)` are ordinary
// code and are still measured. `.`, `?.`, `::` and `->` all end in one of these.
//
// Two things narrow it, because one global keyword list applied to every pack
// took a function *named* after one of these words out of measurement
// altogether — not length, not nesting, not parameters, not complexity. That is
// silent coverage loss, and it hit five of the six packs: `match(pattern,
// input)` in a parser, `lock(resource)` in a scheduler, `using(handle)`,
// `when(condition)` are ordinary names, and in the languages where the word is
// not reserved at all they are legal ones.
//
// First: the keyword has to be the *whole* head. A declaration puts something
// in front of the name — `function`, `func`, `fn`, an access modifier, a return
// type — and every pack's head captures that prefix, so a head that is a bare
// keyword and a paren is the only one that can be a statement. `else if (…)` is
// the one two-token statement any pack's head reaches (Java's and C#'s want
// `<type> <name>` and find `else` and `if`), so `else` is allowed in front of
// the word and nothing else is. That single test clears four packs outright:
// Go's head demands `func`, Rust's `fn`, Java's and C#'s two tokens, so
// `func lock(…)`, `fn lock(…)`, `public int lock(…)` and `public int when(…)`
// are declarations by construction and never reach the list.
//
// Second: the list is only the words that are control flow in the language with
// a bare-name head — JavaScript and TypeScript, whose pattern matches a method
// shorthand `match(a, b) {` with nothing in front of it at all. There the shape
// cannot decide it and the language must: JS has no `match`, `when`, `lock`,
// `using`, `unless`, `until`, `foreach`, `except`, `rescue` or `synchronized`
// statement, so those words are names there and are measured. The words that
// stay are JS's own reserved control-flow keywords, which is what the
// false-positive sweep was ever about — `switch (…) {`, `if (…) {`,
// `while (…) {`, `for (…) {`, `catch (…) {`, `case (…):` and `with (…) {`.
//
// The dropped words cannot cost the other packs a false positive, because none
// of their heads can match those statements in the first place: C#'s
// `foreach (…)`, `using (…)` and `lock (…)`, Java's `synchronized (…)` and
// Kotlin's `when (…)` are one token and a paren, and the line-anchored heads
// need two. Rust's `match x {` and `match (a, b) {` carry no `fn`. Python never
// comes through here — py.js measures by indentation off `def`, which is why it
// was the one pack that already got this right.
const CONTROL_HEAD = /^\s*(?:else\s+)?(if|for|while|switch|case|catch|with)\s*\($/;
const MEMBER_ACCESS = /[.:>]/;

// The same keywords again, anywhere in the text a tail crossed to reach its
// brace. A tail is a return type, a `throws`, a `where` clause — never a
// statement — so a control-flow keyword in it means the brace belongs to that
// keyword's block and not to the signature. Without this the sweep above only
// moves the finding: a bare parenthesised expression is a head to the ts
// pattern, and its tail (`: <return type>` for up to 500 characters, newlines
// included) reaches down to the `{` of an `if` four lines below, which the
// `if`'s own signature used to mask. Reported at the expression's line, which
// is not a function at all. Go's `(int, error)` tail, Rust's `where`, Java's
// `throws` and C#'s `: base(…)` all pass it unchanged.
const CONTROL_IN_TAIL = /(?:^|[^\w$.])(?:if|else|elif|elsif|for|foreach|while|until|unless|switch|case|when|match|catch|except|rescue|do|try|using|lock|synchronized|with)\s*[({]/;

// The character that reaches a name, spaces skipped. Bounded, so a minified
// line — where the run before a token can be the whole file — costs one look:
// no formatter puts four spaces between `.` and a method name.
function reachedBy(text, at) {
  for (let i = at - 1; i >= 0 && at - i <= 4; i -= 1) {
    if (!/\s/.test(text[i])) return text[i];
  }
  return '';
}

function isControlHead(text, at, headText) {
  const keyword = CONTROL_HEAD.exec(headText);
  if (!keyword) return false;
  return !MEMBER_ACCESS.test(reachedBy(text, at + keyword.index + keyword[0].indexOf(keyword[1])));
}

// The one case the refusal above gets wrong, because the line it is anchored to
// does not carry the answer. A JS method shorthand puts *nothing* in front of
// the name — `class Parser { with(a, b, c, d, e, f) { … } }`,
// `const p = { catch(a, b, c, d, e, f) { … } }` — which is exactly the shape the
// refusal rejects, so a method named after a JS statement was measured for
// nothing at all: not length, not nesting, not parameters, not complexity.
// `with` and `catch` are the realistic ones (a parser, a promise-like, a
// builder) and `case`, `when` and `for` appear as method names too.
//
// `catch (e) {` at statement position inside a function body and `catch(e) {` as
// a member of a class body are the same text; what differs is the brace they sit
// directly inside. A `{` that opens a class body or an object literal holds
// *members*, and a member is a declaration however it is named; every other `{`
// opens a block, and a block holds statements. That is the whole distinction,
// and it needs only what a single left-to-right pass already knows: the brace
// stack, the character that reached each brace, and whether a `class` keyword or
// a `case` label stands between the last statement delimiter and it.
//
// It is exactly as reliable as the brace stack itself — a heuristic scanner over
// noise-stripped text, the same one every other measurement here rests on. Two
// shapes it deliberately reads as blocks rather than members, because reading
// them the other way would let a statement be measured as a function again and
// that is the error this whole area exists to prevent:
//
//   `case 'a': {`  — a colon before the brace is what an object literal's nested
//                    value also leaves there, and a case block is full of
//                    statements. A `case`/`default` since the last delimiter
//                    wins, so a switch — where the 104 false positives came from
//                    — stays a switch, at the cost of not measuring a method of
//                    an object literal nested inside a switch case.
//   `static { … }` — a class-body brace is recognised by a `class`/`interface`
//                    keyword on the same line with no delimiter after it, so a
//                    static initialiser's own brace is a block, as it should be.
//
// `node.class` and `mod.default` are property reads, not declarations, so a
// keyword reached through a member access does not count.
const BODY_WORD = /\b(?:class|interface|case|default)\b/g;
const LABEL_WORD = /^(?:case|default)$/;

function bodyWords(text) {
  const words = [];
  for (const match of text.matchAll(BODY_WORD)) {
    if (reachedBy(text, match.index) !== '.') words.push(match);
  }
  return words;
}

// The `class`/`interface` declaration and the `case`/`default` label that most
// recently began, as offsets, up to where the scan has reached.
function noteWords(state, words) {
  while (state.next < words.length && words[state.next].index <= state.at) {
    const word = words[state.next];
    if (LABEL_WORD.test(word[0])) state.labelled = word.index;
    else state.declared = word.index;
    state.next += 1;
  }
}

// `{` reached by `=`, `(`, `,` and their siblings is a data literal, which holds
// members — unless a `case` label is what put the colon there. Otherwise it is a
// block, and only a `class`/`interface` still open on this line makes it a body.
function opensMembers(text, state) {
  const before = text.slice(Math.max(0, state.code - 6), state.code + 1);
  if (BINDING_BRACE.test(before)) return false;
  if (LITERAL_BRACE.test(before)) return state.labelled <= state.delim;
  return state.declared >= state.lineStart && state.declared > state.delim;
}

function scanChar(text, state, stack, words) {
  const ch = text[state.at];
  noteWords(state, words);
  if (ch === '{') stack.push(opensMembers(text, state));
  else if (ch === '}') stack.pop();
  if (ch === '\n') state.lineStart = state.at + 1;
  if (ch === ';' || ch === '{' || ch === '}') state.delim = state.at;
  if (!isSpace(ch)) state.code = state.at;
}

// Answers "is the brace enclosing this offset one that holds members?" for
// offsets asked in increasing order — which is the order a global head scan
// produces. The cursor only ever moves forward, so every character of the file
// is visited at most once however many heads ask, and a file whose heads are
// never bare keywords is never scanned at all.
//
// Only the global heads consult it. A line-anchored head (Java, C#) wants two
// tokens before the parens and the only statement it can reach is `else if (…)`,
// which is a statement in a class body as much as anywhere else.
function memberScanner(text) {
  const stack = [];
  const state = {
    at: 0, next: 0, lineStart: 0, delim: -1, declared: -1, labelled: -1, code: -1,
  };
  // Built on the first question rather than up front: most files hold no bare
  // keyword head at all, and those never pay for either pass.
  let words = null;
  return (offset) => {
    if (!words) words = bodyWords(text);
    for (; state.at < offset && state.at < text.length; state.at += 1) {
      scanChar(text, state, stack, words);
    }
    return stack.length > 0 && stack[stack.length - 1];
  };
}

function* headMatches(stripped, head, { spans, starts, after, member }) {
  if (head.global) {
    for (const match of stripped.matchAll(head)) {
      if (isControlHead(stripped, match.index, match[0]) && !member(match.index)) continue;
      const found = signatureAt(stripped, spans, after, match);
      yield headMatch({
        startLine: lineAtOffset(starts, match.index),
        headAt: match.index,
        parenAt: match.index + match[0].length - 1,
      }, found, found && lineAtOffset(starts, found.end - 1));
    }
    return;
  }
  const lines = stripped.split(/\r?\n/);
  for (let index = 0; index < lines.length; index += 1) {
    const match = head.exec(lines[index]);
    if (!match) continue;
    if (isControlHead(lines[index], match.index, match[0])) continue;
    yield headMatch({
      startLine: index + 1,
      headAt: starts[index] + match.index,
      parenAt: starts[index] + match.index + match[0].length - 1,
    }, signatureAt(lines[index], paramSpans(lines[index]), after, match), index + 1);
  }
}

// Nearest wins, as the backward walk's first hit did: of two signatures whose
// braces land on one line, the inner one describes that block better.
function attribute(wrapped, braceLine, startLine, params) {
  const held = wrapped.get(braceLine);
  if (braceLine > startLine && (!held || held.startLine < startLine)) {
    wrapped.set(braceLine, { startLine, params });
  }
}

// One head, once the pack's own tail has had its say. The tail stopped at a
// line end (Java, C#) or refused to cross one (Go, Rust); where the parameter
// list closed is known whatever it spanned, so only the short tail after it has
// to be re-read.
function recordHead(maps, stripped, at, found) {
  const span = at.spans.get(found.parenAt);
  if (!span) return;
  const reached = braceLineAfter(stripped, at.starts, at.after, span.end + 1);
  // A brace on the signature's own line opens that signature's own block — the
  // same-line body — and is not a wrap at all; `attribute` deliberately keeps
  // only braces below the signature, so it would drop this one.
  if (reached === found.startLine) maps.signatures.set(found.startLine, span.params);
  else if (reached) attribute(maps.wrapped, reached, found.startLine, span.params);
  else if (isExpressionBody(stripped, at.starts, found, span.end + 1)) {
    maps.bodyless.set(found.startLine, span.params);
  }
}

// `fn r#match(a: i32) -> i32 {`, `public int @match(int a) {` — Rust and C#
// spell an identifier that collides with a keyword by prefixing it, and the
// prefix is not a word character. Every pack's name pattern is `\w+`, so the
// name did not match, the head did not match, and the declaration was measured
// for nothing at all — even though the `fn` and the `public int` in front of it
// make it a declaration by construction. Rust's `r#match` and C#'s `@match` are
// the two: Go, Java and Python have no such spelling, and JS/TS has none either
// (a keyword is already a legal member name there, which is DEFECT 1 above).
// Kotlin's backtick identifiers are a third, and are *not* fixed here: a
// backticked name may contain spaces, so `\w+` cannot match one however this
// text is normalised, and widening it belongs to the jvm pack's head.
//
// Blanked rather than deleted, so every offset and line number this scan
// reports is still the source's own — the same rule stripNoise follows.
//
// The lookahead is what keeps a raw *string* out of it: Rust's `r#"…"#` and C#'s
// `@"…"` are followed by a quote, never by an identifier character.
const RAW_IDENT = /\br#(?=[A-Za-z_])|@(?=[A-Za-z_])/g;

const plainNames = (text) => text.replace(RAW_IDENT, (m) => ' '.repeat(m.length));

function signaturesFrom(source, { head, tail }) {
  const stripped = plainNames(source);
  const signatures = new Map();
  // Declarations with no block at all — an expression body — keyed by their
  // line. measureFunctions gives each a one-line span, because there is no
  // brace for analyzeBraces to have found one.
  const maps = { signatures, wrapped: new Map(), bodyless: new Map() };
  const at = {
    spans: paramSpans(stripped),
    starts: lineStarts(stripped),
    // Sticky, and its own copy: the pack's pattern is a module-level regex
    // shared by every file, and exec'ing it here would leave lastIndex adrift.
    after: new RegExp(tail.source, tail.flags.includes('y') ? tail.flags : tail.flags + 'y'),
    member: memberScanner(stripped),
  };

  for (const found of headMatches(stripped, head, at)) {
    if (found.braceLine) {
      signatures.set(found.startLine, found.params);
      attribute(maps.wrapped, found.braceLine, found.startLine, found.params);
    } else recordHead(maps, stripped, at, found);
  }
  signatures.wrapped = maps.wrapped;
  signatures.bodyless = maps.bodyless;
  return signatures;
}

// The signature a block belongs to: the one on its own opening line, or — when
// the signature wrapped — the one whose tail reached this very brace. Reported
// at the signature's first line, so a finding points at the function rather
// than at its brace. Requiring that brace is what keeps `} else {` from being
// attributed to the `if` above it.
function signatureOf(block, signatures) {
  if (signatures.has(block.startLine)) {
    return { startLine: block.startLine, params: signatures.get(block.startLine) };
  }
  return (signatures.wrapped && signatures.wrapped.get(block.startLine)) || null;
}

// One complexity scan per distinct line range, not per block. Every function on
// a single minified line spans the same range, so N blocks over an L-byte line
// cost one scan of L rather than N — the quadratic term, since scanning a block
// costs its whole span. Counting branches per line over one strip of the file
// would also be linear, but it moves the `?:` alternative's window and changes
// reported complexity on ordinary multi-line code, so the scan stays exactly
// the one it was.
function complexityScanner(lines) {
  const scanned = new Map();
  return (block) => {
    const span = block.startLine + ':' + block.endLine;
    if (!scanned.has(span)) {
      scanned.set(span, estimateComplexity(
        lines.slice(block.startLine - 1, block.endLine).join('\n')));
    }
    return scanned.get(span);
  };
}

// Attaches params and complexity to the blocks that start on a signature line,
// dropping the blocks that are not functions at all.
//
// One span per signature, keyed by its line, and the widest of them wins.
//
// A block is attributed to a signature by the line its brace opens on, and more
// than one block can open on that line: `function outer(a) { if (a) {` hands the
// same signature two blocks, and both reported — one function,
// obvious/function-too-long twice, at one line. A `switch` opening a body did
// the same at two different lines, and any head that ever matches something
// extra on a signature's own line will do it again. There is only ever one
// function per signature line — `signatures` is keyed by line and holds one
// parameter count for it — so a second measurement of that line is a duplicate
// by construction, and this is where it stops being able to reach shapeFindings
// at all, rather than being filtered downstream where the next caller would
// have to remember to filter too. The widest span is the function's own block;
// a narrower one is something nested inside it.
function measureFunctions(lines, blocks, signatures) {
  const complexityOf = complexityScanner(lines);
  const measured = new Map();
  const keep = (span, params) => {
    const held = measured.get(span.startLine);
    if (held && held.length >= span.length) return;
    measured.set(span.startLine, { ...span, params, complexity: complexityOf(span) });
  };

  for (const block of blocks) {
    const signature = signatureOf(block, signatures);
    if (!signature) continue;
    keep({
      ...block,
      startLine: signature.startLine,
      length: block.endLine - signature.startLine + 1,
    }, signature.params);
  }
  // An expression body has no brace, so analyzeBraces found no block for it at
  // all: it is one line, at its own line.
  for (const [line, params] of signatures.bodyless || []) {
    if (!measured.has(line)) keep({ startLine: line, endLine: line, length: 1 }, params);
  }
  return [...measured.values()];
}

// A catch block whose body is nothing but whitespace or comments. The caller
// chooses the text: stripped source where comments should not rescue it, raw
// where the pattern spells out the comment forms itself.
function emptyCatchFindings(text, re, message) {
  const starts = lineStarts(text);
  return Array.from(text.matchAll(re), (match) => finding({
    rung: 'TRUE', id: 'true/swallowed-error',
    line: lineAtOffset(starts, match.index),
    message,
    fix: 'log with context and rethrow, or handle it explicitly',
  }));
}

module.exports = {
  stripNoise,
  analyzeBraces,
  analyzeIndent,
  countParams,
  paramSpans,
  estimateComplexity,
  shapeFindings,
  lineRuleFindings,
  signaturesFrom,
  measureFunctions,
  emptyCatchFindings,
};
