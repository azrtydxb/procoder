#!/usr/bin/env node
// procoder — MCP server (JSON-RPC 2.0 over stdio, newline-delimited).
//
// Exposes the same engine the Claude Code hooks use, for hosts that speak MCP
// but not Claude Code plugins. Hand-rolled: this needs three methods, and a
// dependency here would be the exact rung-1 addition procoder argues against.

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { loadConfig, findRepoRoot } = require('../hooks/checks/config');
const { checkFile } = require('../hooks/checks/run');
const { formatFindings } = require('../hooks/checks/finding');
const { skipDescriptor } = require('../hooks/checks/report');
const { getProcoderInstructions } = require('../hooks/procoder-instructions');

const { version: VERSION } = require('./package.json');

// A closed stdout (the client disconnecting) surfaces as an async 'error'
// event, not a synchronous throw, and an uncaught EPIPE would crash the server
// mid-session. Dropping the event is the whole point: there is nowhere left to
// report to once stdout is gone, and stderr belongs to the host.
//
// This used to sit inside a try/catch that swallowed whatever it caught. It
// caught nothing: neither the property write nor `on()` throws on a real
// stream, so the catch was a rung-2 swallowed error guarding an event that
// cannot happen. Deleted rather than marked.
if (!process.stdout.__procoderEpipeGuarded) {
  process.stdout.__procoderEpipeGuarded = true;
  process.stdout.on('error', () => {});
}

// MCP splits into two eras, and this server answers both.
//
//   modern (2026-07-28 and later)  no handshake at all. Every request carries
//                                  its protocol version in `_meta`, the server
//                                  answers each one statelessly, and
//                                  `server/discover` is mandatory.
//   legacy (2025-11-25 and older)  the `initialize` handshake that established
//                                  a session and a negotiated version.
//
// Being dual-era costs almost nothing here and is the difference between
// working and not working for whole categories of client: a modern client
// talking to a legacy-only server FAILS, with no fall-forward, and this server
// was pinned to 2024-11-05 — the first revision there ever was.
//
// It is stateless already, which is what makes the modern era honest to claim:
// nothing here has ever depended on a session, because every tool call resolves
// its own repository root and config from the path it is given.
const MODERN_VERSION = '2026-07-28';

// Newest first: the legacy handshake answers with the newest version both sides
// know, and a client that asks for something older than everything here gets
// the oldest we still speak rather than an error, which is what the handshake
// era expected.
const LEGACY_VERSIONS = ['2025-11-25', '2025-06-18', '2025-03-26', '2024-11-05'];
const SUPPORTED_VERSIONS = [MODERN_VERSION, ...LEGACY_VERSIONS];

const META_VERSION = 'io.modelcontextprotocol/protocolVersion';
const META_SERVER_INFO = 'io.modelcontextprotocol/serverInfo';

// -32022 is the spec's own code for it, and the shape matters: a modern client
// reads `data.supported` and retries with a version both sides speak. Returning
// a plain -32601 instead would leave it with nothing to retry.
const UNSUPPORTED_VERSION = -32022;

const TOOLS = [
  {
    name: 'procoder_doctrine',
    description: 'Return the procoder doctrine — the six rungs (SAFE, TRUE, OBVIOUS, ALONE, FAST, MEANT) that gate whether code may ship — filtered to an intensity level.',
    inputSchema: {
      type: 'object',
      properties: {
        level: { type: 'string', enum: ['pragmatic', 'strict', 'paranoid'], description: 'Intensity level. Defaults to strict.' },
        digest: { type: 'boolean', description: 'Return the shorter digest — every rung, without the session mechanics a one-shot caller cannot act on. Defaults to false.' },
      },
    },
  },
  {
    name: 'procoder_check',
    description: 'Run procoder checks on one file and return findings, one line each. Prefers the project\'s configured linter and always runs the universal pack (secrets, PII in logs, rot).',
    inputSchema: {
      type: 'object',
      properties: { path: { type: 'string', description: 'Absolute path to the file to check.' } },
      required: ['path'],
    },
  },
  {
    name: 'procoder_review',
    description: 'Check every file changed since a git ref (plus anything uncommitted) and return the findings, one line each. This is the diff-scoped gate: use it before proposing a change as finished.',
    inputSchema: {
      type: 'object',
      properties: {
        path: { type: 'string', description: 'Any path inside the repository.' },
        since: { type: 'string', description: 'Git ref to compare against. Defaults to HEAD, which is "what I have not committed yet".' },
      },
      required: ['path'],
    },
  },
  {
    name: 'procoder_baseline',
    description: 'Report the ratchet baseline for a repository: how many pre-existing findings are accepted and therefore suppressed.',
    inputSchema: {
      type: 'object',
      properties: { path: { type: 'string', description: 'Any path inside the repository.' } },
      required: ['path'],
    },
  },
];

