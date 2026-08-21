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
