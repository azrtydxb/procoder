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

function analyzeIndent(source, { tabWidth = 4 } = {}) {
  const lines = stripNoise(source).split(/\r?\n/);
  const blocks = [];
  let maxDepth = 0;
  let openBlock = null;

  lines.forEach((line, index) => {
    if (!line.trim()) return;
    const leading = /^[ \t]*/.exec(line)[0];
    const depth = Math.floor(leading.replace(/\t/g, ' '.repeat(tabWidth)).length / tabWidth);
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

function countParams(signatureText) {
  const inner = String(signatureText || '').replace(/^\s*\(|\)\s*$/g, '');
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

// Signature line number → its parameter text. Packs supply either a global
// regex scanned across the whole stripped source, or a line-anchored one that
// must be exec'd per line to stay clear of catastrophic backtracking.
function signaturesFrom(stripped, re) {
  const signatures = new Map();
  if (re.global) {
    const lineAt = lineCounter(stripped);
    for (const match of stripped.matchAll(re)) {
      signatures.set(lineAt(match.index), match[1]);
    }
    return signatures;
  }
  stripped.split(/\r?\n/).forEach((line, index) => {
    const match = re.exec(line);
    if (match) signatures.set(index + 1, match[1]);
  });
  return signatures;
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

  return blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: complexityOf(block),
    }));
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
