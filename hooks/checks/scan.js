#!/usr/bin/env node
// procoder — scanning many files, in parallel when there are enough of them.
//
// The engine is one synchronous CPU-bound pass per file and Node runs one core,
// so `procoder check .` on a large repository spends most of its wall clock
// waiting for a single thread. Splitting the file list across a few child
// processes is the whole idea; everything else here exists to make that
// invisible to the caller.
//
// Three properties, and the first two are why this is worth any code at all:
//
//   - The result is IDENTICAL to the sequential one, in the same order. Slices
//     are contiguous and reassembled in input order, so nothing about the
//     report depends on how many cores the machine has.
//   - A worker that dies costs nothing but time. Its slice is scanned in this
//     process instead, and the run continues. A parallel scan that lost a
//     slice would be a gate reporting on less than it claims.
//   - Below the threshold, no process is forked at all. Forking costs hundreds
//     of milliseconds before a single file is read, so a scan worth less than
//     that pays more for the workers than the work. The threshold is a measured
//     amount of WORK and not a file count — see PARALLEL_MIN_WORK_MS, and the
//     defect that comment exists to close.
//
// "IDENTICAL" was a claim, not a fact, and three things broke it. All three are
// closed here, and all three are closed the same way — by this file deciding,
// once, what a scan of one file means, and handing the SAME decision down both
// paths instead of letting the worker have opinions of its own:
//
//   - the time budget. A worker ran files at 600,000ms while this process ran
//     them at 2,000ms, so `true/budget-exhausted` — the finding whose whole job
//     is to say "I gave up before I finished this file" — fired sequentially
//     and effectively never in parallel. SCAN_BUDGET_MS now travels in the
//     payload and is passed on the sequential path too, so it is one number in
//     one place. See the note on the constant for why it is 2,000 and not the
//     other way round.
//   - the config. A worker re-read `.procoder.toml` from disk and rebuilt it,
//     so a caller-built config reached only half the files — and any warning
//     the parse emits was printed once per worker. The parent's config object
//     travels in the payload instead.
//   - a worker that HANGS. A dead worker was handled; a live one that never
//     writes was not, and hung the whole scan with no timeout anywhere. A
//     worker now has a bounded slice — its own budget times its own file count
//     — so exceeding it is not slowness, it is a wedge, and the parent kills it
//     and scans the slice itself, exactly as it does for a dead one.

const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const WORKER = path.join(__dirname, 'worker.js');

// --- when forking pays -----------------------------------------------------
//
// This used to be a file count — 250 — and a file count is the wrong unit,
// because forking pays for itself when the WORK is large and a file is not a
// unit of work. Measured on this machine (10 cores, jobs 8, best of three,
// through scanFiles so the fork is reachable at any size):
//
//   shape                      files   bytes   sequential  8 workers  ratio
//   one-line files             2,500    48KB      187ms      200ms     0.94
//   one-line files             4,000    78KB      288ms      263ms     1.10
//   third-party JS (npm tree)    125   538KB      252ms      434ms     0.58
//   third-party JS (npm tree)    175   2.1MB      849ms      760ms     1.12
//   16 x 60KB                     16   960KB      336ms      374ms     0.90
//   32 x 250KB                    32   8.0MB    2,516ms    1,135ms     2.22
//   one 900KB file                 1   900KB      291ms      419ms     0.69
//   Rust crate, [lints.clippy]     5   200 B      298ms      177ms     1.68
//   Rust crate, [lints.clippy]    34   1.4KB    1,663ms      763ms     2.18
//
// Three of those rows kill every predictor that can be computed from the file
// list alone. 250 one-liners fork SIX times slower than not forking (108ms
// against 18ms, and that is what shipped); 32 files of 250KB are 2.2x faster
// forked and were left sequential; and the clippy crate — five files, two
// hundred BYTES — is 1.7x faster forked, because each file costs a `cargo`
// spawnSync that neither a byte count nor a file count can see. Bytes are a
// better predictor than files and still wrong by 30x across those rows.
//
// What every row agrees on is the sequential MILLISECONDS. Each shape's own
// crossover lands between 230ms and 870ms of sequential work — a 3.8x band,
// against 23x for file count and 30x for bytes — so the predictor here is time,
// and it is measured rather than modelled: probeCost scans a few files and
// reports what they actually cost, which is the only estimator that sees a
// linter subprocess, a cold disk or a slow host at all.
//
// Derivation of the number. A forked run costs a startup floor F plus the work
// divided by the speedup S actually achieved; measured here F is 250–400ms at
// eight workers and S is 2.2–2.5 (not 8: each worker JITs the packs from cold,
// and slices are equal in file count rather than in bytes). So forking wins
// once the work still to do exceeds
//
//     F * S / (S - 1)   =   450ms .. 730ms
//
// and the highest crossover actually observed is 870ms — third-party JS at
// `--cpus 4`, where 175 files cost 871ms sequential and 875ms forked. 900 is
// that worst case rounded up, because the threshold has to be the WORST shape's
// crossover: being wrong above it costs a fraction of a scan, being wrong below
// it costs a multiple of one. Verified at jobs 3 and jobs 8, and in containers
// at --cpus 4 and at --cpus 1 — where defaultJobs is 1 and nothing forks at all,
// which is the case an earlier version got wrong by 2.3x.
//
// What this costs, said out loud: a shape whose own crossover is near the bottom
// of the band scans sequentially between there and 900ms. A clippy crate of 16
// files measures 897ms sequential against 318ms forked and just misses. The
// alternative is a threshold that forks a tree of one-liners six times slower
// than not forking, and that is the defect being fixed.
const PARALLEL_MIN_WORK_MS = 900;

