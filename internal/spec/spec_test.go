package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

// completeSpec answers every required section with substance.
func completeSpec() string {
	var b strings.Builder
	b.WriteString("# widget\n\nStatus: draft\n")
	for _, s := range Sections {
		b.WriteString("\n## " + s + "\n\n")
		switch s {
		case "Acceptance criteria":
			b.WriteString("- [ ] report renders with the network cable pulled\n")
			b.WriteString("- [ ] go test ./... exits 0\n")
		case "Open questions":
			// none — every decision resolved
		default:
			b.WriteString("A real answer for " + s + ".\n")
		}
	}
	return b.String()
}

func TestCheckPassesCompleteSpec(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "widget", completeSpec())
	out, lines := collect()
	if code := Check(root, "widget", out); code != 0 {
		t.Fatalf("exit %d, want 0:\n%s", code, strings.Join(*lines, "\n"))
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "COMPLETE") {
		t.Errorf("no COMPLETE verdict: %v", *lines)
	}
}

func TestCheckBlocksOnGaps(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		wantGap string
	}{
		{"missing section", func(s string) string {
			return strings.Replace(s, "## Failure modes", "## Renamed away", 1)
		}, "section missing: Failure modes"},
		{"empty section", func(s string) string {
			return strings.Replace(s, "A real answer for Edge cases.", "", 1)
		}, "section empty: Edge cases"},
		{"open question", func(s string) string {
			return s + "\n- OPEN: which database?\n"
		}, "unresolved OPEN:"},
		{"no checkboxes", func(s string) string {
			s = strings.Replace(s, "- [ ] report renders with the network cable pulled\n", "renders offline\n", 1)
			return strings.Replace(s, "- [ ] go test ./... exits 0\n", "", 1)
		}, "no checkboxes"},
		{"placeholder criterion", func(s string) string {
			return strings.Replace(s, "- [ ] report renders with the network cable pulled", "- [ ] ...", 1)
		}, "placeholder"},
		{"untestable criterion", func(s string) string {
			return strings.Replace(s, "- [ ] report renders with the network cable pulled",
				"- [ ] the UI is user-friendly", 1)
		}, "untestable criterion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeSpec(t, root, "widget", c.mutate(completeSpec()))
			out, lines := collect()
			if code := Check(root, "widget", out); code != 1 {
				t.Fatalf("exit %d, want 1 (blocked):\n%s", code, strings.Join(*lines, "\n"))
			}
			if !strings.Contains(strings.Join(*lines, "\n"), c.wantGap) {
				t.Errorf("gap %q not named:\n%s", c.wantGap, strings.Join(*lines, "\n"))
			}
		})
	}
}

func TestCheckAllTakesWorstVerdict(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "good", completeSpec())
	writeSpec(t, root, "bad", "# bad\n\n## Problem\n\nSomething.\n")
	out, _ := collect()
	if code := Check(root, "all", out); code != 1 {
		t.Fatalf("exit %d, want 1 — one incomplete spec must block", code)
	}
}

func TestCheckRefusesTraversalNames(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "widget", completeSpec())
	for _, name := range []string{"../widget", "..", "a/b", "../../etc/passwd"} {
		out, _ := collect()
		if code := Check(root, name, out); code != 2 {
			t.Errorf("name %q: exit %d, want 2 (refused)", name, code)
		}
	}
	out, _ := collect()
	if code := PrintTemplate("../escape", out); code != 2 {
		t.Error("template must refuse traversal names")
	}
}

func TestCheckUnknownSpec(t *testing.T) {
	out, _ := collect()
	if code := Check(t.TempDir(), "nope", out); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestCheckNoSpecs(t *testing.T) {
	out, _ := collect()
	if code := Check(t.TempDir(), "", out); code != 0 {
		t.Fatalf("exit %d, want 0 — no specs is not a failure", code)
	}
}

func TestTemplateFailsItsOwnCheck(t *testing.T) {
	// the freshly printed template must NOT pass check — the placeholder
	// and the OPEN: line are there precisely so an unfilled spec blocks
	root := t.TempDir()
	writeSpec(t, root, "fresh", strings.ReplaceAll(Template, "%s", "fresh"))
	out, lines := collect()
	if code := Check(root, "fresh", out); code != 1 {
		t.Fatalf("an unfilled template must block, exit %d:\n%s", code, strings.Join(*lines, "\n"))
	}
}

// TestCompleteSpecStillSayingDraftIsSaid pins the note that keeps the
// Status header honest: the template ships `Status: draft` and nothing
// advances it, so twelve delivered specs read as drafts. Complete plus
// draft earns a note; complete plus any other status is silent; and the
// note never changes the verdict.
func TestCompleteSpecStillSayingDraftIsSaid(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "widget", completeSpec())
	out, lines := collect()
	if code := Check(root, "widget", out); code != 0 {
		t.Fatalf("a note is not a gap: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "Status line still says draft") {
		t.Errorf("complete-but-draft must be said: %v", *lines)
	}

	writeSpec(t, root, "widget", strings.Replace(completeSpec(), "Status: draft", "Status: complete", 1))
	out2, lines2 := collect()
	if code := Check(root, "widget", out2); code != 0 {
		t.Fatalf("exit %d %v", code, *lines2)
	}
	if strings.Contains(strings.Join(*lines2, "\n"), "Status line") {
		t.Errorf("an advanced status must earn no note: %v", *lines2)
	}
}

// The mirror of the note above, and the more misleading of the two: a spec
// whose header claims complete while the checker refuses it sends a reader to
// build from an unfinished design.
func TestASpecClaimingCompleteWithGapsIsContradicted(t *testing.T) {
	root := t.TempDir()
	gapped := strings.Replace(completeSpec(), "Status: draft", "Status: complete", 1)
	gapped = strings.Replace(gapped, "A real answer for Data.", "", 1)
	writeSpec(t, root, "widget", gapped)

	out, lines := collect()
	if code := Check(root, "widget", out); code != 1 {
		t.Fatalf("a gap is still a gap: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "says complete, and the gaps above say otherwise") {
		t.Errorf("the contradiction must be named: %v", *lines)
	}
}
