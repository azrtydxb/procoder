#!/usr/bin/env node
// procoder — shape metrics shared by every language pack.
//
// Not a parser. A brace/indent counter that is right often enough to flag a
// 90-line function, and cheap enough to run inside a 2s hook budget.
//
// procoder: heuristic scanner, replace with a real parser per language if
// false positives on shape checks become the top complaint.

const { finding } = require('./finding');

// A regex literal, recognised only where an expression may start — after an
// operator, an opening bracket, a comma, a colon or a statement end. That
// restriction is what keeps `a / b / c` from being read as a regex. Its body
// is quantifier braces and alternation bars, not code: counted as structure it
// reports nesting and branching that the file does not contain.
const REGEX_LITERAL =
  /(^|[(,=:[!&|?{};])(\s*)\/((?:[^/\\\n[]|\\.|\[(?:[^\]\\\n]|\\.)*\])+)\/([a-z]*)/g;

// Removes string, comment and regex content so their braces do not count.
// Replaces rather than deletes, so line numbers survive.
function stripNoise(source) {
  return String(source || '')
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p) => p + m.slice(p.length).replace(/./g, ' '))
    .replace(REGEX_LITERAL, (m, pre, gap, body, flags) =>
      pre + gap + '/' + ' '.repeat(body.length) + '/' + flags)
    .replace(/#[^\n]*/g, (m) => m.replace(/./g, ' '))
    .replace(/"(?:[^"\\\n]|\\.)*"/g, (m) => '"' + ' '.repeat(Math.max(0, m.length - 2)) + '"')
    .replace(/'(?:[^'\\\n]|\\.)*'/g, (m) => "'" + ' '.repeat(Math.max(0, m.length - 2)) + "'")
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
      const literal = LITERAL_BRACE.test(line.slice(Math.max(0, lastCode - 6), lastCode + 1));
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

function analyzeIndent(source, { tabWidth = 4 } = {}) {
  const lines = stripNoise(source).split(/\r?\n/);
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

// A wrapped signature almost always carries a trailing comma — prettier,
// black, gofmt and rustfmt all add one — so it is dropped before counting,
// or every wrapped signature reports one parameter more than it has.
function countParams(signatureText) {
  const inner = String(signatureText || '')
    .replace(/^\s*\(|\)\s*$/g, '')
    .replace(/,\s*$/, '');
  if (!inner.trim()) return 0;

  let depth = 0;
  let count = 1;
  for (const ch of inner) {
    if ('([{<'.includes(ch)) depth += 1;
    else if (')]}>'.includes(ch)) depth -= 1;
    else if (ch === ',' && depth === 0) count += 1;
  }
  return count;
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
// Capping the span keeps it constant; the packs already bound their parameter
// text to 500 characters, so no real signature is excluded.
const SIGNATURE_MAX_CHARS = 1000;

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
function rescanner(stripped, re) {
  // A copy, because the pack's pattern is a module-level global regex shared
  // by every file: exec'ing it here would leave `lastIndex` mid-source, and
  // matchAll seeds its own copy from it — the next file scanned would start
  // partway in and lose the signatures before that point.
  const scan = new RegExp(re.source, re.flags);
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
    return match.index + match[0].length === flat.trimEnd().length ? match[1] : null;
  };
}

// Signature line number → its parameter text. Packs supply either a global
// regex scanned across the whole stripped source, or a line-anchored one that
// must be exec'd per line to stay clear of catastrophic backtracking.
//
// `rescan` rides along on the map because the packs pass this straight into
// measureFunctions and nowhere else: it is the one place that still holds both
// the stripped source and the pattern, and adding it here changes no pack.
function signaturesFrom(stripped, re) {
  const signatures = new Map();
  if (re.global) {
    const lineAt = lineCounter(stripped);
    for (const match of stripped.matchAll(re)) {
      signatures.set(lineAt(match.index), match[1]);
    }
  } else {
    stripped.split(/\r?\n/).forEach((line, index) => {
      const match = re.exec(line);
      if (match) signatures.set(index + 1, match[1]);
    });
  }
  signatures.rescan = rescanner(stripped, re);
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
    measured.push({
      ...span,
      params: countParams('(' + signature.params + ')'),
      complexity: complexityOf(span),
    });
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
  estimateComplexity,
  shapeFindings,
  lineRuleFindings,
  signaturesFrom,
  measureFunctions,
  emptyCatchFindings,
};
