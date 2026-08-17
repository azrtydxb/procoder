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

function text(value) {
  return { content: [{ type: 'text', text: String(value) }] };
}

// The whole point of a gate is that it runs over what changed, and the server
// could only ever answer for one file at a time — so every host using it had to
// re-implement "which files changed" for itself, or not check at all.
//
// A git failure is reported as such, never as an empty result: "nothing to
// check" and "I could not work out what changed" are opposite answers, and the
// second one arriving as the first is how a gate goes quiet.
function changedFiles(repoRoot, since) {
  const out = [];
  for (const argv of [
    ['diff', '--name-only', '--diff-filter=ACM', `${since}...HEAD`],
    ['diff', '--name-only', '--diff-filter=ACM', 'HEAD'],
    ['ls-files', '--others', '--exclude-standard'],
  ]) {
    const r = spawnSync('git', argv, { cwd: repoRoot, encoding: 'utf8' });
    if (r.status !== 0) throw new Error(`git ${argv.join(' ')} failed: ${String(r.stderr || '').trim()}`);
    out.push(...String(r.stdout || '').split('\n').filter(Boolean));
  }
  return Array.from(new Set(out)).map((rel) => path.join(repoRoot, rel)).filter(fs.existsSync);
}

function reviewChanged(args) {
  const repoRoot = findRepoRoot(path.resolve(String(args.path || '.')));
  const since = String(args.since || 'HEAD');
  let files;
  try {
    files = changedFiles(repoRoot, since);
  } catch (e) {
    return `cannot review: ${e.message}`;
  }
  if (files.length === 0) return `no files changed since ${since}`;

  const config = loadConfig(repoRoot);
  const lines = [];
  let skipped = 0;
  for (const absPath of files) {
    const out = checkFile(absPath, { repoRoot, config, maxFindings: Infinity });
    if (out.skipped) { skipped += 1; continue; }
    if (out.findings.length) lines.push(formatFindings(out.findings, out.relPath));
  }
  const tail = skipped ? ` (${skipped} file${skipped === 1 ? '' : 's'} skipped)` : '';
  return lines.length === 0
    ? `clean: ${files.length} changed file${files.length === 1 ? '' : 's'}${tail}`
    : `${lines.join('\n')}\n\n${files.length} changed files${tail}`;
}

function checkOneFile(args) {
  const absPath = path.resolve(String(args.path || ''));
  const repoRoot = findRepoRoot(path.dirname(absPath));
  const config = loadConfig(repoRoot);
  const { relPath, findings, skipped } = checkFile(absPath, { repoRoot, config });
  if (skipped) return `skipped (${skipped}): ${relPath}`;
  if (findings.length === 0) return `clean: ${relPath}`;
  return formatFindings(findings, relPath);
}

function baselineSummary(args) {
  const repoRoot = findRepoRoot(path.resolve(String(args.path || '.')));
  const config = loadConfig(repoRoot);
  // Required lazily: the baseline module is only needed on this path.
  const { loadBaseline } = require('../hooks/checks/baseline');
  const size = loadBaseline(repoRoot, config).size;
  return size === 0
    ? 'No baseline recorded. Every finding is reported.'
    : `${size} pre-existing findings accepted and suppressed. New code is gated in full.`;
}

// A Map, not a chain of ifs: the chain reached four branches and the complexity
// threshold, and argv-adjacent input must never reach a method on
// Object.prototype — the same reason bin/procoder.js dispatches this way.
const HANDLERS = new Map([
  // `digest: true` renders the subagent cut — every rung, without the session
  // mechanics a one-shot caller cannot act on. Offered here because an MCP host
  // pays this text per conversation exactly as SubagentStart does.
  ['procoder_doctrine', (args) => getProcoderInstructions(args.level || 'strict', { digest: !!args.digest })],
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

function toolsCallResult(id, params) {
  try {
    return { jsonrpc: '2.0', id, result: callTool(params && params.name, params && params.arguments) };
  } catch (e) {
    return { jsonrpc: '2.0', id, error: { code: e.code || -32603, message: e.message } };
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

let buffer = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buffer += chunk;
  let index;
  while ((index = buffer.indexOf('\n')) >= 0) {
    const line = buffer.slice(0, index).trim();
    buffer = buffer.slice(index + 1);
    if (!line) continue;

    let response;
    try {
      response = handle(JSON.parse(line));
    } catch (e) {
      // A malformed line must not take the server down.
      response = { jsonrpc: '2.0', id: null, error: { code: -32700, message: 'parse error' } };
    }
    if (response) process.stdout.write(JSON.stringify(response) + '\n');
  }
});
