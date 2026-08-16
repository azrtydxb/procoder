#!/usr/bin/env node
// procoder — TOML subset parser.
//
// Supports exactly what .procoder.toml needs: [tables], [dotted.tables],
// key = "string" | int | float | true/false, and single-line arrays of strings.
// Everything else is skipped rather than raising, because a malformed config
// must degrade to defaults, never break a session.
//
// procoder: subset parser, swap for a real TOML library if the config grows
// multi-line arrays, dates, or inline tables.

// The content of a quoted string, or null if this is not one.
function unquote(text) {
  const quote = text[0];
  const quoted = (quote === '"' || quote === "'") && text.length >= 2 && text.endsWith(quote);
  return quoted ? text.slice(1, -1) : null;
}

const BOOLEANS = new Map([['true', true], ['false', false]]);
const INTEGER = /^-?\d+$/;
const FLOAT = /^-?\d*\.\d+$/;

function parseArray(text) {
  const inner = text.slice(1, -1).trim();
  if (!inner) return [];
  return inner.split(',')
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => parseValue(item));
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
  let inSingle = false;
  let inDouble = false;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === '"' && !inSingle) inDouble = !inDouble;
    else if (ch === "'" && !inDouble) inSingle = !inSingle;
    else if (ch === '#' && !inSingle && !inDouble) return line.slice(0, i);
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

function parseToml(text) {
  const result = Object.create(null);
  let table = result;

  for (const rawLine of String(text || '').split(/\r?\n/)) {
    const line = stripComment(rawLine).trim();
    const header = TABLE_HEADER.exec(line);
    const pair = KEY_VALUE.exec(line);

    // Anything else is silently skipped: defaults beat a crash.
    if (header) table = tableAt(result, header[1]);
    else if (pair && !FORBIDDEN_KEYS.has(pair[1])) table[pair[1]] = parseValue(pair[2]);
  }

  return result;
}

module.exports = { parseToml };