// How a failure is signalled, and it is one decision applied everywhere.
//
// MCP gives two channels, and hosts treat them very differently. A JSON-RPC
// `error` is a protocol fault: hosts surface it to the USER, and most never
// feed it back to the model — the model's context then holds a tool call with
// no answer, which reads as "nothing to report". A result with `isError: true`
// is a tool that RAN and failed: it goes into the model's context as text, the
// same way findings do, and carries a flag the host can act on.
//
// A gate that could not run must land in the model's context, or the model
// proceeds believing the code was checked. So:
//
//   protocol error   the request never named a runnable check — unknown method,
//                    unknown tool, missing or non-string `path`, a protocol
//                    version this server does not speak. Nothing was attempted,
//                    and there is no gate answer to give.
//   isError result   everything else — a file that could not be read, a git
//                    command that failed, an internal throw. The check was
//                    attempted and did not produce a verdict, which is news the
//                    model must see.
//
// The text of a failed result never contains the word "clean", and says what
// was not checked. That is the CLI's rule (`verify` exits 2 for "cannot
// verify", skipped files are named on stderr) in the shape MCP has for it.
const ok = (body) => ({ text: String(body), failed: false });
const failed = (body) => ({ text: String(body), failed: true });

function text({ text: body, failed: isError }) {
  const result = { content: [{ type: 'text', text: body }] };
  if (isError) result.isError = true;
  return result;
}

// argv-adjacent input, validated at the boundary. A tool call naming no file
// used to resolve to the process's own working directory and answer for
// whatever it found there.
function requirePath(args) {
  const value = args && args.path;
  if (typeof value !== 'string' || value.trim() === '') {
    throw Object.assign(new Error('`path` is required and must be a non-empty string'),
      { code: -32602 });
  }
  return value;
}

// One wording for "this file was not checked", taken from the same table the
// SARIF and JSON reports use — including its severity split. An excluded or
// ignored file is a scope decision the project made: coverage lost, worth
// saying, not a broken run. A file that was in scope and could not be read is a
// hole in the scope itself, and that is what `executionSuccessful` drops to
// false for in SARIF and what `isError` means here.
function skipLine(relPath, reason) {
  return `${relPath} — ${skipDescriptor(reason).why} (${reason})`;
}

const skipIsError = (reason) => skipDescriptor(reason).level === 'error';

// The whole point of a gate is that it runs over what changed, and the server
// could only ever answer for one file at a time — so every host using it had to
// re-implement "which files changed" for itself, or not check at all.
//
// A git failure is reported as such, never as an empty result: "nothing to
// check" and "I could not work out what changed" are opposite answers, and the
// second one arriving as the first is how a gate goes quiet.
// `ACMRT`, the same filter `procoder check --since` uses and for the same
// reason: git reports a moved file as a rename rather than a delete plus an
// add, so `ACM` selected nothing at all for a commit that renamed a file
// carrying a live finding — content arriving under a path nothing has checked.
// `D` stays out: a deleted file has no bytes to read, and a rename is already
// named at its destination.
function changedFiles(repoRoot, since) {
  const out = [];
  for (const argv of [
    ['diff', '--name-only', '--diff-filter=ACMRT', `${since}...HEAD`],
    ['diff', '--name-only', '--diff-filter=ACMRT', 'HEAD'],
    ['ls-files', '--others', '--exclude-standard'],
  ]) {
    const r = spawnSync('git', argv, { cwd: repoRoot, encoding: 'utf8' });
    if (r.status !== 0) throw new Error(`git ${argv.join(' ')} failed: ${String(r.stderr || '').trim()}`);
    out.push(...String(r.stdout || '').split('\n').filter(Boolean));
  }
  return Array.from(new Set(out)).map((rel) => path.join(repoRoot, rel)).filter(fs.existsSync);
}

// A file this pass could not process at all. checkFile answers for the reasons
// it knows about; anything it THROWS on is a file that was not checked either,
// and swallowing that into a clean review is the failure this whole file is
// about.
function checkOne(absPath, { repoRoot, config }) {
  try {
    return checkFile(absPath, { repoRoot, config, maxFindings: Infinity });
  } catch (e) {
    return {
      relPath: path.relative(repoRoot, absPath).replace(/\\/g, '/'),
      findings: [],
      skipped: 'unreadable',
    };
  }
}

