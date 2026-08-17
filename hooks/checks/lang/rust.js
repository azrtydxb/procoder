#!/usr/bin/env node
// procoder — Rust pack.

const { finding } = require('../finding');
const { blankStringInteriors, stripComments } = require('./comments');
const { spanRuleFindings } = require('./spans');
const {
  CONCAT, packContext, skipConstant, taintFindings, valuePattern,
} = require('./taint');
const {
  analyzeBraces, lineRuleFindings, measureFunctions, shapeFindings, signaturesFrom, stripNoise,
} = require('../shape');

const EXTENSIONS = ['.rs'];

const LINE_RULES = [
  {
    id: 'true/unwrap-in-library', rung: 'TRUE',
    re: /\.\s*(?:unwrap|expect)\s*\(/,
    message: 'unwrap/expect panics on the caller',
    fix: 'propagate with ? and a typed error',
  },
  {
    id: 'safe/sql-injection', rung: 'SAFE', dataSink: true,
    re: /\bquery\w*\s*\(\s*&?format!|\bexecute\s*\(\s*&?format!/,
    message: 'SQL built with format!',
    fix: 'use bind parameters',
  },
  {
    // safe/shell-injection and safe/weak-random are in SPAN_RULES: both
    // reached across the rest of the statement with a `[^;]`/`[^=]` span, and
    // both had to bound it at 500 characters to stay linear.
    id: 'safe/tls-disabled', rung: 'SAFE',
    re: /danger_accept_invalid_certs\s*\(\s*true\s*\)|danger_accept_invalid_hostnames\s*\(\s*true\s*\)/,
    message: 'TLS certificate verification disabled',
    fix: 'add the proper root certificate instead',
  },
  {
    // reqwest ships no default timeout: `Client::new()` and the one-shot
    // helpers wait for as long as the far end likes.
    //
    // Fully qualified only, and only the two shapes that cannot carry a
    // timeout by construction. A `Client::builder()` chain is left alone —
    // its `.timeout(...)` is on a line of its own and this rung blocks — and an
    // unqualified `Client::new()` may be any crate's client.
    id: 'true/missing-timeout', rung: 'TRUE',
    re: /\breqwest::(?:blocking::)?(?:get\s*\(|Client::new\s*\(\s*\))/,
    message: 'reqwest client with no timeout — reqwest sets none by default',
    fix: 'build it with Client::builder().timeout(Duration::from_secs(..))',
  },
  {
    id: 'alone/debug-leftover', rung: 'ALONE',
    re: /\bdbg!\s*\(|\bprintln!\s*\(|\beprintln!\s*\(/,
    message: 'leftover debugging statement',
    fix: 'delete it, or use the tracing/log crate',
  },
];

// Rules whose needle may sit anywhere later in the same statement — see
// spans.js. A builder chain is exactly where the 500-character ceiling bit:
// `Command::new("sh")` followed by a long `.env(...).current_dir(...)` run
// before `.arg("-c")` went unreported, as did a binding whose type annotation
// ran past 500 characters before the `= rand::random()`.
const SPAN_RULES = [
  {
    id: 'safe/shell-injection', rung: 'SAFE', within: 'statement', dataSink: true,
    anchor: /Command::new\s*\(\s*"(?:sh|bash|cmd|powershell)"\s*\)/g,
    needles: [/\.arg\s*\(\s*"-c"/g],
    message: 'shell invoked with an interpolated command',
    fix: 'call the binary directly with separate args',
  },
  {
    id: 'safe/weak-random', rung: 'SAFE', within: 'statement',
    // The anchor stops at the name: the needle begins with the `=`, and the
    // window opens where the anchor's match ends, so consuming the `=` here
    // would put it behind the window. What sat between the two was the type
    // annotation whose length was the ceiling.
    anchor: /\b(?:token|secret|key|nonce|salt|session)\w*/gi,
    needles: [/=\s*rand::(?:random|thread_rng)\b/g],
    message: 'general-purpose RNG used for a security value',
    fix: 'use a CSPRNG (rand::rngs::OsRng or the ring crate)',
  },
];

// Local taint (taint.js): the bind-then-use form of the SQL rule above.
// `let q = format!("SELECT ... {}", id);` then `sqlx::query(&q)` is the shape
// the line rule cannot see.
//
// The binding pattern takes an optional `mut` and an optional type
// annotation, and also matches a bare reassignment, so rebinding the name to a
// literal clears it. The sink allows a leading `&`, which is how a `String`
// reaches sqlx. Shell gets no taint sink: `Command::new("sh").arg("-c")` is
// already reported on the invocation itself, whatever the argument is.
const RS_WORD = String.raw`[A-Za-z_]\w*`;

const TAINT = {
  // `const` and `static` alongside `let`: without them `const TABLE: &str =
  // "users"` matched nothing at all, so a table name declared the way Rust
  // declares one was neither tracked nor provably constant.
  assign: new RegExp(String.raw`^\s*(?:(?:let|const|static)\s+(?:mut\s+)?)?(${RS_WORD}(?:\.${RS_WORD})*)\s*(?::[^=\n]*)?\+?=(?!=)`),
  declare: /^\s*(?:let|const|static)\s/,
  func: new RegExp(String.raw`^\s*(?:pub(?:\s*\([^()]*\))?\s+)?(?:default\s+)?(?:const\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(${RS_WORD})`),
  sources: [/\bformat!\s*\(/, ...CONCAT],
  sinks: [
    {
      id: 'safe/sql-injection',
      re: new RegExp(String.raw`\b(?:query|execute)\w*\s*\(\s*&?\s*${valuePattern(RS_WORD)}\s*[,)]`),
      message: 'SQL built with format! or concatenation reaches a query',
      fix: 'use bind parameters',
    },
  ],
};

// `unsafe {` at the start of a line or preceded by whitespace — which is what
// `^\s*(?:.*\s)?unsafe\s*\{` said, at a cost that was quadratic in line
// length: `^\s*` may end anywhere in a leading whitespace run and `.*\s` then
// re-scans the rest of the line for each of those ends. comments.js blanks a
// comment to a run of spaces, so a `/* … */` license header on one line is
// exactly that input: a terminated 64KB header cost 1,935ms and 512KB cost
// 124,363ms. Two alternatives with nothing to backtrack over match the same
// lines: `}unsafe {` was excluded before, for want of the whitespace, and
// still is.
const UNSAFE_BLOCK = /(?:^|\s)unsafe\s*\{/;
const SAFETY_COMMENT = /\/\/\s*SAFETY:|\/\/!\s*SAFETY:/i;
// Head up to the parameter list's `(`, tail from where that list closes to the
// block-opening `{`; shape.js counts the parameters in between without
// capturing them, so the list has no length ceiling. See go.js.
const FN_SIGNATURE = {
  head: /fn\s+\w+\s*(?:<[^>]{0,500}>)?\s*\(/g,
  tail: /[^{\n]{0,500}\{/,
};
const TEST_ATTR = /#\[cfg\(test\)\]|#\[test\]|#\[tokio::test\]/;
// unwrap()/expect() directly on a Mutex/RwLock lock() is near-universal Rust
// practice: a poisoned lock means a prior panic already corrupted shared
// state, and there is no sensible recovery short of aborting — so this is
// deliberately not flagged, unlike unwrap on ordinary fallible calls.
const LOCK_UNWRAP = /\.\s*(?:lock|read|write)\s*\(\s*\)\s*\.\s*(?:unwrap|expect)\s*\(/;

// unwrap in a #[cfg(test)] module or a #[test] fn is normal, not a finding.
// Scoped by brace matching to just that block, not "everything after this
// attribute to end of file" — a #[cfg(test)] module in the middle of a file
// must not blind the checker to library code that follows it.
// The line that opens the attributed item's block: the attribute line itself
// for `#[test] fn t() {`, or a following one for stacked attributes and
// multi-line signatures.
function blockOpenIndex(lines, from) {
  for (let j = from; j < lines.length; j += 1) {
    if (lines[j].includes('{')) return j;
  }
  return -1;
}

// The line holding the `}` that balances the block opened on `openIndex`.
function blockCloseIndex(lines, openIndex) {
  let depth = 0;
  for (let j = openIndex; j < lines.length; j += 1) {
    for (const ch of lines[j]) {
      if (ch === '{') depth += 1;
      else if (ch === '}') depth -= 1;
    }
    if (depth === 0) return j;
  }
  return lines.length - 1;
}

// Braces are counted on noise-stripped lines, as shape.js does: a `{` inside a
// string or a comment is not structure, and counting one leaves the depth
// unbalanced and runs the region to end of file. The attributes are matched on
// comment-stripped lines — `#` is not a comment in Rust, so `#[cfg(test)]`
// survives — which keeps a commented-out `// #[test]` from opening a region
// and blinding the checker to the library code that follows.
function testRegions(attrLines, codeLines) {
  const regions = [];
  attrLines.forEach((line, index) => {
    if (!TEST_ATTR.test(line)) return;
    const openIndex = blockOpenIndex(codeLines, index);
    if (openIndex === -1) return;
    regions.push([index + 1, blockCloseIndex(codeLines, openIndex) + 1]);
  });
  return regions;
}

function inRegions(regions, lineNo) {
  return regions.some(([start, end]) => lineNo >= start && lineNo <= end);
}

const TEST_PATH = /(?:^|\/)tests\/|_test\.rs$/;
const SAFETY_LOOKBACK = 3;

// How many lines back a // SAFETY: comment may sit and still count as the
// justification for this unsafe block.
//
// The block itself is found on comment-stripped lines — `// unsafe { ... }` in
// prose is not an unsafe block — while the justification is looked for in the
// raw ones, since a comment is exactly what this rule asks for.
function unsafeFindings(codeLines, lines) {
  const findings = [];
  codeLines.forEach((line, index) => {
    if (!UNSAFE_BLOCK.test(line)) return;
    const preceding = lines.slice(Math.max(0, index - SAFETY_LOOKBACK), index).join('\n');
    if (SAFETY_COMMENT.test(preceding)) return;
    findings.push(finding({
      rung: 'SAFE', id: 'safe/unsafe-block', line: index + 1,
      message: 'unsafe block with no SAFETY comment',
      fix: 'add // SAFETY: stating the invariants that make this sound',
    }));
  });
  return findings;
}

// Every rule here matches code, never prose: `// never parse(x).unwrap()` and
// a doc comment showing the SQL not to build are documentation, not defects.
// String literals stay — `format!("SELECT {}", id)` *is* the SQL. The one
// comment a rule genuinely asks about, `// SAFETY:`, still reads raw lines.
// See comments.js for the principle the six packs share.
function check(source, { relPath, config } = {}) {
  const text = String(source || '');
  const lines = text.split(/\r?\n/);
  const codeLines = stripComments(text, 'c').split(/\r?\n/);
  // The structure input, and only it: a multi-line string's interior is data,
  // and the taint scan reads statements off this text. The rule input above
  // keeps every literal whole — see comments.js.
  //
  // stripNoise runs first, never after. Blanking takes the closing delimiter
  // with it, and stripNoise reading the blanked text would then find an opener
  // with no closer and eat forward to the next quote in the file.
  const stripped = blankStringInteriors(stripNoise(text), 'c');
  const { maxDepth, blocks } = analyzeBraces(text);

  const tests = testRegions(codeLines, stripped.split(/\r?\n/));
  const isTestFile = TEST_PATH.test(String(relPath || ''));
  const expectedPanic = (rule, line, lineNo) => rule.id === 'true/unwrap-in-library'
    && (isTestFile || inRegions(tests, lineNo) || LOCK_UNWRAP.test(line));

  const ctx = packContext({ lines: codeLines, stripped: stripped.split(/\r?\n/), spec: TAINT });
  const inline = lineRuleFindings(LINE_RULES, codeLines, {
    skip: (rule, line, lineNo) => expectedPanic(rule, line, lineNo)
      || skipConstant(rule, line, ctx),
  });

  return [
    ...inline,
    ...spanRuleFindings(SPAN_RULES, codeLines, { existing: inline, ctx }),
    ...taintFindings({ spec: TAINT, ctx, existing: inline }),
    ...unsafeFindings(codeLines, lines),
    ...shapeFindings({
      blocks: measureFunctions(lines, blocks, signaturesFrom(stripped, FN_SIGNATURE)),
      maxDepth,
      thresholds: config.thresholds,
      kind: 'fn',
    }),
  ];
}

module.exports = { check, EXTENSIONS };
