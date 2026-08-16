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

// True when the file indents some lines with tabs and others with spaces.
function mixesTabsAndSpaces(lines) {
  let tabs = false;
  let spaces = false;
  for (const line of lines) {
    if (!line.trim()) continue;
    const leading = /^[ \t]*/.exec(line)[0];
    tabs = tabs || leading.includes('\t');
    spaces = spaces || leading.includes(' ');
    if (tabs && spaces) return true;
  }
  return false;
}

// Depth for a file that mixes tabs and spaces, counted as changes of
// indentation rather than as multiples of tabWidth.
//
// The tab-width conversion answers "how many tabWidths wide is this line's
// indent", which is the nesting level only when every level is exactly one
// tabWidth. Mix a two-space level with a tab level and two real levels land in
// the same bucket: the file reports one level less than it has, and a function
// nested past the limit passes. Under-reporting is the wrong direction of
// error for a nesting check.
//
// Counting changes instead needs no guess at what a tab is worth — a line
// indented wider than the one enclosing it is one level deeper, whatever the
// indent is made of. Python 3 rejects such a file outright (TabError), so
// there is no "correct" tab width to recover; the enclosing/enclosed ordering
// is the only thing both readings agree on, and it is all a depth count needs.
//
// It runs only on mixed files. Consistent indentation — every real Python file
// a formatter has touched — keeps the tabWidth arithmetic byte for byte, so
// this can only change the reading of a file Python itself would refuse.
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
function bracketBalance(line) {
  let balance = 0;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === '(' || ch === '[' || ch === '{') balance += 1;
    else if (ch === ')' || ch === ']' || ch === '}') balance -= 1;
  }
  return balance;
}

