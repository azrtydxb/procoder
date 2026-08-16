#!/usr/bin/env node
// procoder — TypeScript / JavaScript pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:query|execute|raw|exec)\s*\(\s*(?:`[^`]*\$\{|["'][^"']*["']\s*\+)/i,
    message: 'SQL built by interpolation or concatenation',
    fix: 'use a parameterized query with bound values',
  },
  {
    id: 'safe/xss-sink', rung: 'SAFE',
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
    id: 'obvious/nested-ternary', rung: 'OBVIOUS',
    re: /\?[^?:\n]*\?[^:\n]*:[^:\n]*:/,
    message: 'nested ternary',
    fix: 'rewrite as if/else or a lookup',
  },
];

// try { ... } catch (e) { }  — with only whitespace or a comment in the block.
const SWALLOWED = /catch\s*\([^)]*\)\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;

const FUNCTION_SIGNATURE =
  /(?:function\s+\w*|(?:const|let|var)\s+\w+\s*=\s*(?:async\s*)?|(?:async\s+)?\w+\s*)\(([^)]*)\)\s*(?::[^{=]+)?(?:=>)?\s*\{/g;

function check(source, { relPath, config } = {}) {
  const findings = [];
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const stripped = stripNoise(text);

  lines.forEach((line, index) => {
    for (const rule of LINE_RULES) {
      if (rule.re.test(line)) {
        findings.push(finding({
          rung: rule.rung, id: rule.id, line: index + 1,
          message: rule.message, fix: rule.fix,
        }));
      }
    }
  });

  for (const match of stripped.matchAll(SWALLOWED)) {
    findings.push(finding({
      rung: 'TRUE', id: 'true/swallowed-error',
      line: stripped.slice(0, match.index).split('\n').length,
      message: 'error swallowed by an empty catch',
      fix: 'log with context and rethrow, or handle it explicitly',
    }));
  }

  // Attach params and complexity to the brace blocks that start on a signature line.
  const { maxDepth, blocks } = analyzeBraces(text);
  const signatures = new Map();
  for (const match of stripped.matchAll(FUNCTION_SIGNATURE)) {
    signatures.set(stripped.slice(0, match.index).split('\n').length, match[1]);
  }

  const measured = blocks.map((block) => {
    const signature = signatures.get(block.startLine);
    const body = lines.slice(block.startLine - 1, block.endLine).join('\n');
    return {
      ...block,
      params: signature === undefined ? 0 : countParams('(' + signature + ')'),
      complexity: signature === undefined ? 0 : estimateComplexity(body),
    };
  }).filter((block) => signatures.has(block.startLine));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'function',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
