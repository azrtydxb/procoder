package copilot

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// secretMarker and pathMarker are fixed: a redaction that reveals a prefix,
// a suffix, or the length of what it hid is not a redaction. Every secret
// looks the same once it has been through here.
const (
	secretMarker = "[redacted]"
	pathMarker   = "[path]"
	userMarker   = "[user]"
)

// incompleteNote heads a body the parser could not read as the markdown it
// expected. The body is still scrubbed — code, secrets and paths are gone
// either way — but the reader is told the structure was not understood,
// rather than being handed something nobody verified as if it were clean.
const incompleteNote = "NOTE: sanitisation incomplete — the finding's markdown was not in the expected shape, so the whole body is reproduced below (code, secrets and paths removed)."

// titleMax keeps a derived issue title to one readable line.
const titleMax = 120

// Sanitise strips the user's code out of a Copilot finding and leaves the
// metadata about the failure: what the defect was, where it was, and what
// Copilot suggested. Nothing reaches GitHub or the ledger except through
// here, so every step fails towards removal — over-stripping costs a reader
// some context, under-stripping puts someone's source on the internet.
//
// The order matters. Code blocks go first, so a secret inside one is gone
// before any pattern has to recognise it; secrets go before paths, so a
// token that happens to look path-shaped is redacted rather than rewritten;
// paths go before identities, so a home directory is already relative when
// the username rules run.
//
// An empty Body is a real answer, not a failure: the caller skips creating
// an issue for a finding that was nothing but code.
func Sanitise(f Finding, root string) Sanitised {
	body, incomplete := stripCodeBlocks(f.Body)
	body = scrubText(body, root)
	body = cleanMarkdown(body)
	if body != "" && incomplete {
		body = incompleteNote + "\n\n" + body
	}

	line := f.Line
	if line == 0 {
		line = firstLineNumber(body)
	}
	return Sanitised{
		Title:       sanitiseTitle(f.Title, root),
		Body:        body,
		Line:        line,
		OriginalURL: f.OriginalURL,
		Created:     f.Created,
	}
}

// sanitiseTitle runs a title through the same scrub as the body — a title is
// published exactly as widely — then flattens it, because an issue title is
// one line whatever the source did.
func sanitiseTitle(title, root string) string {
	t, _ := stripCodeBlocks(title)
	t = cleanMarkdown(scrubText(t, root))
	t = strings.Join(strings.Fields(strings.ReplaceAll(t, "\n", " ")), " ")
	if len(t) > titleMax {
		// the ellipsis is three bytes, and the cut must land on a rune
		// boundary or the title ends in a broken character
		cut := titleMax - len("…")
		for cut > 0 && !utf8.RuneStart(t[cut]) {
			cut--
		}
		t = strings.TrimSpace(t[:cut]) + "…"
	}
	return t
}

// scrubText is the whole removal pass, in the one order that is safe. It is
// applied to every string that leaves this package, including the body kept
// after a parse failure: no path through Sanitise skips it.
func scrubText(text, root string) string {
	text = redactSecrets(text)
	text = relativiseRoot(text, root)
	text = redactAbsolutePaths(text)
	return stripIdentities(text)
}

// fenceRe matches an opening or closing code fence: three or more backticks
// or tildes, optionally indented, optionally followed by an info string.
var fenceRe = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})(.*)$")

// quoteRe matches the blockquote markers at the head of a line, nesting
// included. Copilot's review annotation IS a quote block, so the code it
// copied out of the user's file arrives quoted: fence detection has to see
// the line's content rather than its quoting, or every real finding smuggles
// its source past the fence rules.
var quoteRe = regexp.MustCompile(`^[ \t]*(?:>[ \t]?)+`)

// stripCodeBlocks removes fenced and indented code blocks entirely. The user
// asked Copilot to review their source, not to publish it, and a fenced block
// in a review comment is almost always a verbatim copy of it.
//
// The second return reports that a fence was opened and never closed. The
// block is still dropped — to end of input, because everything after an open
// fence is code until proven otherwise — but the finding is flagged as one
// this parser did not fully understand.
func stripCodeBlocks(text string) (string, bool) {
	text, htmlOpen := stripHTMLCode(text)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	fence := ""
	prevBlank := true
	inIndented := false
	for _, l := range lines {
		// unquote first, always: a fence, an indent, or a blank line means
		// the same thing whether or not Copilot quoted it
		l = quoteRe.ReplaceAllString(l, "")
		if fence != "" {
			if m := fenceRe.FindStringSubmatch(l); m != nil &&
				strings.HasPrefix(m[1], fence[:1]) && len(m[1]) >= len(fence) &&
				strings.TrimSpace(m[2]) == "" {
				fence = ""
			}
			continue
		}
		if m := fenceRe.FindStringSubmatch(l); m != nil {
			fence = m[1]
			inIndented = false
			continue
		}
		// An indented block only starts after a blank line; the same
		// indentation directly under a list item is a continuation of that
		// item's prose, and dropping it would take the defect description
		// with it.
		indented := strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t")
		blank := strings.TrimSpace(l) == ""
		switch {
		case inIndented && (indented || blank):
			if blank {
				out = append(out, "")
			}
			prevBlank = blank
			continue
		case indented && prevBlank:
			inIndented = true
			prevBlank = false
			continue
		}
		inIndented = false
		prevBlank = blank
		out = append(out, l)
	}
	return strings.Join(out, "\n"), fence != "" || htmlOpen
}

