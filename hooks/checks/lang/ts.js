#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs'];

// `onCode` runs the rule against the noise-stripped line instead of the raw one.
// Set it where the pattern describes code shape rather than string content, so a
// regex literal or a string that merely quotes the pattern is not a hit. Rules
// that must see literal text (SQL built inside a template string) leave it off.
const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:query|execute|raw|exec)\s*\(\s*(?:`[^`]*\$\{|["'][^"']*["']\s*\+)/i,
    message: 'SQL built by interpolation or concatenation',
    fix: 'use a parameterized query with bound values',
  },
  {
    id: 'safe/xss-sink', rung: 'SAFE', onCode: true,
    re: /\.innerHTML\s*=|\.outerHTML\s*=|dangerouslySetInnerHTML|document\.write\s*\(/,
    message: 'raw HTML sink',
    fix: 'use textContent, or sanitize before assigning',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE',
    re: /\beval\s*\(|new\s+Function\s*\(|setTimeout\s*\(\s*["'`]/,
    message: 'dynamic code evaluation',
    fix: 'replace with a lookup table or a direct call',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /rejectUnauthorized\s*:\s*false|NODE_TLS_REJECT_UNAUTHORIZED\s*=\s*["']?0/,
    message: 'TLS certificate verification disabled',
    fix: 'trust the proper CA instead of disabling verification',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|otp|session[_-]?id|password)\b[^\n]{0,40}Math\.random\s*\(/i,
    message: 'Math.random() used for a security value',
    fix: 'use crypto.randomUUID() or crypto.randomBytes()',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bdebugger\s*;|\bconsole\.(?:log|debug|dir|trace)\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
  {
    id: 'obvious/nested-ternary', rung: 'OBVIOUS', onCode: true,
    re: /\?[^?:\n]*\?[^:\n]*:[^:\n]*:/,
    message: 'nested ternary',
    fix: 'rewrite as if/else or a lookup',
  },
];

// try { ... } catch (e) { }  — with only whitespace or a comment in the block.
const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;

const FUNCTION_SIGNATURE =
  /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?\w+\s*)\(([^)]*)\)\s*(?::[^{=]+)?(?:=>)?\s*\{/g;

function lineRuleFindings(lines, codeLines) {
  const findings = [];
  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      const subject = rule.onCode ? codeLines[index] : line;
      if (!rule.re.test(subject)) continue;
      findings.push(finding({
        rung: rule.rung, id: rule.id, line: index + 1,
        message: rule.message, fix: rule.fix,
      }));
    }
  });
  return findings;
}

function swallowedFindings(stripped) {
  return Array.from(stripped.matchAll(SWALLOWED), (match) => finding({
    rung: 'TRUE', id: 'true/swallowed-error',
    line: stripped.slice(0, match.index).split('\n').length,
    message: 'error swallowed by an empty catch',
    fix: 'log with context and rethrow, or handle it explicitly',
  }));
}

// Attaches params and complexity to the brace blocks that start on a signature line.
function measureFunctions(lines, stripped, blocks) {
  const signatures = new Map();
  for (const match of stripped.matchAll(FUNCTION_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  return blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));
}

function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  return [
    ...lineRuleFindings(lines, stripped.split(/\r?\n/)),
    ...swallowedFindings(stripped),
    ...shapeFindings({
      blocks: measureFunctions(lines, stripped, blocks),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'function',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
