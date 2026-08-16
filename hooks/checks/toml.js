#!/usr/bin/env node
// procoder — TOML subset parser.
//
// Supports what a .procoder.toml plausibly contains, and nothing else:
//
//   [tables], [dotted.tables], and dotted keys (a.b = 1);
//   basic strings with escapes ("a\"b", "C:\\src\\", \n \t \r \b \f \uXXXX
//   \UXXXXXXXX) — an exclusion is a path or a regex, so escapes are load-bearing;
//   literal strings ('C:\src\'), TOML's backslash-free answer to Windows paths;
//   int, float, true/false;
//   arrays of those, single-line or spanning lines (trailing commas and
//   comments inside the array are fine).
//
// Deliberately unsupported, because this config's schema (see config.js:
// exclude.paths, exclude.rules, thresholds, rungs, baseline.file) has no place
// to put them: inline tables, dates, arrays of tables, and multi-line strings.
// Each of those, and every value this parser cannot read exactly — an unknown
// escape, an unterminated string, a bare unquoted word — is warned to stderr
// with file and line and the key is left unset. Nothing is ever guessed at:
// a config that silently loads a value the file does not say would enforce
// something other than what its author wrote, which is worse than a default.
//
// procoder: subset parser, swap for a real TOML library if the config ever
// grows a schema needing inline tables, dates, or arrays of tables.

// A value the parser refuses to guess at. Carries the reason for the warning;
// `reasonOf` recognizes it, and no supported value is a plain object.
const INVALID = Symbol('procoder.toml.invalid');
const invalid = (reason) => ({ [INVALID]: reason });
const reasonOf = (value) =>
  (value !== null && typeof value === 'object' && !Array.isArray(value) ? value[INVALID] : undefined);

const BOOLEANS = new Map([['true', true], ['false', false]]);
const INTEGER = /^-?\d+$/;
const FLOAT = /^-?\d*\.\d+$/;

// Updates `state` ({single, double, escape}) for one character of
// quote-tracking and returns whether that character sits inside a "..." or
// '...' string. Shared by every scanner below so each stays a
// one-branch-per-concern loop. A backslash inside a basic string escapes the
// next character, so `"a\"b"` is one string and the `#` in `"a\"#b"` is not a
// comment; literal strings have no escapes, which is their whole point.
function trackQuote(ch, state) {
  if (state.escape) { state.escape = false; return true; }
  if (state.double && ch === '\\') { state.escape = true; return true; }
  if (ch === '"' && !state.single) state.double = !state.double;
  else if (ch === "'" && !state.double) state.single = !state.single;
  return state.single || state.double;
}

const SIMPLE_ESCAPES = new Map([
  ['b', '\b'], ['t', '\t'], ['n', '\n'], ['f', '\f'], ['r', '\r'], ['"', '"'], ['\\', '\\'],
]);

// One \uXXXX / \UXXXXXXXX escape at `body[i]` (the 'u'), or null if it is not
// a well-formed scalar value — a lone surrogate or a past-max code point would
// make String.fromCodePoint throw, and this parser never throws.
function decodeUnicode(body, i) {
  const width = body[i] === 'u' ? 4 : 8;
  const hex = body.slice(i + 1, i + 1 + width);
  if (hex.length !== width || !/^[0-9a-fA-F]+$/.test(hex)) return null;
  const code = parseInt(hex, 16);
  if (code > 0x10ffff || (code >= 0xd800 && code <= 0xdfff)) return null;
  return { text: String.fromCodePoint(code), width };
}

// The body of a basic string with its escapes applied, or null if it contains
// an escape TOML does not define — which the caller refuses rather than
// loading `\q` verbatim and calling it the user's intent.
function decodeEscapes(body) {
  if (!body.includes('\\')) return body;
  let out = '';
  for (let i = 0; i < body.length; i += 1) {
    if (body[i] !== '\\') { out += body[i]; continue; }
    const code = body[i + 1];
    const simple = SIMPLE_ESCAPES.get(code);
    if (simple !== undefined) { out += simple; i += 1; continue; }
    if (code !== 'u' && code !== 'U') return null;
    const unicode = decodeUnicode(body, i + 1);
    if (!unicode) return null;
    out += unicode.text;
    i += 1 + unicode.width;
  }
  return out;
}

