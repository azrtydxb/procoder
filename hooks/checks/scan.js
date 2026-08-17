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
//   - Below the threshold, no process is forked at all. Forking costs tens of
//     milliseconds per child; a scan of eleven files would pay more for the
//     workers than the work.

const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const WORKER = path.join(__dirname, 'worker.js');

// Under this many files, sequential wins: each fork costs process startup plus
// the engine's own require graph, and both are paid before a single file is
// read.
const PARALLEL_MIN_FILES = 250;

// One per core, minus one for this process, and never more than eight: the
// scan is CPU-bound, so more workers than cores only adds scheduling, and the
// returns past eight are thin against the memory each child's heap costs.
function defaultJobs() {
  return Math.max(1, Math.min(8, (os.cpus() || []).length - 1));
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

function runSlice(payload) {
  return new Promise((resolve) => {
    const child = spawn(process.execPath, [WORKER], { stdio: ['pipe', 'pipe', 'inherit'] });
    let out = '';
    let failed = false;
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (chunk) => { out += chunk; });
    // A child that cannot even be spawned, or whose stdin closes under it,
    // resolves null like any other failure: the parent scans the slice itself.
    child.on('error', () => { failed = true; resolve(null); });
    child.stdin.on('error', () => {});
    child.on('close', (code) => { if (!failed) resolve(sliceResult(code, out)); });
    child.stdin.end(JSON.stringify(payload));
  });
}

function scanSequentially(files, options, checkFile) {
  const config = options.config;
  return files.map((absPath) => {
    const out = checkFile(absPath, {
      repoRoot: options.repoRoot,
      config,
      maxFindings: Infinity,
      applyBaseline: options.applyBaseline,
      toolAnswer: (options.toolAnswers && options.toolAnswers.get(absPath)) || null,
    });
    return { absPath, relPath: out.relPath, findings: out.findings, skipped: out.skipped };
  });
}

// The payload a worker needs. `toolAnswers` travels with it because the linters
// were already run once for the whole scan (see runToolBatches) — re-running
// them per worker would undo that saving several times over.
function payloadFor(slice, options) {
  return {
    repoRoot: options.repoRoot,
    files: slice,
    noIgnore: !!(options.config && options.config.noIgnore),
    applyBaseline: options.applyBaseline,
    toolAnswers: Array.from(
      (options.toolAnswers || new Map()).entries()).filter(([file]) => slice.includes(file)),
  };
}

// Results for every file, in input order. `checkFile` is injected rather than
// required here so this module stays free of the engine's require graph in the
// parent — and so a test can prove the parallel and sequential paths agree.
//
// Async, because the workers run at the same time. Callers that do not want to
// be async can pass jobs: 1 and get the sequential path, which is also what a
// small file list gets.
async function scanFiles(files, options, checkFile) {
  const jobs = options.jobs || defaultJobs();
  if (jobs < 2 || files.length < PARALLEL_MIN_FILES) {
    return scanSequentially(files, options, checkFile);
  }

  const slices = sliceInto(files, jobs);
  const settled = await Promise.all(slices.map((slice) => runSlice(payloadFor(slice, options))));
  // Reassembled in slice order, so the report does not depend on which worker
  // finished first — or on how many cores the machine has.
  return settled.flatMap((out, i) => (out === null
    // A dead worker is a slice nobody scanned. Doing it here costs the
    // parallelism for that slice and nothing else.
    ? scanSequentially(slices[i], options, checkFile)
    : out));
}

module.exports = { scanFiles, defaultJobs, PARALLEL_MIN_FILES };
