// tests/perf-guard.js
//
// The linearity guard the six language-pack tests share.
//
// Every rule in a pack must stay linear in line length. Each adversarial unit
// a test lists is a prefix that matches: repeated, it makes any unbounded span
// re-scan to end of line from every offset, which is the quadratic shape that
// took .NET's safe/shell-injection 4.7s on this input, Python's
// true/mutable-default 653ms, and Go's taint assignment pattern 4,887ms.
//
// The bound is relative, not an absolute millisecond count. An absolute one
// measures the machine as much as the code: a 1s bound was tripped at
// 1958-2230ms by a rule that was merely slow, because a loaded runner scales
// every number up at once. So the same 100KB of a benign unit is timed first,
// which is what "linear on this line length, on this machine, right now"
// costs; load moves that baseline and the bound with it. Adversarial units run
// within a handful of times it today and quadratic ran at 146x, so 40x sits an
// order of magnitude clear on both sides. The 5ms floor is for a fast machine
// where the baseline rounds toward zero — it still leaves 200ms, well under
// the 586ms the quadratic rule took.
//
// Each timing is the fastest of three runs: a scheduler stall lands in one run
// of three, not all three.
//
// It lives here rather than being copied into each pack's test because it was
// copied into each pack's test, and the copies had drifted to four different
// bounds for one property.

const SIZE = 100 * 1024;
const PERF_MULTIPLE = 40;
const PERF_FLOOR_MS = 5;
const RUNS = 3;

function bestOf(runs, work) {
  let best = Infinity;
  for (let i = 0; i < runs; i += 1) {
    const started = Date.now();
    work();
    best = Math.min(best, Date.now() - started);
  }
  return best;
}

function lineOf(unit) {
  return unit.repeat(Math.ceil(SIZE / unit.length)).slice(0, SIZE);
}

// `spec` is { assert, check, relPath, config, baseline, units }. `baseline` is
// a benign unit — linear by construction — and `units` are the adversarial
// ones. Sources given whole (the word runs ts guards against) are passed as
// `sources` instead and used as-is rather than repeated to SIZE.
function assertLinear(spec) {
  const timeOf = (text) => bestOf(RUNS, () => spec.check(text, {
    relPath: spec.relPath, config: spec.config,
  }));

  const budget = Math.max(timeOf(lineOf(spec.baseline)), PERF_FLOOR_MS) * PERF_MULTIPLE;
  for (const unit of spec.units || []) {
    const elapsed = timeOf(lineOf(unit));
    spec.assert.ok(elapsed < budget,
      `100KB line took ${elapsed}ms (budget ${budget}ms) for unit ${JSON.stringify(unit)}`);
  }
  for (const source of spec.sources || []) {
    const elapsed = timeOf(source);
    spec.assert.ok(elapsed < budget,
      `a ${source.length}-byte source took ${elapsed}ms (budget ${budget}ms)`);
  }
}

module.exports = { SIZE, assertLinear, bestOf, lineOf };
