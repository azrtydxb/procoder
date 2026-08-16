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

function analyzeBraces(source) {
  const lines = stripNoise(source).split(/\r?\n/);
  const stack = [];
  const blocks = [];
  let depth = 0;
  let maxDepth = 0;

  lines.forEach((line, index) => {
    for (let i = 0; i < line.length; i += 1) {
      const ch = line[i];
      if (ch === '{') {
        const isBlock = !LITERAL_BRACE.test(line.slice(0, i).replace(/\s+$/, ''));
        if (isBlock) {
          depth += 1;
          maxDepth = Math.max(maxDepth, depth);
        }
        stack.push({ startLine: index + 1, isBlock });
      } else if (ch === '}') {
        const open = stack.pop();
        if (open === undefined) continue;
        if (open.isBlock) depth = Math.max(0, depth - 1);
        blocks.push({
          startLine: open.startLine,
          endLine: index + 1,
          length: index + 1 - open.startLine + 1,
        });
      }
    }
  });

  return { maxDepth, blocks };
}

function analyzeIndent(source, { tabWidth = 4 } = {}) {
  const lines = stripNoise(source).split(/\r?\n/);
  const blocks = [];
  let maxDepth = 0;
  let openBlock = null;

  lines.forEach((line, index) => {
    if (!line.trim()) return;
    const leading = /^[ \t]*/.exec(line)[0];
    const width = leading.replace(/\t/g, ' '.repeat(tabWidth)).length;
    const depth = Math.floor(width / tabWidth);
    maxDepth = Math.max(maxDepth, depth);

    if (/^\s*(?:def|class|async def)\s/.test(line)) {
      if (openBlock) {
        blocks.push({ ...openBlock, endLine: index, length: index - openBlock.startLine + 1 });
      }
      openBlock = { startLine: index + 1, baseDepth: depth };
    } else if (openBlock && depth <= openBlock.baseDepth) {
      blocks.push({ ...openBlock, endLine: index, length: index - openBlock.startLine + 1 });
      openBlock = null;
    }
  });

  if (openBlock) {
    blocks.push({ ...openBlock, endLine: lines.length, length: lines.length - openBlock.startLine + 1 });
  }

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

// Signature line number → its parameter text. Packs supply either a global
// regex scanned across the whole stripped source, or a line-anchored one that
// must be exec'd per line to stay clear of catastrophic backtracking.
function signaturesFrom(stripped, re) {
  const signatures = new Map();
  if (re.global) {
    for (const match of stripped.matchAll(re)) {
      signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
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
  return blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));
}

// A catch block whose body is nothing but whitespace or comments. The caller
// chooses the text: stripped source where comments should not rescue it, raw
// where the pattern spells out the comment forms itself.
function emptyCatchFindings(text, re, message) {
  return Array.from(text.matchAll(re), (match) => finding({
    rung: 'TRUE', id: 'true/swallowed-error',
    line: text.slice(0, match.index).split('\n').length,
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
