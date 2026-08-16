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
const QUOTES = '"\'`';

function blank(text) {
  return text.replace(/[^\n]/g, ' ');
}

function lineEnd(text, from) {
  const at = text.indexOf('\n', from);
  return at === -1 ? text.length : at;
}

// Index just past the comment starting at `i`, or -1 if none starts there.
function commentEnd(text, i, py) {
  if (py) return text[i] === '#' ? lineEnd(text, i) : -1;
  if (text.startsWith('//', i)) return lineEnd(text, i);
  if (!text.startsWith('/*', i)) return -1;
  const close = text.indexOf('*/', i + 2);
  return close === -1 ? text.length : close + 2;
}

// Index just past the string literal starting at `i`, or -1 if none starts
// there. A `'`/`"` literal that never closes stops at the newline: a Rust
// lifetime (`&'a str`) and an apostrophe in prose must not swallow the rest of
// the file. Triple quotes (Python) and backticks (JS/Go) do span lines.
function closingQuote(text, from, close, spans) {
  let i = from;
  while (i < text.length) {
    if (text[i] === '\\') i += 1;
    else if (!spans && text[i] === '\n') return i;
    else if (text.startsWith(close, i)) return i + close.length;
    i += 1;
  }
  return text.length;
}

function stringEnd(text, start, py) {
  const quote = text[start];
  if (!QUOTES.includes(quote)) return -1;
  const triple = py && quote !== '`' && text.startsWith(quote.repeat(3), start);
  const close = triple ? quote.repeat(3) : quote;
  return closingQuote(text, start + close.length, close, triple || quote === '`');
}

// Index of the closing `/` of a regex literal starting at `i`, or -1: not JS,
// not a `/`, in a position where `/` can only divide, or the line ends first.
function closingSlash(text, from) {
  let inClass = false;
  for (let j = from; j < text.length; j += 1) {
    const ch = text[j];
    if (ch === '\\') j += 1;
    else if (ch === '\n') return -1;
    else if (ch === '[') inClass = true;
    else if (ch === ']') inClass = false;
    else if (ch === '/' && !inClass) return j;
  }
  return -1;
}

function regexEnd(text, i, style, last) {
  if (style !== 'js' || text[i] !== '/') return -1;
  const opens = last === '' || REGEX_OPENS_AFTER.includes(last);
  return opens ? closingSlash(text, i + 1) : -1;
}

// A Python triple-quoted string opening a statement is a docstring: a comment
// that happens to be spelled as a literal, and the place a Python file
// documents the practice it is warning against. One used as a value is data.
function isDocstring(text, i, py, bareLine) {
  return py && bareLine && text.startsWith(text[i].repeat(3), i);
}

// The scanner's memory of the line it is on: the last non-space code character
// (which decides whether a `/` opens a regex) and whether the line is still
// blank (which decides whether a triple quote opens a docstring).
function afterCode(ch, last, bareLine) {
  if (ch === '\n') return [last, true];
  if (/\s/.test(ch)) return [last, bareLine];
  return [ch, false];
}

// style: 'py' (# comments, statement-position docstrings), 'c' (// and /* */),
// 'js' (as 'c', plus regex-literal bodies).
function stripComments(source, style = 'c') {
  const text = String(source || '');
  const py = style === 'py';
  let out = '';
  let kept = 0; // start of the run not yet copied out
  let last = ''; // last non-space character of code, for the regex test
  let bareLine = true; // nothing but whitespace on this line so far
  let i = 0;

  const drop = (end) => {
    out += text.slice(kept, i) + blank(text.slice(i, end));
    kept = end;
    i = end;
  };

  while (i < text.length) {
    const comment = commentEnd(text, i, py);
    if (comment !== -1) { drop(comment); continue; }

    const string = stringEnd(text, i, py);
    if (string !== -1 && isDocstring(text, i, py, bareLine)) { drop(string); continue; }
    if (string !== -1) { last = text[i]; bareLine = false; i = string; continue; }

    const regex = regexEnd(text, i, style, last);
    // The delimiters stay, only the pattern between them is blanked.
    if (regex !== -1) { i += 1; drop(regex); last = '/'; bareLine = false; i = regex + 1; continue; }

    [last, bareLine] = afterCode(text[i], last, bareLine);
    i += 1;
  }

  return out + text.slice(kept);
}

module.exports = { stripComments };