// `text` as exactly one quoted string: {quote, body}, or null if it is not one
// (unquoted, unterminated, or with anything trailing the closing quote — all
// of which the caller refuses rather than half-reading).
function readQuoted(text) {
  const quote = text[0];
  if (quote !== '"' && quote !== "'") return null;
  for (let i = 1; i < text.length; i += 1) {
    if (quote === '"' && text[i] === '\\') { i += 1; continue; }
    if (text[i] === quote) return i === text.length - 1 ? { quote, body: text.slice(1, -1) } : null;
  }
  return null;
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

// One unreadable item poisons the whole array: half an exclusion list is a
// wrong value, and a wrong value is the thing this parser must never load.
function parseArray(text) {
  const inner = text.slice(1, -1).trim();
  if (!inner) return [];
  const items = [];
  for (const raw of splitArrayItems(inner)) {
    const item = raw.trim();
    if (!item) continue;
    const value = parseValue(item);
    const reason = reasonOf(value);
    if (reason) return invalid(`${reason} (in array item ${item})`);
    items.push(value);
  }
  return items;
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

// A value that opens with a quote — so anything that is not one string,
// exactly, is an error rather than some other kind of value.
function parseString(text) {
  const quoted = readQuoted(text);
  if (!quoted) return invalid(`unterminated or malformed string, ignored: ${text}`);
  // Literal strings are verbatim by definition — that is why they exist.
  if (quoted.quote === "'") return quoted.body;
  const decoded = decodeEscapes(quoted.body);
  return decoded === null ? invalid(`unknown escape in string, ignored: ${text}`) : decoded;
}

function parseValue(raw) {
  const text = raw.trim();
  if (text[0] === '"' || text[0] === "'") return parseString(text);
  if (BOOLEANS.has(text)) return BOOLEANS.get(text);
  if (text.startsWith('[') && text.endsWith(']')) return parseArray(text);
  if (INTEGER.test(text)) return parseInt(text, 10);
  if (FLOAT.test(text)) return parseFloat(text);
  if (text.startsWith('{')) return invalid(`inline tables are not supported, ignored: ${text}`);
  // Dates, times, and bare words alike: unreadable, so unread. Quote it if it
  // was meant as a string.
  return invalid(`unsupported value, ignored: ${text}`);
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
// Dotted keys included: `thresholds.params = 4` says the same as a [thresholds]
// section with one key, and a user who writes it means it.
const KEY_VALUE = /^([A-Za-z0-9_\-]+(?:\.[A-Za-z0-9_\-]+)*)\s*=\s*(.+)$/;
const MULTILINE_STRING = /^("""|''')/;

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

// Stores one key — bare or dotted — in `table`. A forbidden name anywhere in
// the path drops the assignment entirely: tableAt hands back a detached table
// for a forbidden part, and the final part is checked here.
function assign(table, dottedKey, value) {
  const parts = dottedKey.split('.');
  const key = parts.pop();
  if (FORBIDDEN_KEYS.has(key)) return;
  const target = parts.length ? tableAt(table, parts.join('.')) : table;
  target[key] = value;
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

// Consumes the lines a """ or ''' string spans, so its content is not then
// read as config in its own right. Returns the last line index consumed — the
// file's end if the string never terminates, so an unterminated one can't hang
// or leak its body into the table.
function skipMultilineString(lines, i, firstValue) {
  const delimiter = firstValue.slice(0, 3);
  if (firstValue.slice(3).includes(delimiter)) return i;
  for (let n = i + 1; n < lines.length; n += 1) {
    if (lines[n].includes(delimiter)) return n;
  }
  return lines.length - 1;
}

// One value, and the last line it occupies. A non-array resolves on the spot.
// An array reads forward through `lines` until it closes, or until the file
// ends. A multi-line string is skipped whole and refused.
function resolveValue(lines, i, firstValue) {
  if (MULTILINE_STRING.test(firstValue)) {
    return {
      value: invalid('multi-line strings are not supported, ignored'),
      endLine: skipMultilineString(lines, i, firstValue),
    };
  }
  if (!firstValue.startsWith('[')) return { value: parseValue(firstValue), endLine: i };
  const array = collectArray(lines, i, firstValue);
  return {
    value: array.closed ? parseValue(array.valueText)
      : invalid('array is never closed with "]", ignored'),
    endLine: array.endLine,
  };
}

// A bracket line that is not a header this parser understands — `[[x]]` above
// all. Its keys must not land in the previous table, or an unrelated section
// could quietly add paths to [exclude], so warn and hand back a detached
// table: everything under the bad header goes nowhere, until the next header
// this parser does understand.
function detachBadHeader(name, lineNumber, line) {
  warn(name, lineNumber, line.startsWith('[[')
    ? `arrays of tables are not supported, section ignored: ${line}`
    : `unrecognized table header, section ignored: ${line}`);
  return Object.create(null);
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

    if (line[0] === '[') { table = detachBadHeader(name, i + 1, line); continue; }

    const pair = KEY_VALUE.exec(line);
    if (!pair) {
      warn(name, i + 1, `unrecognized syntax, ignored: ${line}`);
      continue;
    }

    const startLine = i + 1;
    const resolved = resolveValue(lines, i, pair[2].trim());
    i = resolved.endLine;
    const refusal = reasonOf(resolved.value);
    if (refusal) {
      warn(name, startLine, refusal);
      continue;
    }

    assign(table, pair[1], resolved.value);
  }

  return result;
}

module.exports = { parseToml };
