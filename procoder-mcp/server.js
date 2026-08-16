#!/usr/bin/env node
// procoder — MCP server (JSON-RPC 2.0 over stdio, newline-delimited).
//
// Exposes the same engine the Claude Code hooks use, for hosts that speak MCP
// but not Claude Code plugins. Hand-rolled: this needs three methods, and a
// dependency here would be the exact rung-1 addition procoder argues against.

const path = require('path');
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
    description: 'Return the procoder doctrine — the four rungs (SAFE, TRUE, OBVIOUS, ALONE) that gate whether code may ship — filtered to an intensity level.',
    inputSchema: {
      type: 'object',
      properties: {
        level: { type: 'string', enum: ['pragmatic', 'strict', 'paranoid'], description: 'Intensity level. Defaults to strict.' },
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

function callTool(name, args = {}) {
  if (name === 'procoder_doctrine') {
    return text(getProcoderInstructions(args.level || 'strict'));
  }

  if (name === 'procoder_check') {
    const absPath = path.resolve(String(args.path || ''));
    const repoRoot = findRepoRoot(path.dirname(absPath));
    const config = loadConfig(repoRoot);
    const { relPath, findings, skipped } = checkFile(absPath, { repoRoot, config });
    if (skipped) return text(`skipped (${skipped}): ${relPath}`);
    if (findings.length === 0) return text(`clean: ${relPath}`);
    return text(formatFindings(findings, relPath));
  }

  if (name === 'procoder_baseline') {
    const repoRoot = findRepoRoot(path.resolve(String(args.path || '.')));
    const config = loadConfig(repoRoot);
    // Required lazily: the baseline module is only needed on this path.
    const { loadBaseline } = require('../hooks/checks/baseline');
    const size = loadBaseline(repoRoot, config).size;
    return text(size === 0
      ? 'No baseline recorded. Every finding is reported.'
      : `${size} pre-existing findings accepted and suppressed. New code is gated in full.`);
  }

  throw Object.assign(new Error(`unknown tool: ${name}`), { code: -32602 });
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
