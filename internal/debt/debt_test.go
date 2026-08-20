package debt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mk is the default marker assembled at runtime, so our own repo's debt
// scan never harvests these fixtures (the same lesson as secret fixtures:
// a literal marker in test source is a self-scan false positive).
const mk = "de" + "bt:"

// gitRepo makes a temp repo with the given files committed.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestScanFindsMarkersAndFlagsMissingTriggers(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"a.go": "package a\n\n// " + mk + " global lock, per-account locks when throughput matters\nvar x int\n",
		"b.py": "# " + mk + " naive O(n^2) scan\nx = 1\n",
		"c.md": "prose that mentions " + mk + " conventions without a comment leader\n",
	})
	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries (prose without comment leader excluded), got %+v", entries)
	}
	byFile := map[string]Entry{}
	for _, e := range entries {
		byFile[e.File] = e
	}
	if e := byFile["a.go"]; e.NoTrigger || e.Line != 3 {
		t.Errorf("a.go: marker with 'when' clause must have a trigger: %+v", e)
	}
	if e := byFile["b.py"]; !e.NoTrigger {
		t.Errorf("b.py: marker naming no revisit condition must be flagged: %+v", e)
	}
}

func TestScanTrimsBlockCommentTerminators(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"a.c":    "int x; /* " + mk + " fixed buffer, grow when inputs exceed 4k */\n",
		"b.html": "<!-- " + mk + " inline styles, extract when a second page exists -->\n",
	})
	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %+v", entries)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Text, "*/") || strings.HasSuffix(e.Text, "-->") {
			t.Errorf("comment terminator not trimmed: %q", e.Text)
		}
	}
}

func TestScanSkipsBinaries(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"bin.dat": "PK\x00\x03// " + mk + " not really\n",
	})
	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("binary content must be skipped, got %+v", entries)
	}
}

func TestCustomMarkerViaConfig(t *testing.T) {
	root := gitRepo(t, map[string]string{
		".procoder/config.toml": "[debt]\nmarker = \"shortcut:\"\n",
		"a.go":                  "package a\n// shortcut: cached forever, revisit when memory matters\n// " + mk + " this one must NOT match under the custom marker\n",
	})
	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Text, "cached forever") {
		t.Fatalf("custom marker must win: %+v", entries)
	}
}

func TestRunCleanLedger(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.go": "package a\n"})
	var lines []string
	if code := Run(root, func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "clean ledger") {
		t.Errorf("no explicit null result: %v", lines)
	}
}

// A marker's revisit condition routinely lands on a continuation line —
// the marker line is already full of what the ceiling is. Reading only the
// first line calls those "no trigger", which is the ledger crying rot over
// debt that was recorded exactly as the principles ask.
func TestTriggerOnAContinuationLineCounts(t *testing.T) {
	// the marker is assembled rather than written literally: a fixture that
	// carries one would be harvested from this repository's own ledger
	m := "de" + "bt:"
	root := gitRepo(t, map[string]string{"a.go": "package a\n\nfunc f() {\n" +
		"\t// " + m + " tsc checks the whole project but only findings in the asked\n" +
		"\t// files are kept (the clippy precedent) — cross-file fallout hides;\n" +
		"\t// revisit if a rename ships broken siblings past this.\n\t_ = 1\n}\n"})
	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want one marker, got %d: %+v", len(entries), entries)
	}
	if entries[0].NoTrigger {
		t.Fatalf("the revisit condition is on the third line and must count: %q", entries[0].Text)
	}
}

// A marker with no revisit condition anywhere in its block is still rot,
// and must stay flagged — the fix above must not simply stop flagging.
func TestAMarkerWithNoConditionAnywhereIsStillFlagged(t *testing.T) {
	m := "de" + "bt:"
	root := gitRepo(t, map[string]string{"b.go": "package b\n\nfunc g() {\n" +
		"\t// " + m + " a global lock here, it is coarse and slow\n" +
		"\t// and nothing about this comment says what would change our mind\n\t_ = 2\n}\n"})
	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].NoTrigger {
		t.Fatalf("a marker with no condition must stay flagged: %+v", entries)
	}
}
