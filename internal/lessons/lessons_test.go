package lessons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ledger = `# Lessons

Intro prose that is not an entry.

## 2026-08-19 PR#18 — block-comment terminator leaked into ledger text

- Class: mechanical
- Missed by: test
- Adaptation: TestScanTrimsBlockCommentTerminators pins both terminators

## 2026-08-19 PR#18 — regex compiled once per task in plan check

- Class: mechanical
- Missed by: linter
- Adaptation:

## 2026-08-19 PR#17 — path traversal via task id

- Class: judgment
- Missed by: rubric
`

func TestParseFindsEntriesAndAdaptations(t *testing.T) {
	entries := Parse(ledger)
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %+v", entries)
	}
	if entries[0].Adaptation == "" {
		t.Error("first entry has an adaptation")
	}
	if entries[1].Adaptation != "" {
		t.Errorf("empty Adaptation: line must read as unlearned, got %q", entries[1].Adaptation)
	}
	if entries[2].Adaptation != "" {
		t.Error("missing Adaptation line must read as unlearned")
	}
}

func TestRunFlagsUnlearned(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder/github"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Path), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	var lines []string
	code := Run(root, func(s string) { lines = append(lines, s) })
	if code != 1 {
		t.Fatalf("unlearned lessons must exit 1, got %d:\n%s", code, strings.Join(lines, "\n"))
	}
	joined := strings.Join(lines, "\n")
	if strings.Count(joined, "UNLEARNED") != 2 {
		t.Errorf("want 2 unlearned flags:\n%s", joined)
	}
	if !strings.Contains(joined, "3 lesson(s), 2 unlearned") {
		t.Errorf("summary wrong:\n%s", joined)
	}
}

// The failure this pins: the template's entry-shape example once started a
// line with "## ", so a fresh ledger written verbatim parsed as one
// UNLEARNED entry and exited 1 — the shipped shape failed its own check.
func TestDefaultLedgerParsesAsEmpty(t *testing.T) {
	if entries := Parse(DefaultLedger); len(entries) != 0 {
		t.Fatalf("the shipped template must yield zero entries, got %+v", entries)
	}
}

func TestRunNoLedgerIsCalm(t *testing.T) {
	var lines []string
	if code := Run(t.TempDir(), func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("missing ledger is not a failure, got %d", code)
	}
}

func TestPlaceholderAdaptationIsUnlearned(t *testing.T) {
	entries := Parse("## x\n\n- Adaptation: <the concrete change>\n")
	if len(entries) != 1 {
		t.Fatal("one entry expected")
	}
	// Run treats template placeholders as unlearned
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder/github"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Path), []byte("## x\n\n- Adaptation: <fill me>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var lines []string
	if code := Run(root, func(s string) { lines = append(lines, s) }); code != 1 {
		t.Fatalf("placeholder adaptation must exit 1, got %d:\n%s", code, strings.Join(lines, "\n"))
	}
}