// htmlCodeRes match the HTML spellings of a code block. GitHub renders HTML
// inside issue bodies, so `<pre>` is a fence by another name — and RE2 has no
// backreference to match the closing tag against the opening one, hence one
// pattern per tag.
var htmlCodeRes = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<pre\b[^>]*>.*?</pre\s*>`),
	regexp.MustCompile(`(?is)<code\b[^>]*>.*?</code\s*>`),
	regexp.MustCompile(`(?is)<samp\b[^>]*>.*?</samp\s*>`),
}

// unclosedHTMLCodeRe finds an opening code tag that survived the pass above,
// which means it was never closed.
var unclosedHTMLCodeRe = regexp.MustCompile(`(?is)<(?:pre|code|samp)\b`)

// stripHTMLCode removes HTML code blocks, and reports an opening tag that was
// never closed. An unclosed tag is treated exactly like an unclosed fence:
// everything after it is code until something proves otherwise, and nothing
// proves it here.
func stripHTMLCode(text string) (string, bool) {
	for _, re := range htmlCodeRes {
		text = re.ReplaceAllString(text, "")
	}
	if loc := unclosedHTMLCodeRe.FindStringIndex(text); loc != nil {
		return text[:loc[0]], true
	}
	return text, false
}

// The secret patterns, most specific first. Each one matches a shape that is
// only ever a credential, so a hit needs no judgment call — except the last
// two, which trade some over-redaction of prose for never missing a value
// that was handed to a key whose name says it is a secret.
var secretPatterns = []struct {
	re   *regexp.Regexp
	with string
}{
	// a PEM block is a secret in its entirety, newlines and all
	{regexp.MustCompile(`(?s)-----BEGIN[ A-Z]*PRIVATE KEY-----.*?-----END[ A-Z]*PRIVATE KEY-----`), secretMarker},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{16,}`), secretMarker},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{16,}`), secretMarker},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`), secretMarker},
	{regexp.MustCompile(`\bAIza[A-Za-z0-9_-]{10,}`), secretMarker},
	{regexp.MustCompile(`\b(?:AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{12,}`), secretMarker},
	{regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{8,}`), "Bearer " + secretMarker},
	{regexp.MustCompile(`(?i)\bauthorization\s*:\s*\S+`), "Authorization: " + secretMarker},
	{regexp.MustCompile(`(?i)\b([A-Za-z0-9_.-]*(?:passwd|password|secret|token|api[_-]?key|apikey|access[_-]?key|private[_-]?key)[A-Za-z0-9_.-]*)\s*[:=]\s*(?:"[^"\n]*"|'[^'\n]*'|[^\s"',;)]{6,})`), "${1}=" + secretMarker},
}

// redactSecrets replaces every credential-shaped run with one fixed marker.
func redactSecrets(text string) string {
	for _, p := range secretPatterns {
		text = p.re.ReplaceAllString(text, p.with)
	}
	return text
}

