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

// Right-sizing is naming which entry point a change belongs at, not
// removing enforcement — D-5. The premise procoder is often described by,
// that every change must run spec → plan → backlog → sprint, does not
// survive contact with the code: no gate finding requires a spec, and
// this repository routinely lands fixes that never had one. What was
// missing was anyone saying so.
// proved by: dropped the build entry from Entries — the report implies
// the smallest change still starts at a written todo, which is the
// belief that made the chain feel mandatory when it never was.
func TestEveryEntryPointIsNamedIncludingTheSmallest(t *testing.T) {
	var lines []string
	if code := Where(t.TempDir(), func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("a report cannot fail: exit %d", code)
	}
	joined := strings.Join(lines, "\n")

	// Matched on the entry's own text, not on its name: the closing prose
	// mentions "build" too, so a bare substring check passes even with the
	// build entry removed. That false positive was real — the mutation
	// this test names sailed through the first version of this assertion.
	for _, want := range []string{"analysis", "spec", "plan", "backlog", "todo", "build"} {
		var e Entry
		for _, c := range Entries {
			if c.Name == want {
				e = c
			}
		}
		if e.Name == "" {
			t.Errorf("entry point %q is missing from the set entirely", want)
			continue
		}
		if e.When == "" || e.Next == "" {
			t.Errorf("entry %s must say when it fits and what to run", want)
			continue
		}
		if !strings.Contains(joined, e.When) || !strings.Contains(joined, e.Next) {
			t.Errorf("entry point %q must reach the report with its guidance:\n%s", want, joined)
		}
	}
	// And it says plainly that starting low is allowed, which is the
	// whole finding: the rigidity was believed, not enforced.
	if !strings.Contains(joined, "No gate finding") {
		t.Errorf("the report must say the chain is not mandatory:\n%s", joined)
	}
}

// A repository mid-flight is told where it already is, so the advice
// lands against its own state rather than in the abstract.
// proved by: returned "" from furthest unconditionally — the report is
// identical for a fresh repository and one three sprints in.
func TestTheReportNamesWhereThisRepositoryAlreadyIs(t *testing.T) {
	bare := t.TempDir()
	var lines []string
	Where(bare, func(s string) { lines = append(lines, s) })
	if strings.Contains(strings.Join(lines, "\n"), "already entered at") {
		t.Error("a repository with no artifacts has not entered anywhere")
	}

	root := t.TempDir()
	write(t, root, ".procoder/specs/thing.md", "# thing\n")
	lines = nil
	Where(root, func(s string) { lines = append(lines, s) })
	if !strings.Contains(strings.Join(lines, "\n"), "already entered at: spec") {
		t.Errorf("a repository with specs has entered at spec: %v", lines)
	}

	// The deepest artifact wins: a repository with sprints is past the
	// point where "start at spec" is useful advice.
	write(t, root, ".procoder/backlog/sprints/001-x.md", "# x\n")
	lines = nil
	Where(root, func(s string) { lines = append(lines, s) })
	if !strings.Contains(strings.Join(lines, "\n"), "already entered at: sprint") {
		t.Errorf("the deepest artifact is where it is: %v", lines)
	}
}