// Indentation *is* the block structure, so this runs only for the indentation
// pack — Python — and says so: `#` opens a comment there, and a docstring is a
// comment spelled as a literal. The brace packs never reach it; they go through
// analyzeBraces and stripNoise, which keep the 'js' default.
function analyzeIndent(source, { tabWidth = 4 } = {}) {
  const lines = stripNoise(source, 'py').split(/\r?\n/);
  const blocks = [];
  const columns = [];
  const mixed = mixesTabsAndSpaces(lines);
  let maxDepth = 0;
  let openBlock = null;
  let open = 0;

  lines.forEach((line, index) => {
    if (!line.trim()) return;
    const leading = /^[ \t]*/.exec(line)[0];
    const column = leading.replace(/\t/g, ' '.repeat(tabWidth)).length;
    const depth = mixed ? changeDepth(columns, column) : Math.floor(column / tabWidth);
    maxDepth = Math.max(maxDepth, depth);

    const continuation = open > 0;
    open = Math.max(0, open + bracketBalance(line));
    if (continuation) return;

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

function countAt(frame, ch) {
  if (ch === ',' && frame.generic === 0) frame.commas += 1;
  else if (ch === '<') frame.generic += 1;
  else if (ch === '>' && frame.generic > 0) frame.generic -= 1;
}

function paramSpans(text) {
  const spans = new Map();
  const stack = [];
  const seen = (ch) => {
    const top = stack[stack.length - 1];
    if (!top || /\s/.test(ch)) return;
    top.last = ch;
    top.content = top.content || ch !== ',';
  };

  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (OPENS.includes(ch)) {
      seen(ch);
      stack.push({ ch, at: i, commas: 0, generic: 0, last: '', content: false });
    } else if (CLOSES.includes(ch)) {
      const frame = stack.pop();
      if (frame && frame.ch === '(') spans.set(frame.at, { end: i, params: frameParams(frame) });
      seen(ch);
    } else {
      const top = stack[stack.length - 1];
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

// Line number for a match offset. A global regex yields matches in increasing
// offset order, so one forward cursor over the newlines answers every call and
// the whole scan stays linear. Slicing the source per match instead — the
// obvious way — is quadratic, and a minified file is one long line where every
// slice spans the entire file.
function lineCounter(text) {
  let line = 1;
  let nextBreak = text.indexOf('\n');
  return (offset) => {
    while (nextBreak !== -1 && nextBreak < offset) {
      line += 1;
      nextBreak = text.indexOf('\n', nextBreak + 1);
    }
    return line;
  };
}

// How far back a block-opening `{` may look for the signature it belongs to.
// Prevailing formatters (prettier, black, gofmt, rustfmt) wrap one parameter
// per line once a signature exceeds the line width, so the span from the
// signature's first line to its brace costs one line per parameter plus the
// opening line, at most one line for a return type / `throws` / `where`, and
// the brace line. Ten reaches a seven-parameter wrap — already well past every
// params threshold worth setting (the default is 4), so the functions this
// exists to catch are inside the bound. A signature wrapped further stays
// unmeasured, exactly as every wrapped signature was before.
const SIGNATURE_LOOKBACK = 10;

// The most text a rescan may look at. The lookback is attempted for every
// block whose own line is not a signature — most blocks in a file — so its
// cost has to be constant per block, and one line may be the whole file.
// Capping the span keeps it constant. It is a cap on the *wrapped* case only:
// a signature on its own line is found by the pack's own scan, which has no
// ceiling at all, so a 60-parameter one-liner is measured however long it is.
// Within the ten-line lookback, 1000 characters is 100 per line, past any
// formatter's line width — the binding limit on a wrapped signature is
// SIGNATURE_LOOKBACK, not this.
const SIGNATURE_MAX_CHARS = 1000;

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
  return tail.exec(text) ? { params: span.params, end: tail.lastIndex } : null;
}

// Re-runs the pack's own signature pattern over a candidate line span,
// flattened to a single line. Flattening is the whole trick: every pack's
// pattern wants the parameter list and the brace on one line, which is why a
// wrapped signature matches none of them. The match must start at the span's
// first line and end at its brace, so a block is attributed only to a
// signature that actually begins there and is opened by that very brace —
// without the end anchor an `} else {` would match the `if` above it. The
// start test allows text before the match on that line, because a pack's
// pattern starts at `function`/`fn`/`func` and leaves `export`, `pub` or a
// decorator to its left.
function rescanner(stripped, head, tail) {
  // A copy, because the pack's pattern is a module-level global regex shared
  // by every file: exec'ing it here would leave `lastIndex` mid-source, and
  // matchAll seeds its own copy from it — the next file scanned would start
  // partway in and lose the signatures before that point.
  const scan = new RegExp(head.source, head.flags);
  let lines = null;
  return (from, to) => {
    if (!lines) lines = stripped.split(/\r?\n/);
    const span = lines.slice(from - 1, to);
    let size = 0;
    for (const line of span) size += line.length + 1;
    if (size > SIGNATURE_MAX_CHARS) return null;

    const flat = span.join(' ');
    scan.lastIndex = 0;
    const match = scan.exec(flat);
    if (!match || match.index >= span[0].length) return null;
    const signature = signatureAt(flat, paramSpans(flat), tail, match);
    if (!signature) return null;
    return signature.end === flat.trimEnd().length ? signature.params : null;
  };
}

// Signature line number → its parameter count. Packs supply a `{ head, tail }`
// pair: either a global head scanned across the whole stripped source, or a
// line-anchored one that must be exec'd per line to stay clear of catastrophic
// backtracking. The parameter counts come from one paramSpans pass over
// whatever text the head is scanned over, so they cost the same whether the
// file holds one signature or ten thousand.
//
// `rescan` rides along on the map because the packs pass this straight into
// measureFunctions and nowhere else: it is the one place that still holds both
// the stripped source and the pattern, and adding it here changes no pack.
function signaturesFrom(stripped, { head, tail }) {
  const signatures = new Map();
  // Sticky and its own copy, for the same reason the rescanner copies the head.
  const after = new RegExp(tail.source, tail.flags.includes('y') ? tail.flags : tail.flags + 'y');

  if (head.global) {
    const spans = paramSpans(stripped);
    const lineAt = lineCounter(stripped);
    for (const match of stripped.matchAll(head)) {
      const signature = signatureAt(stripped, spans, after, match);
      if (signature) signatures.set(lineAt(match.index), signature.params);
    }
  } else {
    stripped.split(/\r?\n/).forEach((line, index) => {
      const match = head.exec(line);
      const signature = match && signatureAt(line, paramSpans(line), after, match);
      if (signature) signatures.set(index + 1, signature.params);
    });
  }
  signatures.rescan = rescanner(stripped, head, after);
  return signatures;
}

// The signature a block belongs to: the one on its own opening line, or — when
// the signature wrapped — the nearest one starting within SIGNATURE_LOOKBACK
// lines above whose parameter list is still open at this brace. Reported at
// the signature's first line, so a finding points at the function rather than
// at its brace.
function signatureOf(block, signatures) {
  if (signatures.has(block.startLine)) {
    return { startLine: block.startLine, params: signatures.get(block.startLine) };
  }
  if (!signatures.rescan) return null;

  const stop = Math.max(1, block.startLine - SIGNATURE_LOOKBACK);
  for (let from = block.startLine - 1; from >= stop; from -= 1) {
    const params = signatures.rescan(from, block.startLine);
    if (params !== null) return { startLine: from, params };
  }
  return null;
}

// Attaches params and complexity to the blocks that start on a signature line,
// dropping the blocks that are not functions at all.
function measureFunctions(lines, blocks, signatures) {
  // One complexity scan per distinct line range, not per block. Every function
  // on a single minified line spans the same range, so N blocks over an L-byte
  // line cost one scan of L rather than N — the quadratic term, since scanning
  // a block costs its whole span. Counting branches per line over one strip of
  // the file would also be linear, but it moves the `?:` alternative's window
  // and changes reported complexity on ordinary multi-line code, so the scan
  // stays exactly the one it was.
  const scanned = new Map();
  const complexityOf = (block) => {
    const span = block.startLine + ':' + block.endLine;
    if (!scanned.has(span)) {
      scanned.set(span, estimateComplexity(
        lines.slice(block.startLine - 1, block.endLine).join('\n')));
    }
    return scanned.get(span);
  };

  const measured = [];
  for (const block of blocks) {
    const signature = signatureOf(block, signatures);
    if (!signature) continue;
    const span = {
      ...block,
      startLine: signature.startLine,
      length: block.endLine - signature.startLine + 1,
    };
    measured.push({ ...span, params: signature.params, complexity: complexityOf(span) });
  }
  return measured;
}

// A catch block whose body is nothing but whitespace or comments. The caller
// chooses the text: stripped source where comments should not rescue it, raw
// where the pattern spells out the comment forms itself.
function emptyCatchFindings(text, re, message) {
  const lineAt = lineCounter(text);
  return Array.from(text.matchAll(re), (match) => finding({
    rung: 'TRUE', id: 'true/swallowed-error',
    line: lineAt(match.index),
    message,
    fix: 'log with context and rethrow, or handle it explicitly',
  }));
}

module.exports = {
  SIGNATURE_LOOKBACK,
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