// How the work is measured: scan a few files for real and see. Strided rather
// than the first few, because a repository whose first file is a 900KB bundle
// is not a repository of 900KB bundles, and the largest sample is dropped for
// the same reason — one outlier in a sample of four would otherwise multiply
// the estimate by thirty and fork a scan that had nothing left to do.
//
// The sample is not a rehearsal: those files are scanned once, by this process,
// with the same options a worker would get, and their results are the ones
// reported. The measurement is therefore free — it is work that had to happen
// anyway — and all it can cost is the parallelism of at most 16 files.
//
// 100ms of it, at most, so a tree of expensive files does not measure itself to
// death; four files at minimum, because a single sample is one file's JIT
// warm-up and nothing else.
const PROBE_MAX_FILES = 16;
const PROBE_MIN_FILES = 4;
const PROBE_MS = 100;

// The per-file budget for a scan, down BOTH paths, and the same 2,000ms
// `run.js` applies to a file it is given no budget for (BUDGET_MS there). Named
// here rather than imported so this module keeps the property its header
// claims — no engine require graph in the parent — and pinned to run.js's
// constant by a test, so the two cannot drift apart again the way they did.
//
// 2,000 and not the worker's old 600,000, because the finding is worth more
// than the coverage it costs. The argument for the large number was that a CLI
// slice is a batch job whose findings are the point, and that is true — but a
// budget only ever cuts the two stages that check the clock, the project's
// linter and the dependency-manifest rules, and both of them ANNOUNCE being
// cut. `true/budget-exhausted` is the announcement. A run that quietly waits
// ten minutes per file for a linter that will never answer reports nothing at
// all about the wait, and that is the failure mode this project has fixed
// nine times. A run that says "cargo clippy: no budget left for it" on the file
// it happened to is a gate telling the truth about its own coverage.
//
// The cost is stated rather than hidden: on a project whose linter cannot be
// batched (cargo clippy — see canBatch in resolve.js), a `procoder check .`
// that used to run clippy to completion per file in a worker now cuts it at
// its share of 2,000ms and says so. That is the sequential path's behaviour
// today, on every repository under 250 files, including this one.
const SCAN_BUDGET_MS = 2000;

// Never more than this many workers, however many are asked for. The scan is
// CPU-bound: past one child per core the extra processes only take turns, and
// each one costs a fork, an engine require graph and a heap. `--jobs 9999` on a
// 260-file scan measured 2.15s against 0.06s sequential — a 35x loss a user
// bought by typing a bigger number, which is why a number above the ceiling is
// refused and named rather than obeyed.
const MAX_JOBS = 8;