// The word "clean" is spent only on a run where every changed file was read.
// It used to be printed over a diff that was skipped entirely — the count of
// skips rode along in a parenthesis after it, which is not a retraction.
function reviewSummary({ files, checked, lines, skipped }) {
  const holed = skipped.some((s) => skipIsError(s.reason));
  const head = lines.length > 0
    ? `${lines.join('\n')}\n\n${checked} of ${files.length} changed files checked`
    : (skipped.length === 0
      ? `clean: ${files.length} changed file${files.length === 1 ? '' : 's'}`
      : `no findings in the ${checked} changed file${checked === 1 ? '' : 's'} that were checked`);
  const tail = skipped.length === 0 ? '' :
    `\n\n${skipped.length} of ${files.length} changed file${files.length === 1 ? '' : 's'} `
    + 'NOT checked — nothing in them was looked at, so this is not a clean review:\n'
    + skipped.map((s) => `  ${skipLine(s.file, s.reason)}`).join('\n');
  return (holed ? failed : ok)(head + tail);
}

function reviewChanged(args) {
  const repoRoot = findRepoRoot(path.resolve(requirePath(args)));
  const since = String(args.since || 'HEAD');
  let files;
  try {
    files = changedFiles(repoRoot, since);
  } catch (e) {
    // "I could not work out what changed" and "nothing changed" are opposite
    // answers, and this one checked no file at all.
    return failed(`cannot review: ${e.message}\nNo file was checked; this is not a clean result.`);
  }
  if (files.length === 0) return ok(`no files changed since ${since}`);

  const config = loadConfig(repoRoot);
  const lines = [];
  const skipped = [];
  let checked = 0;
  for (const absPath of files) {
    const out = checkOne(absPath, { repoRoot, config });
    if (out.skipped) { skipped.push({ file: out.relPath, reason: out.skipped }); continue; }
    checked += 1;
    if (out.findings.length) lines.push(formatFindings(out.findings, out.relPath));
  }
  return reviewSummary({ files, checked, lines, skipped });
}

// maxFindings Infinity, as the CLI uses. The hook's top-5 sample is a budget
// decision for an interactive PostToolUse pass that must not flood a model's
// context mid-edit; this is a caller asking a question and waiting for the
// answer, and truncating it silently told them five findings were all there was.
function checkOneFile(args) {
  const absPath = path.resolve(requirePath(args));
  const repoRoot = findRepoRoot(path.dirname(absPath));
  const config = loadConfig(repoRoot);
  const { relPath, findings, skipped } = checkFile(absPath, { repoRoot, config, maxFindings: Infinity });
  if (skipped) {
    const line = `not checked: ${skipLine(relPath, skipped)}. Nothing in this file was `
      + 'looked at — this is not a clean result.';
    return skipIsError(skipped) ? failed(line) : ok(line);
  }
  return findings.length === 0
    ? ok(`clean: ${relPath}`)
    : ok(formatFindings(findings, relPath));
}

function baselineSummary(args) {
  const repoRoot = findRepoRoot(path.resolve(requirePath(args)));
  const config = loadConfig(repoRoot);
  // Required lazily: the baseline module is only needed on this path.
  const { loadBaseline, BASELINE_VERSION } = require('../hooks/checks/baseline');
  const baseline = loadBaseline(repoRoot, config);
  // A baseline too old to match suppresses nothing, and answering "no baseline
  // recorded" for it is the same lie in miniature: the repository has accepted
  // debt that this run cannot see, so nothing it says about suppression holds.
  if (baseline.staleVersion !== undefined) {
    return failed(`The baseline file is format v${baseline.staleVersion}; this procoder writes `
      + `v${BASELINE_VERSION}. The fingerprint format changed and old entries cannot be migrated, `
      + 'so nothing is suppressed until `procoder baseline <paths>` is re-run.');
  }
  return baseline.size === 0
    ? ok('No baseline recorded. Every finding is reported.')
    : ok(`${baseline.size} pre-existing findings accepted and suppressed. New code is gated in full.`);
}

// A Map, not a chain of ifs: the chain reached four branches and the complexity
// threshold, and argv-adjacent input must never reach a method on
// Object.prototype — the same reason bin/procoder.js dispatches this way.
const HANDLERS = new Map([
  // `digest: true` renders the subagent cut — every rung, without the session
  // mechanics a one-shot caller cannot act on. Offered here because an MCP host
  // pays this text per conversation exactly as SubagentStart does.
  ['procoder_doctrine', (args) => ok(getProcoderInstructions(args.level || 'strict', { digest: !!args.digest }))],
  ['procoder_review', reviewChanged],
  ['procoder_check', checkOneFile],
  ['procoder_baseline', baselineSummary],
]);

function callTool(name, args = {}) {
  const handler = HANDLERS.get(name);
  if (!handler) throw Object.assign(new Error(`unknown tool: ${name}`), { code: -32602 });
  return text(handler(args));
}

// The version a modern request declares, or null for a legacy one. A request
// with no `_meta` version is legacy by construction — that is exactly how the
// two eras tell each other apart on stdio.
function declaredVersion(params) {
  const meta = params && params._meta;
  const version = meta && meta[META_VERSION];
  return typeof version === 'string' ? version : null;
}

