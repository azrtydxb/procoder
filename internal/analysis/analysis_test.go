package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The phase is only worth having if its documents are worth reading. A
// brief whose sections are still template comments has recorded nothing,
// and passing it would make the phase a formality that costs time and
// buys nothing — the same standard `spec check` already holds a spec to,
// for the same reason.
// proved by: dropped the StripComments call from Gaps — a document that
// is nothing but its own template passes as COMPLETE, and the phase
// becomes a file somebody created once.
func TestAHollowAnalysisIsRefused(t *testing.T) {
	hollow := "# probe\n\nStatus: open\n"
	for _, s := range Sections {
		hollow += "\n## " + s + "\n\n<!-- the prompt nobody answered -->\n"
	}
	gaps := Gaps(hollow)
	if len(gaps) != len(Sections) {
		t.Fatalf("every unanswered section is a gap, got %d of %d: %v", len(gaps), len(Sections), gaps)
	}

	// A heading that is not there at all is a different gap from one that
	// is there and empty — a reader fixing the first adds a section, the
	// second fills one.
	missing := Gaps("# probe\n\nStatus: open\n")
	for _, g := range missing {
		if !strings.Contains(g, "section missing") {
			t.Errorf("an absent section is missing, not empty: %q", g)
		}
	}

	filled := "# probe\n\nStatus: open\n"
	for _, s := range Sections {
		filled += "\n## " + s + "\n\nSomething a person actually wrote.\n"
	}
	if gaps := Gaps(filled); len(gaps) != 0 {
		t.Errorf("a document somebody filled in passes: %v", gaps)
	}
}

// Check reports per document and refuses the tree while any is hollow.
// proved by: returned 0 from Check whatever the gaps — `analyze check`
// says NOT ready and exits clean, so nothing downstream can rely on it.
func TestCheckRefusesWhileAnyDocumentIsHollow(t *testing.T) {
	root := t.TempDir()
	write(t, root, Dir+"/hollow.md", "# hollow\n\n## Question\n\n<!-- unanswered -->\n")

	var lines []string
	if code := Check(root, "all", func(s string) { lines = append(lines, s) }); code != 1 {
		t.Fatalf("a hollow document refuses: exit %d\n%s", code, strings.Join(lines, "\n"))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "NOT ready") {
		t.Errorf("and says so: %v", lines)
	}

	// An empty tree is not a fault: the phase is available, never
	// required, so a repository that never used it is not failing.
	lines = nil
	if code := Check(t.TempDir(), "all", func(s string) { lines = append(lines, s) }); code != 0 {
		t.Errorf("no analysis is not a failure: exit %d %v", code, lines)
	}
}

// A spec that came from an analysis says so, and one that did not owes
// nobody a document recording that its author already knew what to build.
// proved by: returned the path unconditionally — every spec claims an
// analysis, including the ones with no such file, and the note points at
// something that is not there.
func TestASpecNamesItsAnalysisOnlyWhenThereIsOne(t *testing.T) {
	root := t.TempDir()
	write(t, root, Dir+"/thing.md", "# thing\n")

	if got := SpecSource(root, "thing"); !strings.Contains(got, "thing.md") {
		t.Errorf("a spec with an analysis names it: %q", got)
	}
	if got := SpecSource(root, "other"); got != "" {
		t.Errorf("a spec without one says nothing: %q", got)
	}
}