// How many cores this process may actually use — not how many the machine has.
// The difference is a container: `os.cpus()` reports the HOST's cores through a
// cgroup CPU quota, so a `docker run --cpus 1` node:24 image reports 10 and
// `os.availableParallelism()` reports 1. Measured, on 4,000 files:
//
//   --cpus 1   jobs=1 14.9s   jobs=4 35.0s   jobs=8 19.9s
//   --cpus 4   jobs=1 15.3s   jobs=4 14.6s   jobs=8 11.1s
//
// So the default computed from `os.cpus()` made a one-core container 2.3x
// SLOWER than not forking at all, which is the same shape of loss `--jobs 9999`
// buys on a laptop — and nobody typed anything to get it. `availableParallelism`
// has been in Node since 18.14 and this package requires 20.
//
// `os.cpus()` stays as the fallback, and is documented to be able to return an
// empty array, so the floor of 1 is not decoration.
function parallelism() {
  if (typeof os.availableParallelism === 'function') return os.availableParallelism();
  return (os.cpus() || []).length;
}

// One per usable core, minus one for this process, and never more than the
// ceiling: the scan is CPU-bound, so more workers than cores only adds
// scheduling, and the returns past eight are thin against the memory each
// child's heap costs.
function defaultJobs() {
  return Math.max(1, Math.min(MAX_JOBS, parallelism() - 1));
}

// `--jobs` from a user, made safe. The precedent is `max_file_bytes` in
// config.js and the semantics are deliberately the same: under the ceiling is
// honoured, over it is REFUSED with a warning naming both the value and the
// ceiling, and anything that is not a usable count is refused rather than
// coerced. Coercion is what this had before — `jobs || defaultJobs()` turned
// 0, NaN and -3 into the default in silence, so a typo in a CI invocation
// looked exactly like not passing the flag at all.
//
// Never throws: a bad flag must not take the gate down with it.
function clampJobs(requested) {
  if (requested === null || requested === undefined) return defaultJobs();
  const value = typeof requested === 'string' ? Number(requested) : requested;
  const fallback = defaultJobs();
  if (typeof value !== 'number' || !Number.isFinite(value) || Math.floor(value) < 1) {
    // `JSON.stringify(NaN)` is "null", which names the wrong thing back to a
    // user who typed `--jobs abc`. Anything unusable is shown as written.
    const shown = typeof requested === 'number' && Number.isNaN(requested)
      ? 'NaN' : JSON.stringify(requested);
    process.stderr.write(`procoder: --jobs ${shown} is not a number of worker processes — want a whole number from 1 to ${MAX_JOBS}; using ${fallback}\n`);
    return fallback;
  }
  if (value > MAX_JOBS) {
    process.stderr.write(`procoder: --jobs ${value} is above the ceiling ${MAX_JOBS} — using ${MAX_JOBS}. More workers than cores only take turns, and each one costs a process.\n`);
    return MAX_JOBS;
  }
  return Math.floor(value);
}

function sliceInto(files, count) {
  const size = Math.ceil(files.length / count);
  const slices = [];
  for (let i = 0; i < files.length; i += size) slices.push(files.slice(i, i + size));
  return slices;
}

// One slice, in a child. Asynchronous, and that is the entire point: spawnSync
// would run the workers one after another, which is the sequential scan with
// extra process startup — the first version of this file did exactly that.
//
// Resolves to null — never a partial or empty result — for every failure, so
// the caller can tell "this slice was not scanned" from "this slice was scanned
// and found nothing".
// A finished child's output, or null. Parsed here rather than inside the close
// handler so the promise body stays one level of nesting deep.
function sliceResult(code, out) {
  if (code !== 0 || !out) return null;
  try {
    return JSON.parse(out);
  } catch (e) {
    return null;
  }
}

// The budget for this scan: the caller's if it has one, the constant otherwise.
// Written out rather than `||` because 0 is a budget — a test uses it to prove
// both paths report the same exhaustion — and `||` would silently replace it.
function budgetOf(options) {
  return typeof options.budgetMs === 'number' ? options.budgetMs : SCAN_BUDGET_MS;
}

