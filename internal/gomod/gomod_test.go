package gomod

import (
	"os"
	"path/filepath"
	"testing"
)

func layout(t *testing.T, files ...string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// proved by: start the ascent at filepath.Dir(p) for a directory too —
// sub (a package directory at the nested module's root) resolves to the
// root module, and the "dir input" row fails.
func TestModuleDir(t *testing.T) {
	root := layout(t, "go.mod", "a/a.go", "sub/go.mod", "sub/main.go", "sub/x/x.go",
		"sub/testdata/go.mod", "sub/testdata/t.go", "vendor/go.mod", "vendor/v/v.go")
	sub := filepath.Join(root, "sub")
	for name, c := range map[string]struct{ in, want string }{
		"file in root module":        {filepath.Join(root, "a", "a.go"), root},
		"file in module root":        {filepath.Join(sub, "main.go"), sub},
		"file below module root":     {filepath.Join(sub, "x", "x.go"), sub},
		"dir input is its own start": {sub, sub},
		"dir input below":            {filepath.Join(sub, "x"), sub},
		"outside root":               {filepath.Join(filepath.Dir(root), "elsewhere", "e.go"), root},
		"testdata go.mod ignored":    {filepath.Join(sub, "testdata", "t.go"), sub},
		"vendor go.mod ignored":      {filepath.Join(root, "vendor", "v", "v.go"), root},
		"no go.mod anywhere":         {filepath.Join(root, "nomod", "z.go"), root},
	} {
		if got := ModuleDir(root, c.in); got != c.want {
			t.Errorf("%s: ModuleDir(%s) = %s, want %s", name, c.in, got, c.want)
		}
	}
}

// proved by: drop the go.work branch of Group — the workspace splits in two.
func TestGroup(t *testing.T) {
	root := layout(t, "go.mod", "a/a.go", "sub/go.mod", "sub/x/x.go")
	a, b := filepath.Join(root, "a", "a.go"), filepath.Join(root, "sub", "x", "x.go")
	if g := Group(root, []string{a, b}); len(g) != 2 {
		t.Errorf("want a group per module, got %v", g)
	}
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if g := Group(root, []string{a, b}); len(g) != 1 || len(g[root]) != 2 {
		t.Errorf("a go.work repo is one group at the root, got %v", g)
	}
}