// relativiseRoot rewrites paths inside the project as paths relative to it,
// forward-slashed on every platform. The absolute form carries the user's
// home directory and therefore their name; the relative form carries the
// only part a reader of the issue needs.
//
// A match that is not followed by a separator is left alone — `/tmp/xyz` is
// not inside `/tmp/x` — and redactAbsolutePaths deals with it afterwards.
func relativiseRoot(text, root string) string {
	if strings.TrimSpace(root) == "" {
		return text
	}
	// not filepath.ToSlash: a finding can carry a Windows path on any host,
	// and ToSlash only knows the separator of the machine we happen to run on
	clean := forwardSlash(filepath.Clean(root))
	if clean == "" || clean == "." || clean == "/" {
		return text
	}
	seen := map[string]bool{}
	for _, variant := range []string{clean, strings.ReplaceAll(clean, "/", `\`)} {
		if seen[variant] {
			continue
		}
		seen[variant] = true
		text = replaceRootPrefix(text, variant)
	}
	return text
}

func replaceRootPrefix(text, root string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, root)
		if i < 0 {
			break
		}
		b.WriteString(text[:i])
		rest := text[i+len(root):]
		if rest != "" && isPathChar(rest[0]) && rest[0] != '/' && rest[0] != '\\' {
			// a longer name that merely starts with the root's name
			b.WriteString(root)
			text = rest
			continue
		}
		n := 0
		for n < len(rest) && isPathChar(rest[n]) {
			n++
		}
		tail := strings.TrimLeft(forwardSlash(rest[:n]), "/")
		if tail == "" {
			tail = "."
		}
		b.WriteString(tail)
		text = rest[n:]
	}
	b.WriteString(text)
	return b.String()
}

// forwardSlash is the repo's one path rule applied to text rather than to a
// filename: every path this package emits is forward-slashed, on every
// platform, whatever platform the finding came from.
func forwardSlash(p string) string { return strings.ReplaceAll(p, `\`, "/") }

func isPathChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("._-+~/\\", c) >= 0
}

// absPathRe matches a URL or an absolute filesystem path. The two are one
// alternation on purpose: the URL alternative is first, and Go's regexp
// prefers it, so the link back to Copilot's own issue survives while
// `/Users/<someone>/...` does not.
//
// The path alternative carries a leading boundary character because RE2 has
// no lookbehind, and without one the `/store/db.go` inside the RELATIVE path
// `internal/store/db.go` matches — which would destroy the very file:line
// that relativiseRoot just produced.
var absPathRe = regexp.MustCompile(`(?m)[a-zA-Z][a-zA-Z0-9+.-]*://[^\s)>\]"']+|(?:^|[^A-Za-z0-9._~+:/\\-])(?:[A-Za-z]:)?[\\/](?:[A-Za-z0-9._~+-]+[\\/])+[A-Za-z0-9._~+-]*`)

// redactAbsolutePaths removes what is left absolute after the project root
// was folded away: anything above the root, or on a different tree entirely,
// names a machine and usually a person. Relative paths — which is what a
// file:line position is by now — are untouched.
func redactAbsolutePaths(text string) string {
	return absPathRe.ReplaceAllStringFunc(text, func(m string) string {
		if strings.Contains(m, "://") {
			return m
		}
		// the boundary character belongs to the surrounding prose, not to
		// the path; a match at a line start or on a separator has none
		lead := ""
		if c := m[0]; c != '/' && c != '\\' && (len(m) < 2 || m[1] != ':') {
			lead = m[:1]
		}
		return lead + pathMarker
	})
}

var (
	emailRe   = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	mentionRe = regexp.MustCompile(`@[A-Za-z0-9][A-Za-z0-9-]{0,38}(?:\[bot\])?`)
	// both Copilot bot spellings are in the wild; see the spec's Q3
	copilotRe = regexp.MustCompile(`(?i)^copilot[a-z0-9-]*(?:\[bot\])?$`)
)

// stripIdentities removes the people. Copilot's own bot names stay, because
// naming the reviewer is the point of the record and a bot is not a person.
func stripIdentities(text string) string {
	text = emailRe.ReplaceAllString(text, secretMarker)
	return mentionRe.ReplaceAllStringFunc(text, func(m string) string {
		if copilotRe.MatchString(m[1:]) {
			return m
		}
		return userMarker
	})
}

var (
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	// RE2 has no backreferences, so the three horizontal-rule characters are
	// spelled out rather than matched against the first one
	ruleRe     = regexp.MustCompile(`^[ \t]*(?:-[ \t]*-[ \t]*-[- \t]*|\*[ \t]*\*[ \t]*\*[* \t]*|_[ \t]*_[ \t]*_[_ \t]*)$`)
	badgeRe    = regexp.MustCompile(`(?i)^\**copilot(\[bot\])?\**:?$`)
	blankRunRe = regexp.MustCompile(`\n{3,}`)
)

// cleanMarkdown takes off Copilot's wrapper — the quote block, the rule above
// it, the badge line, the template comments — and leaves the prose: the
// defect description, the position, and the suggested fix. It is deliberately
// forgiving; anything it fails to recognise is prose that has already been
// scrubbed, so the worst case is a reader seeing one line of Copilot's
// furniture, not a leak.
func cleanMarkdown(text string) string {
	text = htmlCommentRe.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if q := strings.TrimLeft(l, " \t"); strings.HasPrefix(q, ">") {
			l = strings.TrimPrefix(strings.TrimPrefix(q, ">"), " ")
		}
		t := strings.TrimSpace(l)
		if ruleRe.MatchString(l) || badgeRe.MatchString(t) {
			continue
		}
		out = append(out, strings.TrimRight(l, " \t"))
	}
	return strings.TrimSpace(blankRunRe.ReplaceAllString(strings.Join(out, "\n"), "\n\n"))
}

// positionRe matches a `file.ext:123` position, the one piece of location
// detail worth keeping.
var positionRe = regexp.MustCompile(`[A-Za-z0-9._/-]+\.[A-Za-z0-9]+:(\d+)`)

// firstLineNumber recovers the line the finding is about when the caller did
// not already know it, so the position survives even if only the prose had it.
func firstLineNumber(body string) int {
	m := positionRe.FindStringSubmatch(body)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}