// What a worker gets before it has read a single file: its own process startup
// and the engine's require graph. Flat rather than scaled by the budget,
// because starting a process does not get faster when the budget is smaller —
// this is the one term that is a property of the host and not of the scan.
// Measured warm at ~200ms per child (three children, 606ms wall); 5s is the
// same number with room for a cold, loaded CI runner.
//
// Overshooting it costs nothing but time, and only on a host that deserved it:
// a worker killed early has its slice scanned in this process, correctly.
const WORKER_STARTUP_GRACE_MS = 5000;

// How long a slice may take before the parent stops believing in it.
//
// Derived, not picked: startup, plus two budgets per file. One budget is the
// file's own deadline; the second covers the one stage no deadline bounds — the
// language pack is a single synchronous call capped by file size rather than by
// clock, measured here at 674ms against the 2,000ms budget on the most
// adversarial 1MB file that could be built for it.
//
// Past this a worker is not slow, it is wedged — and a wedged worker used to
// hang the entire scan, because nothing here had a timeout of any kind. Now it
// is killed and its slice is scanned in this process, which is exactly what a
// worker that died already got.
function sliceTimeoutMs(count, budgetMs) {
  return WORKER_STARTUP_GRACE_MS + count * budgetMs * 2;
}

// A resolve that only counts the first time. A slice can end three ways —
// answered, crashed, killed by the watchdog — and two of them race: a killed
// child also emits 'close'. Resolving twice is harmless to a promise and
// invisible in a test, which is exactly why it is worth making impossible here.
function once(resolve) {
  let done = false;
  return (value) => {
    if (done) return;
    done = true;
    resolve(value);
  };
}

function runSlice(payload, timeoutMs) {
  return new Promise((resolve) => {
    const child = spawn(process.execPath, [WORKER], { stdio: ['pipe', 'pipe', 'inherit'] });
    const answer = once(resolve);
    let out = '';
    let watchdog = null;
    const finish = (value) => { clearTimeout(watchdog); answer(value); };
    watchdog = setTimeout(() => {
      process.stderr.write(`procoder: scan worker took longer than ${timeoutMs}ms for ${payload.files.length} files — killed; scanning its slice here instead\n`);
      child.kill('SIGKILL');
      finish(null);
    }, timeoutMs);
    // Node keeps the event loop alive for a pending timer, and a scan that
    // finished must not hold the process open for the length of the watchdog.
    if (watchdog.unref) watchdog.unref();
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (chunk) => { out += chunk; });
    // A child that cannot even be spawned, or whose stdin closes under it,
    // resolves null like any other failure: the parent scans the slice itself.
    child.on('error', () => finish(null));
    child.stdin.on('error', () => {});
    child.on('close', (code) => finish(sliceResult(code, out)));
    child.stdin.end(JSON.stringify(payload));
  });
}

// One file, exactly as a worker would check it. The single place this process
// scans anything, so the probe and the sequential path cannot drift apart.
function scanOne(absPath, options, checkFile) {
  const out = checkFile(absPath, {
    repoRoot: options.repoRoot,
    config: options.config,
    maxFindings: Infinity,
    applyBaseline: options.applyBaseline,
    // The same budget the worker is handed, said out loud rather than left to
    // run.js's default, because the whole point is that one number governs
    // both paths and is visible in one place.
    budgetMs: budgetOf(options),
    toolAnswer: (options.toolAnswers && options.toolAnswers.get(absPath)) || null,
  });
  return { absPath, relPath: out.relPath, findings: out.findings, skipped: out.skipped };
}

function scanSequentially(files, options, checkFile) {
  return files.map((absPath) => scanOne(absPath, options, checkFile));
}

