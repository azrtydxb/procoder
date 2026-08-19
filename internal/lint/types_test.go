package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTscRebasesPathsOntoTheRoot(t *testing.T) {
	root := string(os.PathSeparator) + filepath.Join("repo")
	dir := filepath.Join(root, "web")
	raw := "src/app.ts(12,5): error TS2345: Argument of type 'string' is not assignable.\n" +
		"noise line\n" +
		"src/app.ts(20,1): warning TS6133: 'x' is declared but never read.\n"
	got := parseTsc(raw, dir, root, false)
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %v", got)
	}
	if got[0].File != "web/src/app.ts" || got[0].Line != 12 {
		t.Fatalf("path must be root-relative: %+v", got[0])
	}
	if !strings.Contains(got[0].Message, "[TS2345]") || !strings.Contains(got[0].Message, "(lint --types)") {
		t.Fatalf("finding must carry the TS code and the label: %+v", got[0])
	}
}

func TestParsePyrightReadsDiagnosticsOneBased(t *testing.T) {
	raw := `{"generalDiagnostics":[
	  {"file":"/repo/app.py","severity":"error","message":"Operator \"+\" not supported\nextra context","rule":"reportOperatorIssue","range":{"start":{"line":4}}},
	  {"file":"/repo/app.py","severity":"information","message":"ignored","range":{"start":{"line":1}}}
	]}`
	got := parsePyright(raw, "/repo", true)
	if len(got) != 1 {
		t.Fatalf("information-level noise must be dropped: %v", got)
	}
	f := got[0]
	if f.File != "app.py" || f.Line != 5 || !f.Blocking {
		t.Fatalf("0-based line must become 1-based, path root-relative: %+v", f)
	}
	if strings.Contains(f.Message, "extra context") {
		t.Fatalf("only the first message line belongs in the finding: %+v", f)
	}
	if got := parsePyright("not json", "/repo", false); got != nil {
		t.Fatalf("unreadable output must parse to nothing, not panic: %v", got)
	}
}

func TestTypesWithoutTsconfigIsOutOfScopeNotSilent(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "app.ts")
	if err := os.WriteFile(f, []byte("let x: number = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Types(root, []string{f}, false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "tsconfig.json") {
		t.Fatalf("TS without a project tsconfig must be declared out of scope: %v", got)
	}
}

func TestTypesIgnoresAlreadyCompiledEcosystems(t *testing.T) {
	root := t.TempDir()
	if got := Types(root, []string{filepath.Join(root, "main.go")}, false); got != nil {
		t.Fatalf("Go is compiled by its linter — --types must add nothing: %v", got)
	}
}
