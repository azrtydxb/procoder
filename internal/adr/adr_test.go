package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// record builds a well-formed record body for fixtures.
func record(num, title, status string) string {
	return "# " + num + " — " + title + "\n\n" +
		"Status: " + status + "\n" +
		"Date: 2026-08-19\n\n" +
		"## Context\n\nThe constraint that forced this.\n\n" +
		"## Decision\n\nWhat we chose over the alternatives.\n\n" +
		"## Consequences\n\nWhat got easier and what got harder.\n"
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

func TestNewNumbersFromOneAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	out, lines := collect()
	if code := New(root, "Use SCIP over LSP", out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	text := strings.Join(*lines, "\n")
	if !strings.Contains(text, Dir+"/0001-use-scip-over-lsp.md") {
		t.Errorf("empty repo should print 0001, got:\n%s", text)
	}
	if !strings.Contains(text, "# 0001 — Use SCIP over LSP") ||
		!strings.Contains(text, "Status: proposed") ||
		!strings.Contains(text, "Date: "+time.Now().UTC().Format("2006-01-02")) {
		t.Errorf("printed record misses header, status, or today's date:\n%s", text)
	}
	for _, name := range []string{"## Context", "## Decision", "## Consequences"} {
		if !strings.Contains(text, name) {
			t.Errorf("printed record misses section %q", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, Dir)); !os.IsNotExist(err) {
		t.Error("New must print, never write — the adr directory was created")
	}
}

func TestNewNumbersAfterExisting(t *testing.T) {
	root := t.TempDir()
	write(t, root, "0002-first.md", record("0002", "First", "accepted"))
	out, lines := collect()
	if code := New(root, "Second", out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	text := strings.Join(*lines, "\n")
	if !strings.Contains(text, "/0003-second.md") {
		t.Errorf("next after 0002 is 0003, got:\n%s", text)
	}
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil || len(entries) != 1 {
		t.Errorf("New must write nothing: %d entries, err %v", len(entries), err)
	}
}

func TestNewRefusesEmptySlug(t *testing.T) {
	out, _ := collect()
	if code := New(t.TempDir(), "!!!", out); code != 2 {
		t.Errorf("a title slugifying to empty exits 2, got %d", code)
	}
}

func TestCheckCatchesEveryRefusalClass(t *testing.T) {
	root := t.TempDir()
	write(t, root, "0001-empty-decision.md", strings.Replace(
		record("0001", "Empty decision", "accepted"),
		"What we chose over the alternatives.\n", "<!-- still thinking -->\n", 1))
	write(t, root, "0002-bad-status.md", record("0002", "Bad status", "wip"))
	write(t, root, "0003-dangling.md", record("0003", "Dangling", "superseded-by-0099"))
	write(t, root, "0004-twin-a.md", record("0004", "Twin A", "accepted"))
	write(t, root, "0004-twin-b.md", record("0004", "Twin B", "accepted"))
	write(t, root, "0005-unreadable.md", record("0005", "Unreadable", "accepted"))
	locked := filepath.Join(root, Dir, "0005-unreadable.md")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o644); err != nil {
			t.Error(err)
		}
	})

	out, lines := collect()
	findings := Check(root, out)
	text := strings.Join(*lines, "\n")

	wants := map[string]string{
		"empty Decision section": "Decision is empty",
		"unknown status":         `status "wip"`,
		"dangling supersede":     "no record carries that number",
		"duplicated number":      "share number 0004",
		"unreadable file":        "unreadable:",
	}
	for class, needle := range wants {
		found := false
		for _, f := range findings {
			if strings.Contains(f.Message, needle) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s not caught — no finding contains %q\n%s", class, needle, text)
		}
	}
	if code := Run(root, func(string) {}); code != 1 {
		t.Errorf("Run over a broken set exits 1, got %d", code)
	}
}

func TestCheckPassesValidSetAndCountsProposed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "0001-old.md", record("0001", "Old way", "superseded-by-0002"))
	write(t, root, "0002-new.md", record("0002", "New way", "accepted"))
	write(t, root, "0003-open.md", record("0003", "Still open", "proposed"))
	out, lines := collect()
	if findings := Check(root, out); len(findings) != 0 {
		t.Fatalf("valid set must pass clean, got %v", findings)
	}
	if text := strings.Join(*lines, "\n"); !strings.Contains(text, "1 record(s) still proposed") {
		t.Errorf("proposed count is informational and printed, got:\n%s", text)
	}
	if code := Run(root, func(string) {}); code != 0 {
		t.Errorf("Run over a valid set exits 0, got %d", code)
	}
}

func TestCheckNoRecordsPassesClean(t *testing.T) {
	root := t.TempDir()
	out, lines := collect()
	if findings := Check(root, out); findings != nil {
		t.Fatalf("no records passes clean, got %v", findings)
	}
	if text := strings.Join(*lines, "\n"); !strings.Contains(text, "no decision records") {
		t.Errorf("check says no records, got:\n%s", text)
	}
}

func TestListOrdersProposedFirst(t *testing.T) {
	root := t.TempDir()
	write(t, root, "0001-accepted.md", record("0001", "Accepted one", "accepted"))
	write(t, root, "0002-superseded.md", record("0002", "Superseded one", "superseded-by-0001"))
	write(t, root, "0003-proposed.md", record("0003", "Proposed one", "proposed"))
	out, lines := collect()
	if code := List(root, out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	got := *lines
	if len(got) != 3 || !strings.HasPrefix(got[0], "0003") ||
		!strings.HasPrefix(got[1], "0001") || !strings.HasPrefix(got[2], "0002") {
		t.Errorf("want proposed first then by number (0003, 0001, 0002), got:\n%s",
			strings.Join(got, "\n"))
	}
	if !strings.Contains(got[2], "superseded-by-0001") {
		t.Errorf("list shows superseded-by targets, got %q", got[2])
	}
}

func TestListEmptySaysHowToStart(t *testing.T) {
	out, lines := collect()
	if code := List(t.TempDir(), out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if text := strings.Join(*lines, "\n"); !strings.Contains(text, "procoder adr new") {
		t.Errorf("an empty list says how to start, got:\n%s", text)
	}
}
