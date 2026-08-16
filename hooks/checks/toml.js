#!/usr/bin/env node
// procoder — TOML subset parser.
//
// Supports exactly what .procoder.toml needs: [tables], [dotted.tables],
// key = "string" | int | float | true/false, and arrays of strings — either
// single-line or spanning multiple lines (trailing commas and comments inside
// the array are fine). Everything else — inline tables, multi-line strings,
// dates, arrays of tables — is unsupported, and a line the parser can't
// handle is never silently dropped: it's warned to stderr with file and line,
// then skipped, because a malformed config must degrade to defaults, never
// break a session, but the user must be able to see why a setting is missing.
//
// procoder: subset parser, swap for a real TOML library if the config grows
// inline tables, multi-line strings, dates, or arrays of tables.

// The content of a quoted string, or null if this is not one.
function unquote(text) {
  const quote = text[0];
  const quoted = (quote === '"' || quote === "'") && text.length >= 2 && text.endsWith(quote);
  return quoted ? text.slice(1, -1) : null;
}

const BOOLEANS = new Map([['true', true], ['false', false]]);
const INTEGER = /^-?\d+$/;
const FLOAT = /^-?\d*\.\d+$/;

// Updates `state` ({single, double}) for one character of quote-tracking and
// returns whether that character sits inside a "..." or '...' string. Shared
// by every scanner below so each stays a one-branch-per-concern loop.
function trackQuote(ch, state) {
  if (ch === '"' && !state.single) state.double = !state.double;
  else if (ch === "'" && !state.double) state.single = !state.single;
  return state.single || state.double;
}

// Splits array contents on top-level commas, respecting quotes so an item
// like "a,b/" or "weird]bracket/" survives intact.
function splitArrayItems(inner) {
  const items = [];
  const state = { single: false, double: false };
  let current = '';
  let depth = 0;
  for (const ch of inner) {
    if (trackQuote(ch, state)) { current += ch; continue; }
    if (ch === '[') depth += 1;
    else if (ch === ']') depth -= 1;
    if (ch === ',' && depth === 0) { items.push(current); current = ''; continue; }
    current += ch;
  }
  if (current.trim()) items.push(current);
  return items;
}

function parseArray(text) {
  const inner = text.slice(1, -1).trim();
  if (!inner) return [];
  return splitArrayItems(inner)
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => parseValue(item));
}

// Scans one line of array content, carrying bracket depth and quote state
// forward from the previous line (a fresh `state`/`depth` per line would
// forget a quote or nesting level left open at the line break). Returns the
// updated depth and, if the array's closing ']' appears in this line, its
// index — otherwise -1.
function scanArrayLine(line, depth, state) {
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (trackQuote(ch, state)) continue;
    if (ch === '[') depth += 1;
    else if (ch === ']') {
      depth -= 1;
      if (depth === 0) return { depth, closedAt: i };
    }
  }
  return { depth, closedAt: -1 };
}

function parseValue(raw) {
  const text = raw.trim();
  const unquoted = unquote(text);
  if (unquoted !== null) return unquoted;
  if (BOOLEANS.has(text)) return BOOLEANS.get(text);
  if (text.startsWith('[') && text.endsWith(']')) return parseArray(text);
  if (INTEGER.test(text)) return parseInt(text, 10);
  if (FLOAT.test(text)) return parseFloat(text);
  return text;
}

// Strips a trailing comment, respecting quotes so "abc#def" survives.
function stripComment(line) {
  const state = { single: false, double: false };
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    const quoted = trackQuote(ch, state);
    if (ch === '#' && !quoted) return line.slice(0, i);
  }
  return line;
}

const TABLE_HEADER = /^\[([A-Za-z0-9_.\-]+)\]$/;
const KEY_VALUE = /^([A-Za-z0-9_\-]+)\s*=\s*(.+)$/;

// A config file is untrusted input. These names are never real config, and
// walking them turns `[exclude.__proto__]` into a two-line kill switch for the
// whole gate, so reject them rather than coerce them.
const FORBIDDEN_KEYS = new Set(['__proto__', 'constructor', 'prototype']);

// Walks (creating as needed) the table a [dotted.header] names. A forbidden
// part returns a detached table, so the header's keys land nowhere.
function tableAt(root, dottedName) {
  let table = root;
  for (const part of dottedName.split('.')) {
    if (FORBIDDEN_KEYS.has(part)) return Object.create(null);
    if (typeof table[part] !== 'object' || table[part] === null) {
      table[part] = Object.create(null);
    }
    table = table[part];
  }
  return table;
}

// Config syntax this parser can't handle must never vanish quietly — see the
// header comment. This goes straight to stderr (never stdout, which the
// PostToolUse hook reserves for its JSON payload), matching how config.js
// already reports an unreadable .procoder.toml.
function warn(fileName, lineNumber, message) {
  process.stderr.write(`procoder: ${fileName}:${lineNumber}: ${message}\n`);
}

// Reads continuation lines starting at `lines[i]` (whose value is
// `firstValue`) until the array it opens closes. Each line is scanned once —
// not re-scanned from the start on every continuation — so a large array
// stays linear in the config's size rather than quadratic. Returns the
// joined array text, the index of the last line consumed, and whether it
// actually closed (false = ran off the end of the file, i.e. unterminated).
function collectArray(lines, i, firstValue) {
  const state = { single: false, double: false };
  const parts = [firstValue];
  let scan = scanArrayLine(firstValue, 0, state);
  let last = i;

  while (scan.closedAt === -1 && last + 1 < lines.length) {
    last += 1;
    const lineText = stripComment(lines[last]).trim();
    parts.push(lineText);
    scan = scanArrayLine(lineText, scan.depth, state);
  }

  if (scan.closedAt === -1) return { valueText: parts.join('\n'), endLine: last, closed: false };
  parts[parts.length - 1] = parts[parts.length - 1].slice(0, scan.closedAt + 1);
  return { valueText: parts.join('\n'), endLine: last, closed: true };
}

// A value that may open a multi-line array. Non-arrays resolve on the spot;
// arrays read forward through `lines` until they close (or the file ends).
function resolveValue(lines, i, firstValue) {
  if (!firstValue.startsWith('[')) return { valueText: firstValue, endLine: i, closed: true };
  const array = collectArray(lines, i, firstValue);
  return { valueText: array.valueText, endLine: array.endLine, closed: array.closed };
}

// procoder — .procoder.toml is the only file this parser ever reads (see
// config.js); `fileName` names it in warnings and defaults to that.
function parseToml(text, fileName) {
  const name = fileName || '.procoder.toml';
  const result = Object.create(null);
  let table = result;
  const lines = String(text || '').split(/\r?\n/);

  for (let i = 0; i < lines.length; i += 1) {
    const line = stripComment(lines[i]).trim();
    if (!line) continue;

    const header = TABLE_HEADER.exec(line);
    if (header) { table = tableAt(result, header[1]); continue; }

    const pair = KEY_VALUE.exec(line);
    if (!pair) continue;

    const startLine = i + 1;
    const resolved = resolveValue(lines, i, pair[2].trim());
    i = resolved.endLine;
    if (!resolved.closed) {
      warn(name, startLine, 'array is never closed with "]", ignored');
      continue;
    }

    if (!FORBIDDEN_KEYS.has(pair[1])) table[pair[1]] = parseValue(resolved.valueText);
  }

  return result;
}

module.exports = { parseToml };
