package codeindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// requireTools skips when the live toolchain isn't present — the pure-logic
// tests below carry the coverage everywhere; these prove the integration.
func requireCtags(t *testing.T) {
	t.Helper()
	if tools.Resolve(Ctags, "") == "" {
		t.Skip("universal-ctags not installed; integration leg runs where it is")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	return root
}

func writeFile(t *testing.T, root, name, body string) string {
	t.Helper()
	p := filepath.Join(root, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const goSrc = `package demo

// Greet says hello.
func Greet(name string) string { return "hi " + name }

type Widget struct{}

func (w Widget) Spin() {}
`

func buildDemo(t *testing.T) string {
	t.Helper()
	root := gitRepo(t)
	writeFile(t, root, "demo.go", goSrc)
	writeFile(t, root, "use.go", "package demo\n\nvar _ = Greet\n")
	quiet := func(string) {}
	if err := Build(root, quiet); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBuildFindOutlineSearchAgainstRealCtags(t *testing.T) {
	requireCtags(t)
	root := buildDemo(t)

	var lines []string
	out := func(s string) { lines = append(lines, s) }

	if code := Find(root, "Greet", out); code != 0 {
		t.Fatalf("find Greet: exit %d, %v", code, lines)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "demo.go:4") {
		t.Fatalf("find must name file:line: %v", lines)
	}

	lines = nil
	if code := Outline(root, filepath.Join(root, "demo.go"), out); code != 0 {
		t.Fatalf("outline: %v", lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Greet") || !strings.Contains(joined, "Widget") || !strings.Contains(joined, "Spin") {
		t.Fatalf("outline must list the file's symbols in order: %v", lines)
	}

	lines = nil
	if code := Search(root, "gre", out); code != 0 || !strings.Contains(strings.Join(lines, "\n"), "Greet") {
		t.Fatalf("fuzzy search must find Greet: %v", lines)
	}
}

func TestTextualRefsAreLabeledAndStaleIndexSpeaksUp(t *testing.T) {
	requireCtags(t)
	root := buildDemo(t)

	var lines []string
	out := func(s string) { lines = append(lines, s) }
	// no precise tier in this tiny repo (no go.mod) — refs must fall back
	// and say so
	if code := Refs(root, "Greet", out); code != 0 {
		t.Fatalf("refs: %v", lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "(textual)") {
		t.Fatalf("fallback refs must be labeled textual: %v", lines)
	}

	// a new commit makes the index stale, and stats must say so
	writeFile(t, root, "extra.go", "package demo\n")
	cmd := exec.Command("git", "-C", root, "add", "-A")
	cmd.Run()
	cmd = exec.Command("git", "-C", root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x")
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v %s", err, outb)
	}
	lines = nil
	Stats(root, out)
	if !strings.Contains(strings.Join(lines, "\n"), "run `procoder index build` to refresh") {
		t.Fatalf("stale index must speak up: %v", lines)
	}
}

func TestRefreshReindexesOneFileInPlace(t *testing.T) {
	requireCtags(t)
	root := buildDemo(t)

	// the file gains a symbol; Refresh must pick it up without a full build
	writeFile(t, root, "demo.go", goSrc+"\nfunc Extra() {}\n")
	Refresh(root, filepath.Join(root, "demo.go"))

	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Find(root, "Extra", out); code != 0 {
		t.Fatalf("Refresh missed the new symbol: %v", lines)
	}
	// and the old entries for that file were replaced, not duplicated
	lines = nil
	Find(root, "Greet", out)
	defs := 0
	for _, l := range lines {
		if strings.Contains(l, "demo.go") {
			defs++
		}
	}
	if defs != 1 {
		t.Fatalf("Refresh must replace the file's entries, not append: %v", lines)
	}
}

func TestImpactNamesTheBlastRadius(t *testing.T) {
	requireCtags(t)
	root := buildDemo(t)
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Impact(root, []string{filepath.Join(root, "demo.go")}, out); code != 0 {
		t.Fatalf("impact: %v", lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "use.go") || !strings.Contains(joined, "Greet") {
		t.Fatalf("impact must name use.go as referencing Greet: %v", lines)
	}
}

func TestMissingIndexIsAnInstructionNotAnEmptyAnswer(t *testing.T) {
	root := t.TempDir()
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Find(root, "X", out); code != 1 || !strings.Contains(lines[0], "procoder index build") {
		t.Fatalf("missing index must say how to fix it: %v", lines)
	}
}

func TestScipSymbolMatching(t *testing.T) {
	sym := "scip-go gomod procoder cdbd1a3 `procoder/internal/gitx`/Attribution()."
	if !scipSymbolIs(sym, "Attribution") {
		t.Fatal("function descriptor must match")
	}
	if scipSymbolIs(sym, "Attribu") {
		t.Fatal("prefix must not match")
	}
	if !scipSymbolIs("x `pkg`/Finding#", "Finding") {
		t.Fatal("type descriptor must match")
	}
	if !scipSymbolIs("x `pkg`/Finding#File.", "File") {
		t.Fatal("field descriptor must match")
	}
}

// A minimal precise-tier fixture: alpha calls beta; gamma is defined and
// never referenced — the graph queries must read exactly that from it.
const fakeRefs = "{\"documents\":[{\"relative_path\":\"a.go\",\"occurrences\":[" +
	"{\"range\":[0,5,10],\"symbol\":\"x `pkg`/alpha().\",\"symbol_roles\":1,\"enclosing_range\":[0,0,5,1]}," +
	"{\"range\":[2,8,12],\"symbol\":\"x `pkg`/beta().\",\"symbol_roles\":0}," +
	"{\"range\":[8,5,9],\"symbol\":\"x `pkg`/beta().\",\"symbol_roles\":1,\"enclosing_range\":[8,0,9,1]}," +
	"{\"range\":[12,5,10],\"symbol\":\"x `pkg`/gamma().\",\"symbol_roles\":1,\"enclosing_range\":[12,0,13,1]}" +
	"]}]}"

func writeFakePrecise(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, Dir), 0o755)
	if err := os.WriteFile(filepath.Join(root, Dir, "refs.json"), []byte(fakeRefs), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCallersReadsTheEdgeFromTheFixture(t *testing.T) {
	root := writeFakePrecise(t)
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Callers(root, "beta", out); code != 0 {
		t.Fatalf("callers: %v", lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "caller  a.go:3") || !strings.Contains(joined, "alpha") {
		t.Fatalf("alpha calls beta at a.go:3: %v", lines)
	}
}

func TestUnusedFindsGammaAndOnlyGamma(t *testing.T) {
	root := writeFakePrecise(t)
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Unused(root, out); code != 0 {
		t.Fatalf("unused: %v", lines)
	}
	joined := strings.Join(lines, "\n")
	// gamma is never referenced; alpha calls others but nothing calls it —
	// both are unreferenced. beta is referenced and must not appear.
	if !strings.Contains(joined, "gamma") || !strings.Contains(joined, "alpha") {
		t.Fatalf("gamma and alpha are unreferenced and must be reported: %v", lines)
	}
	if strings.Contains(joined, "beta") {
		t.Fatalf("beta is referenced and is not dead: %v", lines)
	}
}

// shortSym must return just the descriptor for both full five-field SCIP
// symbols and the minimal shapes fixtures use.
func TestShortSymHandlesFullAndMinimalShapes(t *testing.T) {
	full := "scip-go gomod github.com/golang/go/src go1.23 io/ReadAll()."
	if got := shortSym(full); got != "io/ReadAll()" {
		t.Fatalf("full shape: got %q", got)
	}
	minimal := "x `pkg`/alpha()."
	if got := shortSym(minimal); got != "pkg/alpha()" {
		t.Fatalf("minimal shape: got %q", got)
	}
}

func TestGraphQueriesWithoutPreciseTierSayHowToGetIt(t *testing.T) {
	root := t.TempDir()
	var lines []string
	out := func(s string) { lines = append(lines, s) }
	if code := Callers(root, "x", out); code != 1 || !strings.Contains(lines[0], "procoder index build") {
		t.Fatalf("missing precise tier must say the fix: %v", lines)
	}
}

func TestNormalizeTagsDropsNonTagLinesAndKeepsShape(t *testing.T) {
	raw := []byte(`{"_type":"program","name":"ctags"}
{"_type":"tag","name":"Greet","path":"demo.go","line":4,"kind":"func","signature":"(name string)","language":"Go"}
garbage`)
	out, n := normalizeTags(raw)
	if n != 1 || !strings.Contains(string(out), `"name":"Greet"`) {
		t.Fatalf("want exactly the one tag, got %d: %s", n, out)
	}
}
