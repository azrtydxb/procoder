#!/usr/bin/env node
// procoder — C# / .NET pack.

const { stripComments } = require('./comments');
const { spanRuleFindings } = require('./spans');
const { CONCAT, skipConstant, taintFindings } = require('./taint');
const {
  analyzeBraces, emptyCatchFindings, lineRuleFindings, measureFunctions,
  shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

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
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /DangerousAcceptAnyServerCertificateValidator/,
    message: 'TLS certificate validation disabled',
    fix: 'validate against the proper CA instead',
  },
  {
    id: 'safe/shell-injection', rung: 'SAFE', dataSink: true,
    // The Process.Start half moved to SPAN_RULES; its `[^)]{0,500}` is where
    // the 500-character ceiling was, and it had no delimiter to terminate it.
    //
    // This half is a conjunction — UseShellExecute = true *and* an
    // interpolated Arguments, in either order on the line. Written as a bare
    // `(?=.*A)(?=.*B)` pair it is anchorless, so the engine retries both scans
    // from every one of the line's n offsets: 4.9s on a 100KB line, 75s at
    // 400KB. Anchoring at `^` (with /m so it still applies per line if this is
    // ever run over whole text) leaves one start per line and makes it linear;
    // the conjunction itself is unchanged, and it has no ceiling.
    re: /^(?=.*UseShellExecute\s*=\s*true).*Arguments\s*=\s*\$"/m,
    message: 'shell invoked with an interpolated command',
    fix: 'use ArgumentList with UseShellExecute = false instead of a shell string',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /Console\.(?:WriteLine|Write)\s*\(|Debug\.WriteLine\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or route through ILogger',
  },
];

// Rules whose needle may sit anywhere inside the same call or the same
// statement — see spans.js. All three carried a 500-character ceiling: a
// Process.Start whose interpolated argument sat further than that from the
// call, a token assignment with a long expression before `new Random(`, and a
// validation callback wrapped in a long expression were all unreported.
const SPAN_RULES = [
  {
    id: 'safe/shell-injection', rung: 'SAFE', within: 'call', dataSink: true,
    anchor: /Process\.Start\s*\(/g,
    needles: [/\$"|"[^"\n]*"\s*\+|\+\s*"[^"\n]*"/g],
    message: 'shell invoked with an interpolated command',
    fix: 'use ArgumentList with UseShellExecute = false instead of a shell string',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE', within: 'statement',
    anchor: /\b(?:token|secret|key|nonce|salt|session)\w*\s*=/gi,
    needles: [/new\s+Random\s*\(/g],
    message: 'System.Random used for a security value',
    fix: 'use RandomNumberGenerator.GetBytes',
  },
  {
    id: 'safe/tls-disabled', rung: 'SAFE', within: 'statement',
    anchor: /ServerCertificateValidationCallback\s*\+?=/g,
    needles: [/=>\s*true/g],
    message: 'TLS certificate validation disabled',
    fix: 'validate against the proper CA instead',
  },
];

// Local taint (taint.js): the assign-then-use form of the two rules above.
// `var q = "SELECT ... id=" + id;` then `new SqlCommand(q, conn)` is the shape
// the line rule cannot see.
//
// FromSqlInterpolated stays out of the sink list for the same reason it stays
// out of the line rule: it is EF Core's *safe* API, and it takes the
// interpolated string by design. The sink accepts either `(` or `=` after the
// verb, so `cmd.CommandText = q;` is read as well as `new SqlCommand(q, c)`;
// the argument must end the statement or the argument list, so a tainted name
// that is only the receiver of a further call is not counted.
const CS_NAME = String.raw`([A-Za-z_@]\w*)`;

const TAINT = {
  assign: /^\s*(?:(?:readonly|const|public|private|protected|internal|static|var)\s+)*(?:[\w<>[\],.?]+\s+)?([A-Za-z_@]\w*)\s*\+?=(?![=>])/,
  sources: [/\$"[^"\n]*\{/, /\b[Ss]tring\.Format\s*\(/, ...CONCAT],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: new RegExp(String.raw`(?:SqlCommand|CommandText|ExecuteSqlRaw|FromSqlRaw)\s*(?:\(|=)\s*${CS_NAME}\s*(?:[,)]|$)`),
      message: 'SQL built by interpolation or concatenation reaches a command',
      fix: 'use parameters (cmd.Parameters.AddWithValue) or FromSqlInterpolated',
    },
    {
      id: 'safe/shell-injection',
      re: new RegExp(String.raw`\bProcess\.Start\s*\(\s*${CS_NAME}\s*[,)]`),
      message: 'shell command built by interpolation or concatenation',
      fix: 'use ArgumentList with UseShellExecute = false instead of a shell string',
    },
  ],
};

const SWALLOWED = /catch\s*(?:\([^)]*\))?\s*\{\s*(?:\/\/[^\n]*\s*|\/\*[\s\S]*?\*\/\s*)*\}/g;
// Anchored to a single line, with `\s` allowed only inside a generic
// argument list so `Dictionary<string, int>` is measured — see jvm.js for
// why the class itself stays whitespace-free: letting the type span newlines
// turns "no method here" into a catastrophic-backtracking search over every
// blank line and brace in the class.
// Split at the parameter list's `(` — see jvm.js for why the count comes from
// shape.js's paramSpans pass rather than from a captured span.
const METHOD_SIGNATURE_LINE = {
  head: /^\s*(?:(?:public|private|protected|internal|static|async|override|virtual|sealed|abstract|readonly)\s+)*[\w<>[\],.?]+(?:\s*<[\w\s<>[\],.?]*>)?\s+\w+\s*\(/,
  tail: /\s*\{\s*$/,
};

// Every rule here matches code, never prose: a comment naming
// `FromSqlRaw($"...")` documents the hole, it does not open one. String
// literals stay — the `$"..."` handed to the sink *is* the SQL, and blanking
// it would blind the rule. See comments.js for the shared principle.
function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const code = stripComments(text, 'c');
  const lines = code.split(/\r?\n/);
  const stripped = stripNoise(text);
  const { maxDepth, blocks } = analyzeBraces(text);

  const inline = lineRuleFindings(LINE_RULES, lines, { skip: skipConstant });

  return [
    ...inline,
    ...spanRuleFindings(SPAN_RULES, lines, { existing: inline }),
    ...taintFindings({
      lines, stripped: stripped.split(/\r?\n/), spec: TAINT, existing: inline,
    }),
    ...emptyCatchFindings(code, SWALLOWED, 'exception swallowed by an empty catch'),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, METHOD_SIGNATURE_LINE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'method',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