// What a file costs on this host, right now, from files this scan had to do
// anyway. Returns the results by index — the caller keeps them — and the
// per-file cost to extrapolate from, with the single most expensive sample
// dropped so one outlier cannot decide the run.
function probeCost(files, options, checkFile) {
  const stride = Math.max(1, Math.floor(files.length / PROBE_MAX_FILES));
  const done = new Map();
  const times = [];
  const started = Date.now();
  for (let i = 0; i < files.length; i += stride) {
    const at = Date.now();
    done.set(i, scanOne(files[i], options, checkFile));
    times.push(Date.now() - at);
    if (times.length >= PROBE_MIN_FILES && Date.now() - started >= PROBE_MS) break;
  }
  if (times.length >= PROBE_MIN_FILES) times.splice(times.indexOf(Math.max(...times)), 1);
  return { done, msPerFile: times.reduce((a, b) => a + b, 0) / times.length };
}

// The payload a worker needs. `toolAnswers` travels with it because the linters
// were already run once for the whole scan (see runToolBatches) — re-running
// them per worker would undo that saving several times over. `config` travels
// with it for the same reason the budget does: the worker must check the file
// the way this process would have, and a worker that rebuilds the config from
// disk answers to a different one whenever the caller built its own.
function payloadFor(slice, options) {
  const answers = options.toolAnswers || new Map();
  return {
    repoRoot: options.repoRoot,
    files: slice,
    config: options.config || null,
    applyBaseline: options.applyBaseline,
    budgetMs: budgetOf(options),
    toolAnswers: slice.filter((file) => answers.has(file)).map((file) => [file, answers.get(file)]),
  };
}

// Results for every file, in input order. `checkFile` is injected rather than
// required here so this module stays free of the engine's require graph in the
// parent — and so a test can prove the parallel and sequential paths agree.
//
// Async, because the workers run at the same time. Callers that do not want to
// be async can pass jobs: 1 and get the sequential path, which is also what a
// small file list gets.
async function runPool(files, options, checkFile, jobs) {
  const slices = sliceInto(files, jobs);
  const timeoutMs = sliceTimeoutMs(slices[0].length, budgetOf(options));
  const settled = await Promise.all(
    slices.map((slice) => runSlice(payloadFor(slice, options), timeoutMs)));
  // Reassembled in slice order, so the report does not depend on which worker
  // finished first — or on how many cores the machine has.
  return settled.flatMap((out, i) => (out === null
    // A dead worker is a slice nobody scanned. Doing it here costs the
    // parallelism for that slice and nothing else.
    ? scanSequentially(slices[i], options, checkFile)
    : out));
}

async function scanFiles(files, options, checkFile) {
  const jobs = clampJobs(options.jobs === undefined ? null : options.jobs);
  // One job means one process, and it means that ahead of everything else:
  // `--jobs 1` is how a user says "do not fork", and a container with a single
  // usable core says the same thing through defaultJobs.
  if (jobs < 2 || files.length < 2) return scanSequentially(files, options, checkFile);
  // `forceParallel` exists for the tests and for nothing else: it skips both the
  // measurement and the threshold, so a test can exercise the workers on a file
  // list far too cheap to be worth forking — which every list in this
  // repository is. It cannot make a scan wrong — a forked slice is required to
  // answer identically — only slower.
  if (options.forceParallel) return runPool(files, options, checkFile, jobs);

  const { done, msPerFile } = probeCost(files, options, checkFile);
  const rest = files.filter((file, i) => !done.has(i));
  // The decision, made once, on measured work: what is LEFT has to be worth a
  // pool, since what the probe scanned is already scanned. A single enormous
  // file leaves nothing behind and so never forks — which is exactly right,
  // because one file is one slice and forking it is pure loss (measured: 419ms
  // against 291ms).
  const scanned = (rest.length === 0 || msPerFile * rest.length < PARALLEL_MIN_WORK_MS)
    ? scanSequentially(rest, options, checkFile)
    : await runPool(rest, options, checkFile, jobs);
  // Back into input order: the probe's files sit where they came from and the
  // rest fill in around them, so the report is the sequential one either way.
  let next = 0;
  return files.map((file, i) => (done.has(i) ? done.get(i) : scanned[next++]));
}

module.exports = {
  scanFiles, defaultJobs, clampJobs, PARALLEL_MIN_WORK_MS, MAX_JOBS, SCAN_BUDGET_MS,
};
