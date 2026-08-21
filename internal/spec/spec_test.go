package spec

import (
	"procoder/internal/answers"

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

// A question is unresolved because it is still in the section, whatever it
// is called. Two specs in this repository carried seven undecided questions
// between them, written as `- [O-1] …` rather than `OPEN:`, and were
// reported COMPLETE — and `backlog seed` refuses anything that is not
// COMPLETE, so it would have seeded stories from a design nobody had
// finished.
// proved by: matched only the OPEN: prefix again — the [O-1] spec then
// passes and the controller blesses an unfinished design.
func TestOpenQuestionsAreUnresolvedWhateverTheyAreCalled(t *testing.T) {
	root := t.TempDir()
	for _, shape := range []string{
		"- [O-1] Should answers persist across runs?",
		"OPEN: should answers persist across runs?",
		"Should answers persist? Nobody has decided.",
		"1. persist or re-ask",
	} {
		spec := strings.Replace(completeSpec(), "## Open questions\n\n", "## Open questions\n\n"+shape+"\n\n", 1)
		writeSpec(t, root, "widget", spec)
		out, lines := collect()
		if code := Check(root, "widget", out); code != 1 {
			t.Errorf("an undecided question must block, whatever its shape (%q): exit %d", shape, code)
		}
		if !strings.Contains(strings.Join(*lines, "\n"), "Open questions") {
			t.Errorf("the refusal must name the section (%q): %v", shape, *lines)
		}
	}

	// A section holding only the template's comment is resolved: that is how
	// a finished spec records "none — decisions above".
	writeSpec(t, root, "widget", completeSpec())
	out, lines := collect()
	if code := Check(root, "widget", out); code != 0 {
		t.Errorf("an empty section is a resolved one: exit %d %v", code, *lines)
	}
}

// D-5 and C-09: a question a human has decided no longer blocks, while an
// unanswered one still does and a stale answer counts for nothing. The
// verdict moves; the silence does not — a reader is told where the decisions
// live rather than shown a section of questions and called finished.
// proved by: ignored the answers store again — the answered spec then still
// reports NOT ready and the human's decision buys nothing.
func TestAnAnsweredQuestionNoLongerBlocks(t *testing.T) {
	root := t.TempDir()
	question := "which database?"
	spec := strings.Replace(completeSpec(), "## Open questions\n\n", "## Open questions\n\n- "+question+"\n\n", 1)
	writeSpec(t, root, "widget", spec)

	// Unanswered: blocked, exactly as before.
	out, lines := collect()
	if code := Check(root, "widget", out); code != 1 {
		t.Fatalf("an undecided question still blocks: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "unanswered") {
		t.Errorf("the refusal must say they are unanswered: %v", *lines)
	}

	// Answered: COMPLETE, and it says where the decision lives.
	writeAnswer(t, root, "spec", "widget", question, "postgres, for the constraints")
	out2, lines2 := collect()
	if code := Check(root, "widget", out2); code != 0 {
		t.Fatalf("an answered question no longer blocks: exit %d %v", code, *lines2)
	}
	joined := strings.Join(*lines2, "\n")
	if !strings.Contains(joined, "COMPLETE") {
		t.Errorf("the verdict moves: %v", *lines2)
	}
	if !strings.Contains(joined, "answered in .procoder/ask/answers.md") {
		t.Errorf("and says where the decisions are, so nobody reads the section as finished: %v", *lines2)
	}

	// Reworded: the answer was to a different question, so it blocks again.
	writeSpec(t, root, "widget", strings.Replace(spec, question, "which database, and why?", 1))
	out3, lines3 := collect()
	if code := Check(root, "widget", out3); code != 1 {
		t.Fatalf("a stale answer counts for nothing: exit %d %v", code, *lines3)
	}
}

// writeAnswer records one decision the way `procoder ask` would.
func writeAnswer(t *testing.T, root, source, origin, question, answer string) {
	t.Helper()
	dir := filepath.Join(root, ".procoder", "ask")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# answers\n\n## " + origin + "\n\n" +
		"Key: " + answers.Key(source, origin, question) + "\n" +
		"Answer: " + answer + "\n"
	if err := os.WriteFile(filepath.Join(dir, "answers.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
