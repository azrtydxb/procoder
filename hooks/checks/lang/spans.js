#!/usr/bin/env node
// procoder — sink rules that span an argument list or a statement, with no
// length ceiling on either.
//
// The rules these replace all had the same shape: an anchor (`Process.Start(`,
// `Runtime.getRuntime().exec(`, `var token =`) followed, somewhere in the same
// call or the same statement, by a needle (`$"`, a `+`, `new Random(`). Each
// wrote that "somewhere" as a *captured span* — `[^)]{0,500}`, `[^;]{0,500}` —
// and the bound was load-bearing: an unbounded `[^)]*` is retried from every
// anchor on the line and, with no `)` ahead, runs to end of line each time.
// That is quadratic, and it took 4.7s on a 100KB line and 75s at 400KB.
//
// The bound bought that back by giving up the finding whenever the needle sat
// more than 500 characters from the anchor — which is precisely the code most
// worth reporting: a long argument list is where a stray concatenation hides,
// and a minified or generated line has no short spans at all.
//
// shape.js already solved this problem once, for parameter counting: one
// left-to-right paramSpans pass records where every `(` closes, so nothing has
// to capture a span. The same pass answers "where does this call end" here,
// and a statement's end is the next `;`. What was missing was the other half —
// "is the needle inside that window" — and that needs no span either: the
// needle's offsets on the line are collected once, in one pass, and each
// anchor asks the sorted list by binary search. One paramSpans pass plus one
// scan per needle per line, then O(log n) per anchor. Linear in the line, with
// no ceiling anywhere.

const { finding } = require('../finding');
const { skipConstant } = require('./taint');
const { paramSpans } = require('../shape');

const SEMICOLON = /;/g;

// Every match offset of a global pattern, in increasing order.
function offsetsOf(text, re) {
  const found = [];
  re.lastIndex = 0;
  for (let m = re.exec(text); m; m = re.exec(text)) {
    found.push(m.index);
    if (m[0] === '') re.lastIndex += 1;
  }
  return found;
}

// Position in `sorted` of the first offset at or after `from`.
function firstFrom(sorted, from) {
  let lo = 0;
  let hi = sorted.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (sorted[mid] < from) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

function anyIn(sorted, from, to) {
  const at = firstFrom(sorted, from);
  return at < sorted.length && sorted[at] < to;
}

// The per-line work, computed at most once each and only when a rule asks.
function lineCache(line) {
  const needles = new Map();
  let spans = null;
  let semis = null;
  return {
    spans: () => (spans || (spans = paramSpans(line))),
    semis: () => (semis || (semis = offsetsOf(line, SEMICOLON))),
    needle: (re) => {
      if (!needles.has(re)) needles.set(re, offsetsOf(line, re));
      return needles.get(re);
    },
  };
}

// Where the window opened by `match` ends: the `)` that closes the call whose
// `(` the anchor ends on, or the next `;`, or end of line. -1 when a `call`
// rule's parenthesis never closes on this line, which is no window at all.
function windowEnd(rule, match, line, cache) {
  if (rule.within === 'statement') {
    const semis = cache.semis();
    const at = firstFrom(semis, match.index);
    return at < semis.length ? semis[at] : line.length;
  }
  const span = cache.spans().get(match.index + match[0].length - 1);
  return span ? span.end : -1;
}

// An optional test applied immediately after the window closes — `after` is
// sticky, so it can only match right there. It is how "an empty block follows
// this parameter list" is expressed without capturing the list.
function tailMatches(rule, line, end) {
  if (!rule.after) return true;
  rule.after.lastIndex = end + 1;
  return rule.after.test(line);
}

function ruleHits(rule, line, cache) {
  rule.anchor.lastIndex = 0;
  for (let m = rule.anchor.exec(line); m; m = rule.anchor.exec(line)) {
    const end = windowEnd(rule, m, line, cache);
    const from = m.index + m[0].length;
    if (end !== -1
      && rule.needles.every((re) => anyIn(cache.needle(re), from, end))
      && tailMatches(rule, line, end)) return true;
  }
  return false;
}

// Applies a pack's span-rule table. `existing` is the pack's line-rule
// findings: where a line rule already reported the same id on the same line —
// the two halves of .NET's shell rule, say — this adds nothing.
//
// A rule marked `dataSink` is discharged on a wholly constant line, the same
// test and for the same reason as the line rules — see taint.js.
// `Command::new("sh").arg("-c").arg("ls /tmp")` runs a literal command.
function spanRuleFindings(rules, lines, { existing = [] } = {}) {
  const already = new Set(existing.map((f) => `${f.id}:${f.line}`));
  const findings = [];

  lines.forEach((line, index) => {
    const cache = lineCache(line);
    for (const rule of rules) {
      const key = `${rule.id}:${index + 1}`;
      if (already.has(key) || !ruleHits(rule, line, cache) || skipConstant(rule, line)) continue;
      already.add(key);
      findings.push(finding({
        rung: rule.rung, id: rule.id, line: index + 1, message: rule.message, fix: rule.fix,
      }));
    }
  });
  return findings;
}

module.exports = { spanRuleFindings };
