// Package textutil holds the small text operations several domains had each
// written for themselves. Consolidation is not tidiness here: `slugify` was
// copied into three packages and then hand-patched in all three for the
// Windows path limit, which is how the fourth copy gets missed. These have
// one definition because their behaviour must not drift.
//
// Only genuinely identical helpers live here. A package whose variant reads
// differently on purpose — a byte scanner, a different cap, an error
// folded into the fallback — keeps its own, because moving those would
// change output under the cover of a cleanup.
package textutil

import "strings"

// maxLine is the width a single reported line is truncated to: long enough
// for a compiler's first complaint, short enough not to flood a report.
const maxLine = 160

// FirstLine returns the first line with content, trimmed and truncated —
// the shape every tool report uses when it quotes what a failing command
// said. An input with nothing in it answers "no output" rather than an
// empty string, so a report never shows a blank where a reason belongs.
func FirstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return Trim(t)
		}
	}
	return "no output"
}

// Trim caps a line at the reporting width.
func Trim(s string) string {
	if len(s) > maxLine {
		return s[:maxLine]
	}
	return s
}

// maxSlug bounds a generated file name. Windows caps a full path at 260
// characters and CI checkouts sit deep inside the runner's workspace; a
// slug born from a whole sentence — a seeded acceptance criterion — must
// not push a file past that.
const maxSlug = 60

// Slug turns a title into a file name: lowercase, alphanumerics and single
// dashes, cut at a word boundary. Empty means the title carried nothing a
// file name can be made of, and the caller refuses.
func Slug(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlug {
		cut := slug[:maxSlug]
		if i := strings.LastIndexByte(cut, '-'); i > 40 {
			cut = cut[:i]
		}
		slug = strings.Trim(cut, "-")
	}
	return slug
}

// Section returns the body between "## name" and the next section heading —
// how every Markdown-backed domain (spec, plan, todo, backlog, adr) reads
// one part of a document.
func Section(text, name string) string {
	i := strings.Index(text, "## "+name)
	if i < 0 {
		return ""
	}
	body := text[i+len("## "+name):]
	if j := strings.Index(body, "\n## "); j >= 0 {
		body = body[:j]
	}
	return body
}

// StripComments removes HTML comments, so a template's guidance text never
// counts as the content a controller is asking the writer to supply.
func StripComments(s string) string {
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+3:]
	}
}
