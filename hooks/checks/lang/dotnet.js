#!/usr/bin/env node
// procoder — C# / .NET pack.

const { finding } = require('../finding');
const { analyzeBraces, countParams, estimateComplexity, shapeFindings, stripNoise } = require('../shape');

const EXTENSIONS = ['.cs'];

const LINE_RULES = [
  {
    // Deliberately keys on SqlCommand/CommandText/ExecuteSqlRaw/FromSqlRaw
    // only — never FromSqlInterpolated, which is EF Core's *safe* API: it
    // takes a C# interpolated string by design and parameterizes it
    // internally, so treating it like string concatenation would be a
    // false positive on the exact pattern teams are told to migrate to.
    id: 'safe/sql-injection', rung: 'SAFE',
    re: /(?:SqlCommand|CommandText|ExecuteSqlRaw|FromSqlRaw)\s*(?:\(|=)\s*(?:\$"|"[^"]*"\s*\+|\w+\s*\+)/,
    message: 'SQL built by interpolation or concatenation',
    fix: 'use parameters (cmd.Parameters.AddWithValue) or FromSqlInterpolated',
  },
  {
    id: 'safe/unsafe-deserialize', rung: 'SAFE',
    re: /new\s+BinaryFormatter\s*\(|NetDataContractSerializer|LosFormatter|TypeNameHandling\s*=\s*TypeNameHandling\.(?:All|Objects|Auto)/,
    message: 'unsafe deserialization of untrusted input',
    fix: 'use System.Text.Json with an explicit contract',
  },
  {
    id: 'safe/weak-hash', rung: 'SAFE',
    re: /\b(?:MD5|SHA1)\.Create\s*\(|new\s+(?:MD5|SHA1)CryptoServiceProvider\s*\(/,
    message: 'weak hash used where a secure one is expected',
    fix: 'use SHA256, or PBKDF2/Argon2 for passwords',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE',
    re: /\b(?:token|secret|key|nonce|salt|session)\w*\s*=\s*[^;]*new\s+Random\s*\(/i,
    message: 'System.Random used for a security value',
    fix: 'use RandomNumberGenerator.GetBytes',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /ServerCertificateValidationCallback\s*(?:\+?=)\s*[^;]*=>\s*true|DangerousAcceptAnyServerCertificateValidator/,
    message: 'TLS certificate validation disabled',
    fix: 'validate against the proper CA instead',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /Console\.(?:WriteLine|Write)\s*\(|Debug\.WriteLine\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through ILogger',
  },
];

const SWALLOWED = /catch\s*(?:\([^)]*\))?\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;
// Anchored to a single line, and with no `\s` inside the return-type
// character class — see jvm.js for why: letting the type span newlines
// turns "no method here" into a catastrophic-backtracking search over every
// blank line and brace in the class.
const METHOD_SIGNATURE_LINE =
  /^\s*(?:(?:public|private|protected|internal|static|async|override|virtual|sealed|abstract|readonly)\s+)*[\w<>[\],.?]+\s+\w+\s*\(([^)]*)\)\s*\{\s*$/;

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

  for (const match of text.matchAll(SWALLOWED)) {
    findings.push(finding({
      rung: 'TRUE', id: 'true/swallowed-error',
      line: text.slice(0, match.index).split('\n').length,
      message: 'exception swallowed by an empty catch',
      fix: 'log with context and rethrow, or handle it explicitly',
    }));
  }

  const { maxDepth, blocks } = analyzeBraces(text);
  const strippedLines = stripped.split(/\r?\n/);
  const signatures = new Map();
  strippedLines.forEach((line, index) => {
    const match = METHOD_SIGNATURE_LINE.exec(line);
    if (match) signatures.set(index + 1, match[1]);
  });

  const measured = blocks
    .filter((block) => signatures.has(block.startLine))
    .map((block) => ({
      ...block,
      params: countParams('(' + signatures.get(block.startLine) + ')'),
      complexity: estimateComplexity(lines.slice(block.startLine - 1, block.endLine).join('\n')),
    }));

  findings.push(...shapeFindings({
    blocks: measured, maxDepth, thresholds: config.thresholds, kind: 'method',
  }));

  return findings;
}

module.exports = { check, EXTENSIONS };
