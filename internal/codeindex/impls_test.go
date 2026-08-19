package codeindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRefs plants a precise tier by hand, so the pure-logic legs run
// without any SCIP indexer installed.
func writeRefs(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, refsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const implsFixture = `{"documents":[
 {"relative_path":"widget.go",
  "occurrences":[{"range":[9,5,11],"symbol":"x ` + "`demo`" + `/Widget#","symbol_roles":1}],
  "symbols":[{"symbol":"x ` + "`demo`" + `/Widget#",
   "relationships":[{"symbol":"x ` + "`demo`" + `/Spinner#","is_implementation":true}]}]},
 {"relative_path":"iface.go",
  "occurrences":[{"range":[2,5,12],"symbol":"x ` + "`demo`" + `/Spinner#","symbol_roles":1}],
  "symbols":[{"symbol":"x ` + "`demo`" + `/Spinner#"}]}
]}`

func TestImplsFindsImplementingDefinitions(t *testing.T) {
	root := t.TempDir()
	writeRefs(t, root, implsFixture)
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Impls(root, "Spinner", out); code != 0 {
		t.Fatalf("impls: exit %d, %v", code, lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "widget.go:10") || !strings.Contains(joined, "Widget#") {
		t.Fatalf("must name the implementing definition at file:line: %v", lines)
	}
}

func TestImplsWithoutPreciseTierSaysNotBuilt(t *testing.T) {
	root := t.TempDir()
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Impls(root, "Spinner", out); code != 1 {
		t.Fatalf("missing precise tier must exit 1: %d %v", code, lines)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "not built") {
		t.Fatalf("must say the tier is not built, never guess textually: %v", lines)
	}
}

func TestImplsUnknownSymbolSaysSo(t *testing.T) {
	root := t.TempDir()
	writeRefs(t, root, implsFixture)
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Impls(root, "Nothing", out); code != 1 {
		t.Fatalf("unknown symbol must exit 1: %d %v", code, lines)
	}
}

func TestDetectIndexersCoversPolyglotLayouts(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"go.mod", "Cargo.toml", "tsconfig.json", "package.json", "pyproject.toml"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := detectIndexers(root)
	want := []string{"scip-go", "rust-analyzer", "scip-typescript", "scip-python"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("polyglot layout must list every indexer once (tsconfig and package.json share one): got %v", got)
	}
	if got := detectIndexers(t.TempDir()); got != nil {
		t.Fatalf("an empty layout has no indexers: %v", got)
	}
}
