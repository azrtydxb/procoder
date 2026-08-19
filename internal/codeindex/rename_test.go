package codeindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// writeTags plants a broad-tier index by hand, so the pure-logic legs run
// without ctags installed.
func writeTags(t *testing.T, root string, tags []Tag) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, tag := range tags {
		enc, _ := json.Marshal(tag)
		b.Write(enc)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, tagsFile), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenameUnknownSymbolSaysSo(t *testing.T) {
	root := t.TempDir()
	writeTags(t, root, []Tag{{Name: "Greet", Path: "demo.go", Line: 4, Kind: "func"}})
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Rename(root, "Nope", "Better", "", out); code != 1 {
		t.Fatalf("unknown symbol must exit 1: %d %v", code, lines)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "no definition") {
		t.Fatalf("must say the symbol is unknown: %v", lines)
	}
}

func TestRenameAmbiguousDefinitionDemandsAt(t *testing.T) {
	root := t.TempDir()
	writeTags(t, root, []Tag{
		{Name: "Run", Path: "a.go", Line: 3, Kind: "func"},
		{Name: "Run", Path: "b.go", Line: 9, Kind: "func"},
	})
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Rename(root, "Run", "Exec", "", out); code != 2 {
		t.Fatalf("ambiguity must exit 2: %d %v", code, lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "--at") || !strings.Contains(joined, "a.go:3") || !strings.Contains(joined, "b.go:9") {
		t.Fatalf("must list every definition and ask for --at: %v", lines)
	}
}

func TestRenameNonGoAnswersWithTheWorksheet(t *testing.T) {
	requireCtags(t)
	root := gitRepo(t)
	writeFile(t, root, "app.py", "def greet(name):\n    return name\n\n\nprint(greet(\"x\"))\n")
	if err := Build(root, func(string) {}); err != nil {
		t.Fatal(err)
	}
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	Rename(root, "greet", "welcome", "", out)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "no rename engine") {
		t.Fatalf("non-Go must say there is no engine: %v", lines)
	}
	if !strings.Contains(joined, "app.py") {
		t.Fatalf("the worksheet must list the reference sites: %v", lines)
	}
}

func TestRenameGoPrintsADiffAndWritesNothing(t *testing.T) {
	requireCtags(t)
	if tools.Resolve(Gopls, "") == "" {
		t.Skip("gopls not installed; integration leg runs where it is")
	}
	root := gitRepo(t)
	writeFile(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	writeFile(t, root, "demo.go", goSrc)
	writeFile(t, root, "use.go", "package demo\n\nvar _ = Greet\n")
	if err := Build(root, func(string) {}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "demo.go"))

	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Rename(root, "Greet", "Welcome", "", out); code != 0 {
		t.Fatalf("rename: exit %d, %v", code, lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Welcome") || !strings.Contains(joined, "use.go") {
		t.Fatalf("the diff must cover every referencing file: %v", lines)
	}
	if !strings.Contains(joined, "procoder never writes code") {
		t.Fatalf("the P-CONTROL footer must be present: %v", lines)
	}
	after, _ := os.ReadFile(filepath.Join(root, "demo.go"))
	if string(before) != string(after) {
		t.Fatal("rename must not modify the file — the diff is the whole answer")
	}
}

func TestSymbolColumnFindsWordBoundaries(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "x.go")
	os.WriteFile(p, []byte("package x\nfunc Greeting() { Greet() }\n"), 0o644)
	col, err := symbolColumn(p, 2, "Greet")
	if err != nil {
		t.Fatal(err)
	}
	// "Greeting" must not match; the call at column 19 must
	if col != 19 {
		t.Fatalf("want column 19, got %d", col)
	}
	if _, err := symbolColumn(p, 2, "Absent"); err == nil {
		t.Fatal("a symbol not on the line must error, not guess")
	}
}