// The legacy handshake: answer with the client's own version when it is one we
// speak, and with the newest we speak when it is not. Echoing an unknown
// version back would be a claim to speak it; answering the oldest would drag a
// current client backwards for no reason.
function negotiateLegacy(requested) {
  return LEGACY_VERSIONS.includes(requested) ? requested : LEGACY_VERSIONS[0];
}

function discoverResult(id) {
  return {
    jsonrpc: '2.0',
    id,
    result: {
      resultType: 'complete',
      supportedVersions: SUPPORTED_VERSIONS,
      capabilities: { tools: {} },
      _meta: { [META_SERVER_INFO]: { name: 'procoder', version: VERSION } },
      instructions: 'Gate code against procoder\'s six rungs. procoder_review checks '
        + 'everything changed since a git ref; procoder_check answers for one file; '
        + 'procoder_doctrine returns the rungs themselves.',
    },
  };
}

// A request declaring a version this server does not speak, refused with the
// list it does speak — whatever the request was asking for. Returns null when
// there is nothing to refuse.
function refuseVersion(id, params) {
  const declared = declaredVersion(params);
  if (declared === null || SUPPORTED_VERSIONS.includes(declared)) return null;
  return {
    jsonrpc: '2.0',
    id,
    error: {
      code: UNSUPPORTED_VERSION,
      message: 'Unsupported protocol version',
      data: { supported: SUPPORTED_VERSIONS, requested: declared },
    },
  };
}

function initializeResult(id, params) {
  return {
    jsonrpc: '2.0', id,
    result: {
      protocolVersion: negotiateLegacy(params && params.protocolVersion),
      capabilities: { tools: {} },
      serverInfo: { name: 'procoder', version: VERSION },
    },
  };
}

// An error carrying a `code` was raised deliberately by the dispatch above and
// is a protocol fault — the request never named a runnable check. Anything else
// is the engine failing partway through a check that was attempted, and comes
// back as a failed RESULT, because a host shows that to the model and typically
// shows a protocol error only to the user. A gate that did not run has to reach
// the model, or the model concludes the code was checked.
function toolsCallResult(id, params) {
  try {
    return { jsonrpc: '2.0', id, result: callTool(params && params.name, params && params.arguments) };
  } catch (e) {
    if (e && e.code) return { jsonrpc: '2.0', id, error: { code: e.code, message: e.message } };
    const why = (e && e.message) || String(e);
    return {
      jsonrpc: '2.0',
      id,
      result: text(failed(`procoder failed to run this check: ${why}\n`
        + 'Nothing was checked; this is not a clean result.')),
    };
  }
}

// One entry per method, for the same two reasons the tool dispatch is a Map:
// the chain had grown past the complexity threshold, and `method` is remote
// input that must not reach a name on Object.prototype.
//
// `server/discover` is mandatory for a modern server and doubles as the stdio
// backward-compatibility probe: a dual-era client sends it first and reads the
// answer to decide which era it is talking to.
const METHODS = new Map([
  ['server/discover', (id) => discoverResult(id)],
  ['initialize', initializeResult],
  ['tools/list', (id) => ({ jsonrpc: '2.0', id, result: { tools: TOOLS } })],
  ['tools/call', toolsCallResult],
]);

function handle(request) {
  const { id, method, params } = request;

  const refused = refuseVersion(id, params);
  if (refused) return refused;

  const respond = METHODS.get(method);
  if (respond) return respond(id, params);

  // Notifications carry no id and expect no response.
  if (id === undefined) return null;

  return { jsonrpc: '2.0', id, error: { code: -32601, message: `unknown method: ${method}` } };
}

const rpcError = (id, code, message) => ({ jsonrpc: '2.0', id, error: { code, message } });

// Nothing may throw out of here: a crashed server is indistinguishable from a
// clean one to a host, which is the same failure as a silent gate one level up.
// Parse failure and dispatch failure are separated because they are different
// news — reporting an internal throw as "parse error" tells the caller to fix
// a request that was fine.
function respondTo(line) {
  let request;
  try {
    request = JSON.parse(line);
  } catch (e) {
    return rpcError(null, -32700, 'parse error');
  }
  if (!request || typeof request !== 'object' || Array.isArray(request)) {
    return rpcError(null, -32600, 'invalid request: expected a JSON-RPC object');
  }
  try {
    return handle(request);
  } catch (e) {
    // Notifications carry no id and take no response, not even for a failure.
    if (request.id === undefined) return null;
    return rpcError(request.id, -32603, `internal error: ${(e && e.message) || String(e)}`);
  }
}

let buffer = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buffer += chunk;
  let index;
  while ((index = buffer.indexOf('\n')) >= 0) {
    const line = buffer.slice(0, index).trim();
    buffer = buffer.slice(index + 1);
    if (!line) continue;
    const response = respondTo(line);
    if (response) process.stdout.write(JSON.stringify(response) + '\n');
  }
});
