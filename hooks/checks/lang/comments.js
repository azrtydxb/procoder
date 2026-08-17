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
// the file. Triple quotes and backticks do span lines.
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

// C#'s verbatim string: `@"…"`, where `\` is not an escape at all and `""` is
// how a quote is written. It spans lines, and reading its interior as code is
// exactly the false positive this scanner exists to prevent.
function verbatimEnd(text, start) {
  for (let i = start + 2; i < text.length; i += 1) {
    if (text[i] !== '"') continue;
    if (text[i + 1] === '"') i += 1;
    else return i + 1;
  }
  return text.length;
}

// Rust's raw string: `r"…"`, `r#"…"#`, `r##"…"##`. It closes only on a quote
// followed by the same number of hashes, spans lines, and has no escapes —
// which is why the ordinary scanner cut it at the first inner `"` and read the
// rest of it as code.
function rawStringOpen(text, i) {
  if (text[i] !== 'r' || /[\w$]/.test(text[i - 1] || ' ')) return -1;
  let hashes = 0;
  while (text[i + 1 + hashes] === '#') hashes += 1;
  return text[i + 1 + hashes] === '"' ? hashes : -1;
}

// The delimiters and the end of the literal starting at `i`, or null.
function stringSpan(text, i, py) {
  if (!py) {
    if (text[i] === '@' && text[i + 1] === '"') return { end: verbatimEnd(text, i), open: 2, close: 1 };
    const hashes = rawStringOpen(text, i);
    if (hashes !== -1) {
      const close = `"${'#'.repeat(hashes)}`;
      const open = hashes + 2;
      return { end: closingQuote(text, i + open, close, true), open, close: close.length };
    }
  }
  const quote = text[i];
  if (!QUOTES.includes(quote)) return null;
  // Triple quotes in every pack, not just Python: Java and Kotlin spell a text
  // block that way too, and three quotes in a row are not a thing C or JS can
  // mean by anything else.
  const triple = quote !== '`' && text.startsWith(quote.repeat(3), i);
  const close = triple ? quote.repeat(3) : quote;
  const end = closingQuote(text, i + close.length, close, triple || quote === '`');
  return { end, open: close.length, close: close.length };
}

function stringEnd(text, start, py) {
  const span = stringSpan(text, start, py);
  return span ? span.end : -1;
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

// The other half of what this scanner knows: where the *strings* are.
//
// Everything above is about text a rule must not read as code because it will
// never run. A multi-line string literal is the same problem seen from the
// other side — its interior is data, and it is full of things that look like
// code because quoting code is most of what a multi-line literal is for. A
// docstring holding an example, a SQL heredoc, a C# verbatim template, a Rust
// `r#"…"#` fixture: every one of them hands an indented, statement-shaped
// block to any rule that reads line structure.
//
// Two rules read line structure, and both were wrong on it. Python's nesting
// depth counted a docstring's indentation as real nesting — CPython's
// `test_enum.py` reported a depth-6 finding that is a string. And the taint
// scan, which now joins physical lines into statements, read `q = "SELECT " +
// id` inside a triple-quoted string, a C# `@"…"` and a Rust `r#"…"#` as three
// separate SQL injections.
//
// So: any literal that spans a line is blanked from just past its opening
// delimiter to its end, and everything else — comments included — is left
// exactly as it was. Each end of that range is where it is for a reason:
//
//   * the *opening* delimiter stays. It is the last thing on its line, and a
//     line ending in a quote is not a line ending in an operator — blanking it
//     would leave `doc = ` there, which the statement joiner reads as a wrapped
//     right-hand side and would splice the string's first line onto it.
//   * the *closing* delimiter goes, along with the whitespace in front of it.
//     A closing `"""` indented to column 57 is a non-blank line at column 57 to
//     any scan that counts indentation, which is exactly the finding this is
//     here to remove; and what follows it on that line — `" + id;` — survives
//     either way, because the delimiters are one character wide.
//
// This is deliberately *not* what stripComments does, and the two must not be
// merged. A string's content is what a sink executes — `"SELECT " + id`,
// `f"…"`, a template literal — so the text handed to the rules keeps every
// literal whole. Only the text a rule reads *structure* off gets this.
// Does this span cross a line? Walked rather than asked of `indexOf` or
// `lastIndexOf`, both of which search past the span to the end of the text: on
// a 400KB line holding no newline at all, that is a full scan per literal and
// so quadratic — measured at 4.7s before this loop replaced it. Here the spans
// partition the text, so the whole scan stays linear.
function spansLine(text, from, to) {
  for (let i = from; i < to; i += 1) if (text[i] === '\n') return true;
  return false;
}

// Comments and regex literals are skipped over rather than blanked — they are
// not this function's business, but they have to be *recognised*, or a quote
// inside one opens a literal that is not there. That is not hypothetical: this
// project's own `JS_TEMPLATE` is a regex holding a backtick, and reading it as
// a template literal blanked forty lines of real code below it.
function blankStringInteriors(source, style = 'c') {
  const text = String(source || '');
  const py = style === 'py';
  let out = '';
  let kept = 0;
  let last = '';
  let bareLine = true;
  let i = 0;

  while (i < text.length) {
    const comment = commentEnd(text, i, py);
    if (comment !== -1) { i = comment; continue; }

    const regex = regexEnd(text, i, style, last);
    if (regex !== -1) { last = '/'; bareLine = false; i = regex + 1; continue; }

    const span = stringSpan(text, i, py);
    if (!span) {
      [last, bareLine] = afterCode(text[i], last, bareLine);
      i += 1;
      continue;
    }
    const from = i + span.open;
    if (spansLine(text, from, span.end)) {
      out += text.slice(kept, from) + blank(text.slice(from, span.end));
      kept = span.end;
    }
    last = text[i];
    bareLine = false;
    i = span.end;
  }

  return out + text.slice(kept);
}

module.exports = { blankStringInteriors, stripComments };
