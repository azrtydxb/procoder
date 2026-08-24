package release

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// newestEntry returns the body of the top `## <version>` section of this
// repository's own changelog — the text the release job publishes verbatim
// as the GitHub Release notes.
func newestEntry(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("the changelog must be readable: %v", err)
	}
	// CRLF leveled first: a Windows checkout rewrites line endings, so the
	// paragraph below would end in "\r" and an italic closing "_" would
	// never be the last character.
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("the changelog has no version heading")
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			return strings.Join(lines[start+1:i], "\n")
		}
	}
	return strings.Join(lines[start+1:], "\n")
}

var (
	headline    = regexp.MustCompile(`(?m)^\*\*(Added|Fixed|Changed|Removed|Security) — `)
	anyHeadline = regexp.MustCompile(`(?m)^\*\*`)
)

// The newest entry is what the release job publishes as the release notes,
// so the layout rules at the top of CHANGELOG.md are a promise to whoever
// lands on that page — not a style note. A convention nothing checks is one
// that decays entry by entry until the page is a wall again.
// proved by: dropped the leading "Fixed — " from a headline and the italic
// summary line — this test names both, where nothing else in the repository
// reads the changelog's shape at all.
func TestTheNewestChangelogEntryFollowsTheLayout(t *testing.T) {
	entry := newestEntry(t)

	// The summary is what a reader skimming a release page is guaranteed to
	// see, so it comes first, before any headline. It is checked as a
	// paragraph rather than a line because the changelog is hard-wrapped:
	// one sentence of prose occupies three lines, and a line-anchored check
	// would demand the summary be short enough not to wrap.
	//
	// Underscores as well as asterisks: prettier normalises *italic* to
	// _italic_ and prettier is what the gate runs, so accepting only the
	// asterisk form would fail every formatted changelog.
	first, _, _ := strings.Cut(strings.TrimSpace(entry), "\n\n")
	italic := func(s string, mark string) bool {
		return strings.HasPrefix(s, mark) && strings.HasSuffix(s, mark)
	}
	if !italic(first, "_") && !italic(first, "*") {
		t.Errorf("the entry must open with an italic one-sentence summary, got:\n%s", first)
	}

	kinds := headline.FindAllString(entry, -1)
	all := anyHeadline.FindAllString(entry, -1)
	if len(all) == 0 {
		t.Fatal("the entry has no **headline** at all")
	}
	// Every headline carries its kind. A reader must be able to tell a new
	// feature from a broken thing made whole without reading the paragraph.
	if len(kinds) != len(all) {
		t.Errorf("%d of %d headlines do not open with Added/Fixed/Changed/Removed/Security",
			len(all)-len(kinds), len(all))
	}
	// An entry a reader cannot get from the sentence to the change is a
	// claim they have to take on faith.
	if !strings.Contains(entry, "](https://github.com/") {
		t.Error("no headline links an issue or PR — the reader cannot reach the change")
	}
}

var (
	// A link into this repository's pull requests or issues — what lets a
	// reader get from a claim on the release page to the change that made
	// it true.
	changeLink = regexp.MustCompile(`\]\(https://github\.com/[^)]+/(?:pull|issues)/\d+\)`)
	// A mention that is not a link. Inside a plain .md file "@handle" is
	// text: it renders as prose on the release page and leads nowhere.
	bareHandle = regexp.MustCompile(`(^|[^\[\w/])@([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)\b`)
)

// paragraphs splits an entry on blank lines, so a headline and the prose
// that belongs to it are judged together — the link usually sits in the
// sentence after the bolded claim, not in the claim itself.
func paragraphs(entry string) []string {
	var out []string
	for _, p := range strings.Split(entry, "\n\n") {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// Every entry links the change that shipped it, and every contributor
// mention is a link rather than text. Both rules are written at the top of
// CHANGELOG.md and neither was enforced: they were added as prose in #157,
// after a merged commit credited the wrong person, and prose is what had
// just failed.
//
// This is the changelog's own version of the gap `spec check` now closes
// for specs — a promise with nothing testing it. It matters more here than
// most places because the release job publishes this entry verbatim: an
// unlinked claim is one a reader on the release page cannot check, and a
// bare "@handle" is a credit that renders as prose and leads nowhere.
// proved by: removed the PR link from any headline paragraph, or wrote a
// contributor as a bare @handle instead of a markdown link — this names
// the paragraph, where every other check on the changelog passes.
func TestTheNewestEntryLinksItsChangesAndItsPeople(t *testing.T) {
	entry := newestEntry(t)

	var unlinked []string
	for _, p := range paragraphs(entry) {
		if !anyHeadline.MatchString(p) {
			continue // prose continuing a headline, or the summary
		}
		if !changeLink.MatchString(p) {
			unlinked = append(unlinked, strings.SplitN(p, "\n", 2)[0])
		}
	}
	if len(unlinked) > 0 {
		t.Errorf("every entry links the PR or issue that shipped it — a claim a reader cannot follow is one they take on faith:\n  %s",
			strings.Join(unlinked, "\n  "))
	}

	for _, m := range bareHandle.FindAllStringSubmatch(entry, -1) {
		t.Errorf("contributor @%s is named but not linked — inside a .md file that is text, not a link; write [@%s](https://github.com/%s)",
			m[2], m[2], m[2])
	}
}
