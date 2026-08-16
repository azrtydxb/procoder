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

function parseValue(raw) {
  const text = raw.trim();
  if (text.startsWith('"') && text.endsWith('"') && text.length >= 2) {
    return text.slice(1, -1);
  }
  if (text.startsWith("'") && text.endsWith("'") && text.length >= 2) {
    return text.slice(1, -1);
  }
  if (text === 'true') return true;
  if (text === 'false') return false;
  if (text.startsWith('[') && text.endsWith(']')) {
    const inner = text.slice(1, -1).trim();
    if (!inner) return [];
    return inner.split(',')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => parseValue(item));
  }
  if (/^-?\d+$/.test(text)) return parseInt(text, 10);
  if (/^-?\d*\.\d+$/.test(text)) return parseFloat(text);
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

function parseToml(text) {
  const result = {};
  let table = result;

  for (const rawLine of String(text || '').split(/\r?\n/)) {
    const line = stripComment(rawLine).trim();
    if (!line) continue;

    const tableMatch = /^\[([A-Za-z0-9_.\-]+)\]$/.exec(line);
    if (tableMatch) {
      table = result;
      for (const part of tableMatch[1].split('.')) {
        if (typeof table[part] !== 'object' || table[part] === null) table[part] = {};
        table = table[part];
      }
      continue;
    }

    const pairMatch = /^([A-Za-z0-9_\-]+)\s*=\s*(.+)$/.exec(line);
    if (pairMatch) {
      table[pairMatch[1]] = parseValue(pairMatch[2]);
    }
    // Anything else is silently skipped: defaults beat a crash.
  }

  return result;
}

module.exports = { parseToml };
