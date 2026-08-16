#!/usr/bin/env node
// procoder — comment stripping shared by the six language packs.
//
// The packs match their rules against the text the language will *execute*.
// Three kinds of text sit in a source file:
//
//   comments       — never executed. A line warning "never build SQL with
//                    Sprintf" is documentation, not a sink. Blanked, always.
//   regex literals — never executed as the thing they spell. A pattern that
//                    names a sink is a matcher for it; a codebase whose job is
//                    matching sinks would otherwise flag itself. Blanked in
//                    JS/TS, the only pack whose grammar has them.
//   string literals— routinely executed by the sink that consumes them: SQL,
//                    shell words, HTML, generated code. Kept, always. Half the
//                    sink rules can only match by seeing them at all —
//                    `"SELECT " + id`, `f"..."`, `$"..."`, a template literal.
//
// So this strips comments (and regex bodies) and never strings. A rule whose
// pattern is a syntactic operator rather than a sink — a nested ternary cannot
// occur inside a literal — asks shape.js's `stripNoise` instead, via `onCode`.
//
// Line numbers and column offsets survive: bodies are replaced with spaces,
// never deleted, exactly as stripNoise does.
//
// Not a parser — a single left-to-right scan, linear in input length, with no
// backtracking. Where it cannot tell (an unterminated quote, a `/` that may be
// division) it keeps the text as code, which can only leave a finding that is
// reported today, never hide one.

// A `/` in one of these positions opens a regex literal rather than dividing —
// the same restriction shape.js uses, and what keeps `a / b / c` out.
const REGEX_OPENS_AFTER = '(,=:[!&|?{};';

function blank(text) {
  return text.replace(/[^\n]/g, ' ');
}

// Index just past the string literal starting at `start`. A `'`/`"` literal
// that never closes stops at the newline: a Rust lifetime (`&'a str`) and an
// apostrophe in prose must not swallow the rest of the file. Triple quotes
// (Python) and backticks (JS/Go) do span lines.
function endOfString(text, start, triples) {
  const quote = text[start];
  const triple = triples && quote !== '`' && text.startsWith(quote.repeat(3), start);
  const close = triple ? quote.repeat(3) : quote;
  const spans = triple || quote === '`';
  let i = start + close.length;
  while (i < text.length) {
    if (text[i] === '\\') { i += 2; continue; }
    if (!spans && text[i] === '\n') return i;
    if (text.startsWith(close, i)) return i + close.length;
    i += 1;
  }
  return text.length;
}

// Index of the closing `/` of a regex literal, or -1 if the line ends first —
// in which case the `/` was division after all.
function endOfRegex(text, start) {
  let inClass = false;
  for (let i = start + 1; i < text.length; i += 1) {
    const ch = text[i];
    if (ch === '\\') { i += 1; continue; }
    if (ch === '\n') return -1;
    if (ch === '[') inClass = true;
    else if (ch === ']') inClass = false;
    else if (ch === '/' && !inClass) return i;
  }
  return -1;
}

// style: 'py' (# comments, statement-position docstrings), 'c' (// and /* */),
// 'js' (as 'c', plus regex-literal bodies).
function stripComments(source, style = 'c') {
  const text = String(source || '');
  const py = style === 'py';
  const n = text.length;

  let out = '';
  let kept = 0; // start of the run not yet copied out
  let last = ''; // last non-space character of code, for the regex test
  let bareLine = true; // nothing but whitespace on this line so far
  let i = 0;

  const eol = (from) => (text.indexOf('\n', from) === -1 ? n : text.indexOf('\n', from));
  const drop = (end) => {
    out += text.slice(kept, i) + blank(text.slice(i, end));
    kept = end;
    i = end;
  };

  while (i < n) {
    const ch = text[i];

    if (py && ch === '#') { drop(eol(i)); continue; }
    if (!py && ch === '/' && text[i + 1] === '/') { drop(eol(i)); continue; }
    if (!py && ch === '/' && text[i + 1] === '*') {
      const close = text.indexOf('*/', i + 2);
      drop(close === -1 ? n : close + 2);
      continue;
    }

    if (ch === '"' || ch === "'" || ch === '`') {
      const end = endOfString(text, i, py);
      // A Python triple-quoted string opening a statement is a docstring: a
      // comment that happens to be spelled as a literal, and the place a
      // Python file documents the practice it is warning against.
      if (py && bareLine && text.startsWith(ch.repeat(3), i)) { drop(end); continue; }
      last = ch;
      bareLine = false;
      i = end;
      continue;
    }

    if (style === 'js' && ch === '/' && (last === '' || REGEX_OPENS_AFTER.includes(last))) {
      const end = endOfRegex(text, i);
      if (end !== -1) {
        out += text.slice(kept, i + 1) + blank(text.slice(i + 1, end));
        kept = end;
        i = end + 1;
        last = '/';
        bareLine = false;
        continue;
      }
    }

    if (ch === '\n') bareLine = true;
    else if (!/\s/.test(ch)) { last = ch; bareLine = false; }
    i += 1;
  }

  return out + text.slice(kept);
}

module.exports = { stripComments };
