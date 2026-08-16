#!/usr/bin/env node
// procoder — Python pack.

const { finding } = require('../finding');
const { stripComments } = require('./comments');
const {
  analyzeIndent, countParams, estimateComplexity, lineRuleFindings, shapeFindings,
} = require('../shape');

const EXTENSIONS = ['.py'];

const LINE_RULES = [
  {
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /\b(?:execute|executemany|raw|text)\s*\(\s*(?:f["']|["'][^"']*["']\s*%|["'][^"']*["']\s*\+|["'][^"']*["']\s*\.format\s*\()/i,
    message: 'SQL built by f-string, % or concatenation',
    fix: 'pass parameters as the second argument instead',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE',
    re: /shell\s*=\s*True|\bos\.system\s*\(|\bos\.popen\s*\(/,
    message: 'shell execution with an interpolated command',
    fix: 'pass an argument list and leave shell=False',
  },
  {
    id: 'safe/dynamic-eval', rung: 'SAFE',
    re: /\beval\s*\(|\bexec\s*\(|\b__import__\s*\(/,
    message: 'dynamic code evaluation',
    fix: 'replace with a dict lookup or a direct call',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /\bpickle\.loads?\s*\(|\bmarshal\.loads?\s*\(|\byaml\.load\s*\((?![^)]*Safe)/,
    message: 'unsafe deserialization of untrusted bytes',
    fix: 'use json, or yaml.safe_load',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\bhashlib\.(?:md5|sha1)\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use hashlib.sha256, or argon2/bcrypt for passwords',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /verify\s*=\s*False|ssl\._create_unverified_context/,
    message: 'TLS certificate verification disabled',
    fix: 'point at the proper CA bundle instead',
  },
  {
    id: 'true/mutable-default', rung: 'TRUE',
    re: /\bdef\s+\w+\s*\([^)]*=\s*(?:\[\s*\]|\{\s*\}|set\s*\(\s*\))/,
    message: 'mutable default argument — shared across calls',
    fix: 'default to None and build the value inside the function',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /^\s*print\s*\(|\bbreakpoint\s*\(\s*\)|\bpdb\.set_trace\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through the project logger',
  },
];

const BARE_EXCEPT = /^\s*except\s*:\s*$/;
const BROAD_EXCEPT = /^\s*except\s+(?:Exception|BaseException)\b[^:]*:\s*$/;
const SILENT_BODY = /^\s*(?:pass|\.\.\.)\s*$/;
const DEF_LINE = /^\s*(?:async\s+)?def\s+\w+\s*\(([^)]*)\)/;

function exceptFindings(lines) {
  const findings = [];
  lines.forEach((line, index) => {
    if (BARE_EXCEPT.test(line)) {
      findings.push(finding({
        rung: 'TRUE', id: 'true/bare-except', line: index + 1,
        message: 'bare except catches SystemExit and KeyboardInterrupt too',
        fix: 'catch the specific exception you can actually handle',
      }));
      return;
    }
    if (BROAD_EXCEPT.test(line) && SILENT_BODY.test(lines[index + 1] || '')) {
      findings.push(finding({
        rung: 'TRUE', id: 'true/swallowed-error', line: index + 1,
        message: 'exception silently discarded',
        fix: 'log with context and re-raise, or handle it explicitly',
      }));
    }
  });
  return findings;
}

// Python has no signature-opening brace to key on, so every indent block is
// measured and the def line, when there is one, supplies the parameters.
function measureBlocks(lines, blocks) {
  return blocks.map((block) => {
    const signature = DEF_LINE.exec(lines[block.startLine - 1] || '');
    return {
      ...block,
      params: signature ? countParams('(' + signature[1] + ')') : 0,
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    };
  });
}

// Every rule here matches code, never prose: `# never use eval(user_input)`
// documents the hole, it does not open one. String literals stay — an f-string
// or a `%` template *is* the SQL, and blanking it would blind the rule that
// looks for it. See comments.js for the principle the six packs share.
function check(source, { relPath, config } = {}) {
  const lines = stripComments(source, 'py').split(/\r?\n/);
  const { maxDepth, blocks } = analyzeIndent(source, { tabWidth: 4 });

  return [
    ...lineRuleFindings(LINE_RULES, lines),
    ...exceptFindings(lines),
    ...shapeFindings({
      blocks: measureBlocks(lines, blocks),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'function',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
