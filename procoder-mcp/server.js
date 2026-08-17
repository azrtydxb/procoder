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

const PROTOCOL_VERSION = '2024-11-05';

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

function handle(request) {
  const { id, method, params } = request;

  if (method === 'initialize') {
    return {
      jsonrpc: '2.0', id,
      result: {
        protocolVersion: PROTOCOL_VERSION,
        capabilities: { tools: {} },
        serverInfo: { name: 'procoder', version: VERSION },
      },
    };
  }

  if (method === 'tools/list') {
    return { jsonrpc: '2.0', id, result: { tools: TOOLS } };
  }

  if (method === 'tools/call') {
    try {
      return { jsonrpc: '2.0', id, result: callTool(params && params.name, params && params.arguments) };
    } catch (e) {
      return { jsonrpc: '2.0', id, error: { code: e.code || -32603, message: e.message } };
    }
  }

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
