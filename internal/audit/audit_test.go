package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/docs"
	"procoder/internal/gitx"
)

// write plants a fixture file, creating its directory.
func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A BLOCK finding sitting past the section cap must still be shown:
// orderForDisplay puts blocking findings first, so truncation only ever
// drops info lines.
func TestOrderForDisplayBlockingSurvivesCap(t *testing.T) {
	var findings []gitx.Finding
	for i := range maxPerSection + 5 {
		findings = append(findings, gitx.Finding{Message: fmt.Sprintf("info %d", i)})
	}
	findings = append(findings, gitx.Finding{Blocking: true, Message: "block A"})
	findings = append(findings, gitx.Finding{Blocking: true, Message: "block B"})

	ordered := orderForDisplay(findings)
	if len(ordered) != len(findings) {
		t.Fatalf("ordering changed length: got %d, want %d", len(ordered), len(findings))
	}
	if ordered[0].Message != "block A" || ordered[1].Message != "block B" {
		t.Fatalf("blocking findings not first (stable): got %q, %q", ordered[0].Message, ordered[1].Message)
	}
	for i, f := range ordered[2:] {
		if want := fmt.Sprintf("info %d", i); f.Message != want {
			t.Fatalf("info order not stable at %d: got %q, want %q", i, f.Message, want)
		}
	}
}

// A sweep passes every tracked file, so the two diff-scoped documentation
// questions — does a doc mention what you changed, did this diff move
// public surface — would answer about everything at once. On this
// repository that was 129 findings burying nine real ones.
func TestSweepDropsTheDiffScopedDocumentationQuestions(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/thing\n\ngo 1.22\n")
	write(t, root, "internal/service/handler.go", "package service\n\n// Serve serves.\nfunc Serve() {}\n")
	write(t, root, "docs/guide.md", "# Guide\n\nSee internal/service/handler.go for the entry point.\n")
	files := []string{
		filepath.Join(root, "go.mod"),
		filepath.Join(root, "internal/service/handler.go"),
		filepath.Join(root, "docs/guide.md"),
	}
	for _, f := range docs.CollectSweep(root, files) {
		if strings.Contains(f.Message, "mentions changed file") ||
			strings.Contains(f.Message, "documentation obligation") {
			t.Fatalf("a sweep must not ask a diff-scoped question: %s", f.Message)
		}
	}
	// and the diff path still asks it, so nothing was lost
	var sawDrift bool
	for _, f := range docs.CollectOfflineFor(root, files, "", false) {
		if strings.Contains(f.Message, "mentions changed file") {
			sawDrift = true
		}
	}
	if !sawDrift {
		t.Fatal("the diff path must still report drift — the check moved, it did not die")
	}
}
