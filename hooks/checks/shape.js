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

function analyzeBraces(source) {
  const lines = stripNoise(source).split(/\r?\n/);
  const stack = [];
  const blocks = [];
  let depth = 0;
  let maxDepth = 0;

  lines.forEach((line, index) => {
    for (const ch of line) {
      if (ch === '{') {
        depth += 1;
        maxDepth = Math.max(maxDepth, depth);
        stack.push(index + 1);
      } else if (ch === '}') {
        const startLine = stack.pop();
        depth = Math.max(0, depth - 1);
        if (startLine !== undefined) {
          blocks.push({ startLine, endLine: index + 1, length: index + 1 - startLine + 1 });
        }
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

module.exports = {
  stripNoise, analyzeBraces, analyzeIndent, countParams, estimateComplexity, shapeFindings,
};
